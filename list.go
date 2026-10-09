package tuikit

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// List is a vertical list to pick one item: ↑↓ or j/k move, / filters, enter
// picks; the Value of the item is under its id in Values.
type List struct {
	id        string
	items     []Item
	sel       int
	offset    int
	filtering bool
	filter    textinput.Model
	width     int
	height    int
}

// NewList offers items; an item shows its Label, or its Value without one.
func NewList(id string, items []Item) *List {
	l := &List{id: id, items: items, filter: newInput(""), width: 60, height: 10}
	l.filter.Blur()
	return l
}

// Default puts the selection on the item whose Value is value.
func (l *List) Default(value string) *List {
	for i, it := range l.items {
		if it.Value == value {
			l.sel = i
		}
	}
	return l
}

func (l *List) ID() string    { return l.id }
func (l *List) Changed() bool { return false }

func (l *List) Resize(width, height int) {
	l.width, l.height = width, max(height, 1)
	l.filter.SetWidth(max(width-3, 1))
}

// shown are the indexes of the items matching the filter.
func (l *List) shown() []int {
	q := foldText(strings.TrimSpace(l.filter.Value()))
	var out []int
	for i, it := range l.items {
		if q == "" || strings.Contains(foldText(it.Value+" "+it.Label), q) {
			out = append(out, i)
		}
	}
	return out
}

func (l *List) Update(msg tea.Msg) (Action, tea.Cmd) {
	shown := l.shown()
	pos := max(indexOf(shown, l.sel), 0)
	k, isKey := msg.(tea.KeyPressMsg)
	if !isKey {
		if l.filtering {
			var cmd tea.Cmd
			l.filter, cmd = l.filter.Update(msg)
			l.keepSelection()
			return Continue, cmd
		}
		return Continue, nil
	}
	ks := k.Keystroke()
	switch {
	case ks == "enter":
		if len(shown) == 0 {
			return Continue, nil
		}
		return Submit, nil
	case ks == "up" || (!l.filtering && ks == "k"):
		pos--
	case ks == "down" || (!l.filtering && ks == "j"):
		pos++
	case ks == "pgup":
		pos -= l.rows()
	case ks == "pgdown":
		pos += l.rows()
	case ks == "home" || (!l.filtering && ks == "g"):
		pos = 0
	case ks == "end" || (!l.filtering && ks == "G"):
		pos = len(shown) - 1
	case !l.filtering && ks == "/":
		l.filtering = true
		return Continue, l.filter.Focus()
	case l.filtering && ks == "backspace" && l.filter.Value() == "":
		l.filtering = false
		l.filter.Blur()
		return Continue, nil
	case l.filtering:
		var cmd tea.Cmd
		l.filter, cmd = l.filter.Update(msg)
		l.keepSelection()
		return Continue, cmd
	default:
		return Continue, nil
	}
	if len(shown) > 0 {
		l.sel = shown[min(max(pos, 0), len(shown)-1)]
	}
	return Continue, nil
}

// keepSelection moves the selection to the first match when the filter hides it.
func (l *List) keepSelection() {
	shown := l.shown()
	if len(shown) > 0 && indexOf(shown, l.sel) < 0 {
		l.sel = shown[0]
	}
}

func indexOf(xs []int, x int) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

// rows is the room for items, less the filter line.
func (l *List) rows() int {
	if l.filtering {
		return max(l.height-1, 1)
	}
	return l.height
}

func (l *List) View() string {
	shown := l.shown()
	var lines []string
	if l.filtering {
		lines = append(lines, sFocus.Render("/ ")+l.filter.View())
	}
	rows := l.rows()
	pos := max(indexOf(shown, l.sel), 0)
	if pos < l.offset {
		l.offset = pos
	}
	if pos >= l.offset+rows {
		l.offset = pos - rows + 1
	}
	l.offset = min(max(l.offset, 0), max(len(shown)-rows, 0))
	if len(shown) == 0 {
		lines = append(lines, sDim.Render(tr("  aucun élément", "  no item")))
	}
	for i := l.offset; i < len(shown) && i < l.offset+rows; i++ {
		it := l.items[shown[i]]
		label := it.Label
		if label == "" {
			label = it.Value
		}
		if shown[i] == l.sel {
			lines = append(lines, sChosen.Render(truncate("› "+label, l.width)))
		} else {
			lines = append(lines, truncate("  "+label, l.width))
		}
	}
	return strings.Join(lines, "\n")
}

func (l *List) Submit() (Values, bool) {
	shown := l.shown()
	if len(shown) == 0 || indexOf(shown, l.sel) < 0 {
		return Values{}, false
	}
	var v Values
	v.Set(l.id, l.items[l.sel].Value)
	return v, true
}

func (l *List) Keys() string {
	if l.filtering {
		return tr("↑↓ déplacer · entrée choisir · ⌫ vide fin du filtre · esc annuler", "↑↓ move · enter pick · ⌫ on empty ends filter · esc cancel")
	}
	return tr("↑↓ ou j/k déplacer · / filtrer · entrée choisir · esc annuler", "↑↓ or j/k move · / filter · enter pick · esc cancel")
}
