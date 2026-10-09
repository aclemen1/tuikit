package tuikit

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/aclemen1/tuikit/when"
)

// Form shows all its fields at once, each with its label, help and error;
// tab and shift+tab go from field to field, enter goes on (and inserts a line
// in a TextArea), ctrl+s validates.
type Form struct {
	id        string
	fields    []*Field
	byID      map[string]*Field
	focus     int
	validate  func(Values) Errors
	now       func() time.Time
	width     int
	height    int
	submitted bool
	offset    int
	initial   string
}

// NewForm builds a form; id names its answer in DoneMsg.
func NewForm(id string, fields ...*Field) *Form {
	f := &Form{id: id, fields: fields, byID: map[string]*Field{}, now: time.Now, width: 60, height: 20}
	for _, x := range fields {
		f.byID[x.id] = x
		x.init(f)
	}
	f.initial = f.snapshot()
	f.focus = -1
	f.move(1)
	f.Resize(f.width, f.height)
	return f
}

// Validate adds a check run at validation, after each field's own.
func (f *Form) Validate(fn func(Values) Errors) *Form {
	f.validate = fn
	return f
}

// Now sets the clock dates are read against; time.Now by default.
func (f *Form) Now(fn func() time.Time) *Form {
	f.now = fn
	return f
}

// Field is the field id.
func (f *Form) Field(id string) *Field { return f.byID[id] }

func (f *Form) ID() string { return f.id }

func (f *Form) labelWidth() int {
	w := 0
	for _, x := range f.fields {
		w = max(w, lipgloss.Width(x.label))
	}
	return w + 4
}

func (f *Form) valueWidth() int { return max(f.width-f.labelWidth(), 10) }

func (f *Form) Resize(width, height int) {
	f.width, f.height = width, height
	vw := f.valueWidth()
	for _, x := range f.fields {
		switch {
		case x.textual():
			x.in.SetWidth(max(vw-1, 1))
		case x.kind == kTextArea:
			x.area.resize(vw, 3, max(min(8, height-4), 3))
		}
	}
}

func (f *Form) visible(x *Field) bool {
	if x.showID == "" {
		return true
	}
	t := f.byID[x.showID]
	return t != nil && f.visible(t) && slices.Contains(x.showValues, t.current())
}

// move gives the focus to the next visible field in direction d; it stays
// put at either end.
func (f *Form) move(d int) tea.Cmd {
	for i := f.focus + d; i >= 0 && i < len(f.fields); i += d {
		if f.visible(f.fields[i]) {
			return f.focusOn(i)
		}
	}
	return nil
}

func (f *Form) focusOn(i int) tea.Cmd {
	if f.focus >= 0 && f.focus < len(f.fields) {
		f.fields[f.focus].blur()
	}
	f.focus = i
	return f.fields[i].focus()
}

func (f *Form) current() *Field {
	if f.focus < 0 || f.focus >= len(f.fields) {
		return nil
	}
	return f.fields[f.focus]
}

func (f *Form) Update(msg tea.Msg) (Action, tea.Cmd) {
	x := f.current()
	if x == nil {
		return Continue, nil
	}
	now := f.now()
	var cmd tea.Cmd
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch ks := k.Keystroke(); {
		case ks == "tab":
			cmd = f.move(1)
		case ks == "shift+tab":
			cmd = f.move(-1)
		case ks == "enter" && x.kind != kTextArea:
			if x.kind == kRef {
				x.accept()
			}
			cmd = f.move(1)
		case (ks == "up" || ks == "down") && x.kind == kRef && (x.suggOpen || ks == "down"):
			x.navigate(ks)
		case (ks == "up" || ks == "down") && x.kind != kTextArea:
			cmd = f.move(map[string]int{"up": -1, "down": 1}[ks])
		default:
			cmd = x.update(msg, now)
		}
	} else {
		cmd = x.update(msg, now)
	}
	if f.submitted {
		f.check()
	}
	return Continue, cmd
}

// navigate moves in the suggestions of a Ref, opening them on ↓.
func (x *Field) navigate(ks string) {
	if x.complete == nil {
		return
	}
	if !x.suggOpen {
		q := strings.TrimSpace(x.in.Value())
		x.sugg = x.complete(q)
		if len(x.sugg) > maxSuggestions {
			x.sugg = x.sugg[:maxSuggestions]
		}
		x.suggOpen = len(x.sugg) > 0
		x.sel = 0
		return
	}
	switch ks {
	case "down":
		x.sel = min(x.sel+1, len(x.sugg)-1)
	case "up":
		x.sel--
		if x.sel < 0 {
			x.suggOpen = false
		}
	}
}

// check reads every visible field and sets the error shown under each; it
// returns the answers and the index of the first field in error, or -1.
func (f *Form) check() (Values, int) {
	now := f.now()
	var v Values
	errs := map[string]string{}
	for _, x := range f.fields {
		if !f.visible(x) {
			continue
		}
		val, has, err := x.parse(now)
		switch {
		case err != nil:
			errs[x.id] = err.Error()
		case !has && x.required:
			errs[x.id] = "obligatoire"
		case has:
			v.set(x.id, val)
		}
	}
	if f.validate != nil && len(errs) == 0 {
		for id, msg := range f.validate(v) {
			errs[id] = msg
		}
	}
	first := -1
	for i, x := range f.fields {
		x.err = ""
		if msg, ok := errs[x.id]; ok && f.visible(x) {
			x.err = msg
			if first < 0 {
				first = i
			}
		}
	}
	return v, first
}

func (f *Form) Submit() (Values, bool) {
	f.submitted = true
	v, first := f.check()
	if first >= 0 {
		f.focusOn(first)
		return Values{}, false
	}
	return v, true
}

func (f *Form) snapshot() string {
	parts := make([]string, len(f.fields))
	for i, x := range f.fields {
		parts[i] = x.current()
	}
	return strings.Join(parts, "\x00")
}

func (f *Form) Changed() bool { return f.snapshot() != f.initial }

func (f *Form) Keys() string {
	k := "tab champ suivant · ctrl+s valider · esc annuler"
	if x := f.current(); x != nil {
		k = x.keys() + " · " + k
	}
	return k
}

// fieldLines draws the value column of a field: its editor, then its live
// reading, suggestions or calendar, its help and its error.
func (f *Form) fieldLines(x *Field, focused bool, now time.Time) []string {
	vw := f.valueWidth()
	var lines []string
	switch {
	case x.textual():
		lines = append(lines, x.in.View())
	case x.kind == kTextArea:
		lines = append(lines, strings.Split(x.area.view(), "\n")...)
	case x.kind == kChoice:
		lines = append(lines, x.choiceView(focused))
	case x.kind == kBool:
		box := "[ ] non"
		if x.on {
			box = "[x] oui"
		}
		if focused {
			box = sChosen.Render(box)
		}
		lines = append(lines, box)
	}
	raw := strings.TrimSpace(x.current())
	if raw != "" && x.err == "" {
		if _, _, err := x.parse(now); err != nil && x.kind != kRef {
			lines = append(lines, sError.Render(wrapText(err.Error(), vw)))
		} else if r := f.reading(x, now); r != "" {
			lines = append(lines, sOK.Render(truncate("→ "+r, vw)))
		}
	}
	if x.kind == kRef && focused && x.suggOpen {
		for i, it := range x.sugg {
			label := it.Value
			if it.Label != "" && it.Label != it.Value {
				label += " · " + it.Label
			}
			if i == x.sel {
				lines = append(lines, sChosen.Render(truncate("› "+label, vw)))
			} else {
				lines = append(lines, sDim.Render(truncate("  "+label, vw)))
			}
		}
	}
	if x.kind == kDate && focused {
		lines = append(lines, calendar(x, now)...)
	}
	if x.help != "" {
		lines = append(lines, sDim.Render(wrapText(x.help, vw)))
	}
	if x.err != "" {
		lines = append(lines, sError.Render(wrapText("! "+x.err, vw)))
	}
	return strings.Split(strings.Join(lines, "\n"), "\n")
}

// reading is the live interpretation of a valid field.
func (f *Form) reading(x *Field, now time.Time) string {
	v, has, err := x.parse(now)
	if err != nil || !has {
		return ""
	}
	switch x.kind {
	case kDate:
		return when.Describe(v.moment, now, x.clock)
	case kClock:
		return v.raw
	case kDuration:
		return when.Words(v.duration)
	case kDurations:
		parts := make([]string, len(v.durations))
		var ref time.Time
		if src := f.byID[x.from]; src != nil && src.kind == kDate {
			if sv, ok, _ := src.parse(now); ok {
				ref = sv.at
			}
		}
		for i, d := range v.durations {
			if ref.IsZero() {
				parts[i] = when.Words(d)
			} else {
				parts[i] = when.FormatDuration(d) + " " + when.Day(when.BeforeBy(ref, d))
			}
		}
		return strings.Join(parts, " · ")
	}
	return ""
}

func (x *Field) choiceView(focused bool) string {
	parts := make([]string, len(x.options))
	for i, o := range x.options {
		mark := "( ) "
		if i == x.choice {
			mark = "(•) "
		}
		switch {
		case i == x.choice && focused:
			parts[i] = sChosen.Render(mark + o)
		case i == x.choice:
			parts[i] = sBold.Render(mark + o)
		default:
			parts[i] = sDim.Render(mark + o)
		}
	}
	return strings.Join(parts, "   ")
}

var monthsFR = []string{"", "janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}

// calendar draws the month of the date a Date field reads, or of today; the
// date is reversed, today underlined.
func calendar(x *Field, now time.Time) []string {
	sel := time.Time{}
	if m, err := when.Parse(x.in.Value(), now); err == nil {
		sel = m.Time(now.Location(), "")
	}
	shown := sel
	if shown.IsZero() {
		shown = now
	}
	first := time.Date(shown.Year(), shown.Month(), 1, 0, 0, 0, 0, now.Location())
	lines := []string{sDim.Render(fmt.Sprintf("%s %d", monthsFR[shown.Month()], shown.Year())), sDim.Render("lu ma me je ve sa di")}
	offset := (int(first.Weekday()) + 6) % 7
	row := strings.Repeat("   ", offset)
	days := first.AddDate(0, 1, -1).Day()
	for d := 1; d <= days; d++ {
		cell := fmt.Sprintf("%2d", d)
		day := time.Date(shown.Year(), shown.Month(), d, 0, 0, 0, 0, now.Location())
		sameDay := func(t time.Time) bool {
			return !t.IsZero() && t.Year() == day.Year() && t.YearDay() == day.YearDay()
		}
		switch {
		case sameDay(sel):
			cell = sReverse.Foreground(cAccent).Render(cell)
		case sameDay(now):
			cell = lipgloss.NewStyle().Underline(true).Bold(true).Render(cell)
		}
		row += cell
		if (offset+d)%7 == 0 || d == days {
			lines = append(lines, row)
			row = ""
		} else {
			row += " "
		}
	}
	return lines
}

func wrapText(s string, width int) string {
	return lipgloss.NewStyle().Width(max(width, 1)).Render(s)
}

func (f *Form) View() string {
	now := f.now()
	lw := f.labelWidth()
	var all []string
	start, end := 0, 0
	for i, x := range f.fields {
		if !f.visible(x) {
			continue
		}
		if len(all) > 0 {
			all = append(all, "")
		}
		focused := i == f.focus
		label := "  " + x.label
		if focused {
			label = sFocus.Render("› " + x.label)
		}
		if x.required {
			label += sDim.Render(" *")
		}
		label = lipgloss.NewStyle().Width(lw).Render(label)
		lines := f.fieldLines(x, focused, now)
		if focused {
			start, end = len(all), len(all)+len(lines)
		}
		for j, l := range lines {
			if j == 0 {
				all = append(all, label+l)
			} else {
				all = append(all, strings.Repeat(" ", lw)+l)
			}
		}
	}
	h := max(f.height, 1)
	if len(all) <= h {
		f.offset = 0
		return strings.Join(all, "\n")
	}
	if start < f.offset {
		f.offset = start
	}
	if end > f.offset+h {
		f.offset = min(end-h, start)
	}
	f.offset = min(max(f.offset, 0), len(all)-h)
	out := slices.Clone(all[f.offset : f.offset+h])
	if f.offset > 0 {
		out[0] = sDim.Render("  ↑ …")
	}
	if f.offset+h < len(all) {
		out[h-1] = sDim.Render("  ↓ …")
	}
	return strings.Join(out, "\n")
}
