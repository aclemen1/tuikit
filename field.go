package tuikit

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/aclemen1/tuikit/when"
)

type kind int

const (
	kText kind = iota
	kTextArea
	kChoice
	kBool
	kDate
	kClock
	kDuration
	kDurations
	kRef
	kRefs
)

// Item is a proposal of a Completer: Value goes into the field, Label is shown.
type Item struct{ Value, Label string }

// Completer proposes the items matching what is typed in a Ref field.
type Completer func(query string) []Item

const maxSuggestions = 6

// Field is one input of a Form, built by Text, TextArea, Choice, Bool, Date,
// Clock, Duration, Durations or Ref, then refined by its methods.
type Field struct {
	id, label, help, placeholder string
	kind                         kind
	required, strict, allowNone  bool
	options                      []string
	clock                        string
	from                         string
	complete                     Completer
	showID                       string
	showValues                   []string
	def                          any

	form     *Form
	in       textinput.Model
	area     *area
	choice   int
	on       bool
	sugg     []Item
	sel      int
	suggOpen bool
	items    []string
	err      string
}

func newField(k kind, id, label string) *Field {
	return &Field{kind: k, id: id, label: label, choice: -1, sel: -1}
}

// Text is a one-line text.
func Text(id, label string) *Field { return newField(kText, id, label) }

// TextArea is a multiline text with soft wrap and a scroll bar; enter inserts
// a line.
func TextArea(id, label string) *Field { return newField(kTextArea, id, label) }

// Choice is a closed list shown on one line; ←→ or a first letter choose.
// Without Default, a required Choice starts with nothing chosen and any other
// starts on its first option.
func Choice(id, label string, options ...string) *Field {
	f := newField(kChoice, id, label)
	f.options = options
	return f
}

// Bool is a check box; space toggles, o/n set.
func Bool(id, label string) *Field { return newField(kBool, id, label) }

// Date is a date typed freely or picked in a calendar, with its reading shown
// live: « vendredi prochain », « demain 14h », « 12.11 », « +3j », ISO.
func Date(id, label string) *Field { return newField(kDate, id, label) }

// Clock is a time of day typed freely: 14h30, 14:30, midi.
func Clock(id, label string) *Field { return newField(kClock, id, label) }

// Duration is a duration typed freely: 1h30, 45min, 3j, 2sem.
func Duration(id, label string) *Field { return newField(kDuration, id, label) }

// Durations is a comma-separated list of durations: « 30d, 7d, 1d ».
func Durations(id, label string) *Field { return newField(kDurations, id, label) }

// Ref is a reference (dossier, person, sitting) with completion from complete.
func Ref(id, label string, complete Completer) *Field {
	f := newField(kRef, id, label)
	f.complete = complete
	return f
}

// Refs is a list of references, each completed as in Ref: enter or a comma
// adds the value typed, backspace on an empty entry removes the last one.
func Refs(id, label string, complete Completer) *Field {
	f := newField(kRefs, id, label)
	f.complete = complete
	return f
}

// Required refuses an empty field at validation; a Refs needs one value.
func (f *Field) Required() *Field { f.required = true; return f }

// Help is the line shown under the field.
func (f *Field) Help(s string) *Field { f.help = s; return f }

// Placeholder is the grey text of an empty field.
func (f *Field) Placeholder(s string) *Field { f.placeholder = s; return f }

// Default prefills the field: a string for any field, a bool for a Bool, a
// time.Time or a when.Moment for a Date, a time.Duration for a Duration, a
// []time.Duration for Durations, a []string for Refs.
func (f *Field) Default(v any) *Field { f.def = v; return f }

// Clock is the time a Date takes when typed without one (15:04).
func (f *Field) Clock(defaultClock string) *Field { f.clock = defaultClock; return f }

// From previews the dates of a Durations list counted back from the Date
// field dateID.
func (f *Field) From(dateID string) *Field { f.from = dateID; return f }

// AllowNone lets a Date say there is none: « aucune », « jamais », « none »,
// « never ». Values.None tells it, and Values.String reads « none ».
func (f *Field) AllowNone() *Field { f.allowNone = true; return f }

var noneWords = []string{"aucune", "aucun", "jamais", "none", "never"}

// Strict refuses a Ref value the completer does not propose.
func (f *Field) Strict() *Field { f.strict = true; return f }

// ShowIf shows the field only while the field id holds one of values.
func (f *Field) ShowIf(id string, values ...string) *Field {
	f.showID, f.showValues = id, values
	return f
}

func (f *Field) textual() bool {
	switch f.kind {
	case kText, kDate, kClock, kDuration, kDurations, kRef, kRefs:
		return true
	}
	return false
}

// init builds the editors of the field, holding its default.
func (f *Field) init(form *Form) {
	f.form = form
	raw := ""
	switch d := f.def.(type) {
	case string:
		raw = d
	case bool:
		f.on = d
	case time.Time:
		raw = d.Format("02.01.2006 15:04")
	case when.Moment:
		if t, err := time.Parse("2006-01-02", d.Date); err == nil {
			raw = t.Format("02.01.2006")
			if !d.AllDay {
				raw += " " + d.Clock
			}
		}
	case time.Duration:
		raw = when.FormatDuration(d)
	case []time.Duration:
		parts := make([]string, len(d))
		for i, x := range d {
			parts[i] = when.FormatDuration(x)
		}
		raw = strings.Join(parts, ", ")
	case []string:
		for _, x := range d {
			f.add(x)
		}
	}
	if f.kind == kRefs && raw != "" {
		for x := range strings.SplitSeq(raw, ",") {
			f.add(x)
		}
		raw = ""
	}
	switch {
	case f.textual():
		f.in = newInput(raw)
		f.in.Placeholder = f.placeholder
		f.in.Blur()
	case f.kind == kTextArea:
		f.area = newArea(raw)
		f.area.ta.Placeholder = f.placeholder
		f.area.blur()
	case f.kind == kChoice:
		f.choice = slices.Index(f.options, raw)
		if f.choice < 0 && raw == "" && !f.required && len(f.options) > 0 {
			f.choice = 0
		}
	case f.kind == kBool:
		if raw != "" {
			f.on = raw == "oui" || raw == "true"
		}
	}
}

// current is the state of the field as text, for ShowIf and change detection.
func (f *Field) current() string {
	switch {
	case f.kind == kRefs:
		all := slices.Clone(f.items)
		if s := strings.TrimSpace(f.in.Value()); s != "" {
			all = append(all, s)
		}
		return strings.Join(all, ", ")
	case f.textual():
		return f.in.Value()
	case f.kind == kTextArea:
		return f.area.value()
	case f.kind == kChoice:
		if f.choice < 0 {
			return ""
		}
		return f.options[f.choice]
	case f.kind == kBool:
		if f.on {
			return "oui"
		}
		return "non"
	}
	return ""
}

func (f *Field) focus() tea.Cmd {
	switch {
	case f.textual():
		return f.in.Focus()
	case f.kind == kTextArea:
		return f.area.focus()
	}
	return nil
}

func (f *Field) blur() {
	switch {
	case f.textual():
		f.in.Blur()
	case f.kind == kTextArea:
		f.area.blur()
	}
	f.suggOpen = false
}

// parse reads the field: has is false for an empty field.
func (f *Field) parse(now time.Time) (v value, has bool, err error) {
	raw := strings.TrimSpace(f.current())
	if f.kind == kBool {
		return value{raw: raw, on: f.on}, true, nil
	}
	if raw == "" {
		return value{}, false, nil
	}
	switch f.kind {
	case kDate:
		if f.allowNone && slices.Contains(noneWords, strings.ToLower(raw)) {
			return value{raw: "none", none: true}, true, nil
		}
		m, err := when.Parse(raw, now)
		if err != nil {
			return value{}, false, err
		}
		return value{raw: m.String(), moment: m, at: m.Time(now.Location(), f.clock)}, true, nil
	case kClock:
		c, err := when.ParseClock(raw)
		if err != nil {
			return value{}, false, err
		}
		return value{raw: c}, true, nil
	case kDuration:
		d, err := when.ParseDuration(raw)
		if err != nil {
			return value{}, false, err
		}
		return value{raw: when.FormatDuration(d), duration: d}, true, nil
	case kDurations:
		ds, err := when.ParseDurations(raw)
		if err != nil {
			return value{}, false, err
		}
		parts := make([]string, len(ds))
		for i, d := range ds {
			parts[i] = when.FormatDuration(d)
		}
		return value{raw: strings.Join(parts, ","), durations: ds}, true, nil
	case kRef:
		if f.strict && !f.proposes(raw) {
			return value{}, false, fmt.Errorf(tr("%q n'est pas proposé", "%q is not offered"), raw)
		}
	case kRefs:
		all := strings.Split(raw, ", ")
		if f.strict {
			for _, x := range all {
				if !f.proposes(x) {
					return value{}, false, fmt.Errorf(tr("%q n'est pas proposé", "%q is not offered"), x)
				}
			}
		}
		return value{raw: strings.Join(all, ","), strings: all}, true, nil
	}
	return value{raw: raw}, true, nil
}

func (f *Field) proposes(v string) bool {
	if f.complete == nil {
		return false
	}
	for _, it := range f.complete(v) {
		if it.Value == v {
			return true
		}
	}
	return false
}

func (f *Field) refreshSuggestions() {
	if f.complete == nil {
		return
	}
	q := strings.TrimSpace(f.in.Value())
	f.sugg = f.proposals(q)
	if len(f.sugg) > maxSuggestions {
		f.sugg = f.sugg[:maxSuggestions]
	}
	f.suggOpen = q != "" && len(f.sugg) > 0 && !(len(f.sugg) == 1 && f.sugg[0].Value == q)
	f.sel = -1
	if f.suggOpen {
		f.sel = 0
	}
}

// proposals are the completer's items for q, less the values a Refs holds.
func (f *Field) proposals(q string) []Item {
	var out []Item
	for _, it := range f.complete(q) {
		if !slices.Contains(f.items, it.Value) {
			out = append(out, it)
		}
	}
	return out
}

// add appends a value to a Refs, once.
func (f *Field) add(v string) {
	v = strings.TrimSpace(v)
	if v != "" && !slices.Contains(f.items, v) {
		f.items = append(f.items, v)
	}
}

// commit adds to a Refs the chosen suggestion, or else the text typed; it
// tells whether there was anything to add.
func (f *Field) commit() bool {
	v := strings.TrimSpace(f.in.Value())
	if f.suggOpen && f.sel >= 0 && f.sel < len(f.sugg) {
		v = f.sugg[f.sel].Value
	}
	f.suggOpen, f.sel = false, -1
	if v == "" {
		return false
	}
	f.add(v)
	f.in.SetValue("")
	return true
}

func (f *Field) accept() {
	if f.suggOpen && f.sel >= 0 && f.sel < len(f.sugg) {
		f.in.SetValue(f.sugg[f.sel].Value)
		f.in.CursorEnd()
	}
	f.suggOpen = false
	f.sel = -1
}

// shiftDate moves a Date by days and months from what it reads, today when
// it reads nothing, and keeps its time.
func (f *Field) shiftDate(days, months int, now time.Time) {
	base := now
	clock := ""
	if m, err := when.Parse(f.in.Value(), now); err == nil {
		base = m.Time(now.Location(), "")
		if !m.AllDay {
			clock = m.Clock
		}
	}
	t := base.AddDate(0, months, days)
	s := t.Format("02.01.2006")
	if clock != "" {
		s += " " + clock
	}
	f.in.SetValue(s)
	f.in.CursorEnd()
}

// update feeds a key or a paste to the field; navigation keys are the form's.
func (f *Field) update(msg tea.Msg, now time.Time) tea.Cmd {
	switch f.kind {
	case kTextArea:
		return f.area.update(msg)
	case kChoice:
		k, ok := msg.(tea.KeyPressMsg)
		if !ok || len(f.options) == 0 {
			return nil
		}
		n := len(f.options)
		switch k.Keystroke() {
		case "left":
			if f.choice < 0 {
				f.choice = n - 1
			} else {
				f.choice = (f.choice - 1 + n) % n
			}
		case "right", "space":
			f.choice = (f.choice + 1) % n
		default:
			if k.Text != "" {
				f.jump(k.Text)
			}
		}
		return nil
	case kBool:
		k, ok := msg.(tea.KeyPressMsg)
		if !ok {
			return nil
		}
		switch strings.ToLower(k.Keystroke()) {
		case "space", "left", "right", "x":
			f.on = !f.on
		case "o", "y":
			f.on = true
		case "n":
			f.on = false
		}
		return nil
	}
	if k, ok := msg.(tea.KeyPressMsg); ok && f.kind == kDate {
		switch k.Keystroke() {
		case "ctrl+left":
			f.shiftDate(-1, 0, now)
			return nil
		case "ctrl+right":
			f.shiftDate(1, 0, now)
			return nil
		case "ctrl+up":
			f.shiftDate(-7, 0, now)
			return nil
		case "ctrl+down":
			f.shiftDate(7, 0, now)
			return nil
		case "pgup":
			f.shiftDate(0, -1, now)
			return nil
		case "pgdown":
			f.shiftDate(0, 1, now)
			return nil
		}
	}
	if f.kind == kRefs {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch {
			case msg.Text == ",":
				f.commit()
				return nil
			case msg.Keystroke() == "backspace" && f.in.Value() == "" && len(f.items) > 0:
				f.items = f.items[:len(f.items)-1]
				return nil
			}
		case tea.PasteMsg:
			parts := strings.FieldsFunc(msg.Content, func(r rune) bool { return r == ',' || r == '\n' })
			last := ""
			if n := len(parts); n > 0 && !strings.ContainsAny(msg.Content[len(msg.Content)-1:], ",\n") {
				last, parts = parts[n-1], parts[:n-1]
			}
			for _, p := range parts {
				f.add(p)
			}
			f.in.SetValue(f.in.Value() + strings.TrimSpace(last))
			f.in.CursorEnd()
			f.refreshSuggestions()
			return nil
		}
	}
	before := f.in.Value()
	var cmd tea.Cmd
	f.in, cmd = f.in.Update(msg)
	if (f.kind == kRef || f.kind == kRefs) && f.in.Value() != before {
		f.refreshSuggestions()
	}
	return cmd
}

// jump chooses the next option after the current one starting with s.
func (f *Field) jump(s string) {
	s = foldText(s)
	n := len(f.options)
	for i := 1; i <= n; i++ {
		j := (f.choice + i + n) % n
		if f.choice < 0 {
			j = i - 1
		}
		if strings.HasPrefix(foldText(f.options[j]), s) {
			f.choice = j
			return
		}
	}
}

var foldTextReplacer = strings.NewReplacer("à", "a", "â", "a", "é", "e", "è", "e", "ê", "e", "î", "i", "ô", "o", "ù", "u", "û", "u", "ç", "c")

func foldText(s string) string { return foldTextReplacer.Replace(strings.ToLower(s)) }

// keys is the help for the focused field.
func (f *Field) keys() string {
	switch f.kind {
	case kTextArea:
		return tr("entrée nouvelle ligne", "enter new line")
	case kDate:
		return tr("ctrl+flèches jour/semaine · pgup/pgdn mois", "ctrl+arrows day/week · pgup/pgdn month")
	case kChoice:
		return tr("←→ choisir", "←→ choose")
	case kBool:
		return tr("espace cocher", "space toggle")
	case kRef:
		return tr("↑↓ suggestions · entrée choisir", "↑↓ suggestions · enter pick")
	case kRefs:
		return tr("↑↓ suggestions · entrée ou virgule ajouter · ⌫ retirer", "↑↓ suggestions · enter or comma add · ⌫ remove")
	}
	return tr("entrée suivant", "enter next")
}
