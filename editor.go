package tuikit

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// area is a multiline text zone with soft wrap and a scroll bar. It never
// limits the text: no character, line or height cap.
type area struct {
	ta         textarea.Model
	width      int // with the scroll bar
	minH, maxH int
}

func newArea(text string) *area {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.MaxWidth = 0
	ta.MaxContentHeight = 0
	ta.KeyMap.InsertNewline.SetKeys("enter", "ctrl+m", "shift+enter", "alt+enter")
	st := textarea.DefaultStyles(darkBackground)
	st.Focused.CursorLine = lipgloss.NewStyle()
	st.Focused.Base = lipgloss.NewStyle()
	st.Blurred.Base = lipgloss.NewStyle()
	st.Blurred.Text = sDim
	st.Cursor.Blink = false
	st.Cursor.Color = cAccent
	ta.SetStyles(st)
	ta.SetValue(text)
	a := &area{ta: ta, minH: 3, maxH: 8, width: 40}
	a.fit()
	return a
}

func (a *area) resize(width, minH, maxH int) {
	a.width, a.minH, a.maxH = max(width, 4), max(minH, 1), max(maxH, minH, 1)
	a.ta.SetWidth(a.width - 1)
	a.fit()
}

// visualLines counts the rows of the text once wrapped to the area.
func (a *area) visualLines() int {
	w := max(a.ta.Width(), 1)
	n := 0
	for line := range strings.SplitSeq(a.ta.Value(), "\n") {
		n += max(strings.Count(ansi.Wrap(line, w, ""), "\n")+1, 1)
	}
	return n
}

// fit grows the area with its text, between minH and maxH rows.
func (a *area) fit() {
	h := min(max(a.visualLines(), a.minH), a.maxH)
	if h != a.ta.Height() {
		a.ta.SetHeight(h)
	}
}

func (a *area) focus() tea.Cmd { return a.ta.Focus() }
func (a *area) blur()          { a.ta.Blur() }
func (a *area) value() string  { return a.ta.Value() }

func (a *area) update(msg tea.Msg) tea.Cmd {
	if p, ok := msg.(tea.PasteMsg); ok {
		p.Content = normalizeNewlines(p.Content)
		msg = p
	}
	var cmd tea.Cmd
	a.ta, cmd = a.ta.Update(msg)
	a.fit()
	return cmd
}

func (a *area) view() string {
	text := a.ta.View()
	h := a.ta.Height()
	total := a.visualLines()
	bar := make([]string, h)
	if total <= h {
		for i := range bar {
			bar[i] = sFaint.Render("│")
		}
	} else {
		thumb := max(h*h/total, 1)
		top := a.ta.ScrollYOffset() * (h - thumb) / max(total-h, 1)
		top = min(max(top, 0), h-thumb)
		for i := range bar {
			if i >= top && i < top+thumb {
				bar[i] = sFocus.Render("┃")
			} else {
				bar[i] = sFaint.Render("│")
			}
		}
	}
	lines := strings.Split(text, "\n")
	for len(lines) < h {
		lines = append(lines, "")
	}
	w := a.width - 1
	for i := range lines[:h] {
		lines[i] = lipgloss.NewStyle().Width(w).MaxWidth(w).Render(lines[i]) + bar[i]
	}
	return strings.Join(lines[:h], "\n")
}

// Editor is a free text zone filling the modal: enter inserts a line, ctrl+s
// validates; the text is under its id in Values.
type Editor struct {
	id, label, initial string
	area               *area
	width, height      int
}

// NewEditor opens a multiline zone holding text, under an optional label.
func NewEditor(id, label, text string) *Editor {
	e := &Editor{id: id, label: label, initial: text, area: newArea(text)}
	e.area.ta.CursorEnd()
	e.area.focus()
	return e
}

func (e *Editor) ID() string { return e.id }

func (e *Editor) Resize(width, height int) {
	e.width, e.height = width, height
	h := height
	if e.label != "" {
		h--
	}
	e.area.resize(width, max(h, 1), max(h, 1))
}

func (e *Editor) Update(msg tea.Msg) (Action, tea.Cmd) {
	return Continue, e.area.update(msg)
}

func (e *Editor) View() string {
	if e.label == "" {
		return e.area.view()
	}
	return sLabel.Render(e.label) + "\n" + e.area.view()
}

func (e *Editor) Submit() (Values, bool) {
	var v Values
	v.set(e.id, value{raw: e.area.value()})
	return v, true
}

func (e *Editor) Changed() bool { return e.area.value() != e.initial }

func (e *Editor) Keys() string {
	return "entrée nouvelle ligne · ctrl+s valider · esc annuler"
}

// Text is the current text.
func (e *Editor) Text() string { return e.area.value() }
