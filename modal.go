// Package tuikit holds the input components shared by the TUIs: a centred
// modal window that takes every key, a multiline editor, forms with typed
// fields (text, date, clock, duration, choice, reference, check box) and
// confirmations.
//
// A TUI opens a Modal, gives it every message while it is open, draws it with
// Overlay, and reads the answer as a DoneMsg or a CancelMsg.
package tuikit

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Action is what a Content asks of its Modal after a message.
type Action int

const (
	Continue Action = iota
	Submit          // validate, as ctrl+s
	Cancel          // close without an answer
)

// Content is what a Modal shows: a Form, an Editor, a Confirm or a
// ConfirmTyped.
type Content interface {
	// ID names the answer in DoneMsg and CancelMsg.
	ID() string
	// Resize gives the room inside the modal: width columns, height lines at most.
	Resize(width, height int)
	// Update takes every message but the modal's own keys (ctrl+s, esc).
	Update(msg tea.Msg) (Action, tea.Cmd)
	// View draws the content within the room given by Resize.
	View() string
	// Submit validates; false keeps the modal open, errors shown.
	Submit() (Values, bool)
	// Changed tells whether esc must ask before dropping the input.
	Changed() bool
	// Keys is the help line under the content.
	Keys() string
}

// DoneMsg carries the answer of a validated modal.
type DoneMsg struct {
	ID     string
	Values Values
}

// CancelMsg tells that a modal was closed without an answer.
type CancelMsg struct{ ID string }

// Modal is a centred window over the TUI that takes every key while open:
// ctrl+s (or ctrl+enter) validates, esc and ctrl+c cancel, after a Oui/Non
// question when the input has changed.
type Modal struct {
	title   string
	content Content
	width   int // terminal
	height  int
	open    bool
	quit    *Confirm
}

// NewModal opens a modal on content. Give it the terminal size with SetSize.
func NewModal(title string, content Content) *Modal {
	m := &Modal{title: title, content: content, open: true, width: 80, height: 24}
	m.resize()
	return m
}

// SetSize gives the terminal size; the modal also follows tea.WindowSizeMsg.
func (m *Modal) SetSize(width, height int) *Modal {
	m.width, m.height = width, height
	m.resize()
	return m
}

// Open tells whether the modal is still open; a nil modal is closed.
func (m *Modal) Open() bool { return m != nil && m.open }

// Content is the content of the modal.
func (m *Modal) Content() Content { return m.content }

// boxWidth is the outer width of the window.
func (m *Modal) boxWidth() int {
	return max(min(m.width-4, 90), min(m.width, 24))
}

// bodyHeight is the room left to the content: the terminal less a margin,
// the border, the title and the help line with their blank lines.
func (m *Modal) bodyHeight() int {
	return max(m.height-2-2-4, 3)
}

func (m *Modal) resize() {
	m.content.Resize(m.boxWidth()-4, m.bodyHeight())
	if m.quit != nil {
		m.quit.Resize(m.boxWidth()-4, 3)
	}
}

// Update feeds a message to the modal. Its answer comes back as a DoneMsg or
// a CancelMsg through the returned command.
func (m *Modal) Update(msg tea.Msg) tea.Cmd {
	if !m.Open() {
		return nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return nil
	case tea.BackgroundColorMsg:
		SetDarkBackground(msg.IsDark())
		return nil
	case tea.PasteMsg:
		msg.Content = normalizeNewlines(msg.Content)
		if m.quit != nil {
			return nil
		}
		return m.feed(msg)
	}
	if m.quit != nil {
		act, cmd := m.quit.Update(msg)
		if k, ok := msg.(tea.KeyPressMsg); ok && k.Keystroke() == "ctrl+c" {
			act = Cancel
		}
		switch act {
		case Submit:
			return m.close(CancelMsg{ID: m.content.ID()})
		case Cancel:
			m.quit = nil
		}
		return cmd
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.Keystroke() {
		case "ctrl+s", "ctrl+enter":
			return m.submit()
		case "esc", "ctrl+c":
			if m.content.Changed() {
				m.quit = NewConfirm("", tr("Abandonner la saisie ?", "Discard your input?"))
				m.quit.Resize(m.boxWidth()-4, 3)
				return nil
			}
			return m.close(CancelMsg{ID: m.content.ID()})
		}
	}
	return m.feed(msg)
}

func (m *Modal) feed(msg tea.Msg) tea.Cmd {
	act, cmd := m.content.Update(msg)
	switch act {
	case Submit:
		return tea.Batch(cmd, m.submit())
	case Cancel:
		return tea.Batch(cmd, m.close(CancelMsg{ID: m.content.ID()}))
	}
	return cmd
}

func (m *Modal) submit() tea.Cmd {
	v, ok := m.content.Submit()
	if !ok {
		return nil
	}
	return m.close(DoneMsg{ID: m.content.ID(), Values: v})
}

func (m *Modal) close(msg tea.Msg) tea.Cmd {
	m.open = false
	return func() tea.Msg { return msg }
}

// View draws the window alone; Overlay places it over the TUI.
func (m *Modal) View() string {
	if !m.Open() {
		return ""
	}
	inner := m.boxWidth() - 4
	parts := []string{sTitle.Render(truncate(m.title, inner)), "", m.content.View(), ""}
	if m.quit != nil {
		parts = append(parts, m.quit.View(), sDim.Render(truncate(m.quit.Keys(), inner)))
	} else {
		parts = append(parts, sDim.Render(truncate(m.content.Keys(), inner)))
	}
	return sBox.Width(m.boxWidth()).Render(strings.Join(parts, "\n"))
}

// Overlay draws the modal centred over the view of the TUI; a closed modal
// leaves the view as it is.
func Overlay(view string, m *Modal, width, height int) string {
	if !m.Open() {
		return view
	}
	if m.width != width || m.height != height {
		m.SetSize(width, height)
	}
	box := m.View()
	x := max((width-lipgloss.Width(box))/2, 0)
	y := max((height-lipgloss.Height(box))/2, 0)
	base := lipgloss.NewLayer(lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Render(view))
	return lipgloss.NewCompositor(base, lipgloss.NewLayer(box).X(x).Y(y).Z(1)).Render()
}

func normalizeNewlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

func truncate(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > width {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
