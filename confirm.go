package tuikit

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Confirm is a Oui/Non question: arrows, tab or o/n choose, enter validates.
// Oui closes the modal with a DoneMsg, Non with a CancelMsg. Non is chosen
// first unless Default(true).
type Confirm struct {
	id, question string
	yes          bool
	width        int
}

// NewConfirm asks question.
func NewConfirm(id, question string) *Confirm {
	return &Confirm{id: id, question: question, width: 60}
}

// Default sets the answer chosen first.
func (c *Confirm) Default(yes bool) *Confirm {
	c.yes = yes
	return c
}

// Yes tells whether Oui is chosen.
func (c *Confirm) Yes() bool { return c.yes }

func (c *Confirm) ID() string               { return c.id }
func (c *Confirm) Resize(width, height int) { c.width = width }
func (c *Confirm) Changed() bool            { return false }

func (c *Confirm) Update(msg tea.Msg) (Action, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return Continue, nil
	}
	switch strings.ToLower(k.Keystroke()) {
	case "left", "right", "up", "down", "tab", "shift+tab", "space", "h", "l":
		c.yes = !c.yes
	case "o", "y":
		c.yes = true
	case "n":
		c.yes = false
	case "enter":
		if c.yes {
			return Submit, nil
		}
		return Cancel, nil
	case "esc":
		return Cancel, nil
	}
	return Continue, nil
}

func (c *Confirm) View() string {
	oui, non := "  Oui  ", "  Non  "
	if c.yes {
		oui, non = sReverse.Foreground(cAccent).Render(oui), sDim.Render(non)
	} else {
		oui, non = sDim.Render(oui), sReverse.Foreground(cAccent).Render(non)
	}
	q := lipgloss.NewStyle().Width(c.width).Render(c.question)
	if c.question == "" {
		return oui + "  " + non
	}
	return q + "\n\n" + oui + "  " + non
}

func (c *Confirm) Submit() (Values, bool) { return Values{}, c.yes }

func (c *Confirm) Keys() string {
	return "←→ ou o/n choisir · entrée valider · esc annuler"
}

// ConfirmTyped guards a destructive action: the id must be typed back before
// enter validates.
type ConfirmTyped struct {
	id, question, expected string
	in                     textinput.Model
	err                    string
	width                  int
}

// NewConfirmTyped asks question and wants expected typed back.
func NewConfirmTyped(id, question, expected string) *ConfirmTyped {
	return &ConfirmTyped{id: id, question: question, expected: expected, in: newInput(""), width: 60}
}

func (c *ConfirmTyped) ID() string { return c.id }

func (c *ConfirmTyped) Resize(width, height int) {
	c.width = width
	c.in.SetWidth(max(width-1, 1))
}

func (c *ConfirmTyped) Changed() bool { return false }

func (c *ConfirmTyped) Update(msg tea.Msg) (Action, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && k.Keystroke() == "enter" {
		if _, ok := c.Submit(); ok {
			return Submit, nil
		}
		return Continue, nil
	}
	var cmd tea.Cmd
	c.in, cmd = c.in.Update(msg)
	c.err = ""
	return Continue, cmd
}

func (c *ConfirmTyped) View() string {
	lines := []string{
		lipgloss.NewStyle().Width(c.width).Render(c.question),
		"",
		"Recopiez " + sBold.Render(c.expected) + " pour confirmer :",
		c.in.View(),
	}
	if c.err != "" {
		lines = append(lines, sError.Render(c.err))
	}
	return strings.Join(lines, "\n")
}

func (c *ConfirmTyped) Submit() (Values, bool) {
	if strings.TrimSpace(c.in.Value()) != c.expected {
		c.err = "ne correspond pas à " + c.expected
		return Values{}, false
	}
	return Values{}, true
}

func (c *ConfirmTyped) Keys() string { return "entrée confirmer · esc annuler" }

// newInput is a one-line editor without blink: arrows, word moves, ctrl+a,
// ctrl+e, ctrl+w, ctrl+k, ctrl+u, and paste.
func newInput(value string) textinput.Model {
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = 0
	st := textinput.DefaultStyles(darkBackground)
	st.Cursor.Blink = false
	st.Cursor.Color = cAccent
	in.SetStyles(st)
	in.SetValue(value)
	in.CursorEnd()
	in.Focus()
	return in
}
