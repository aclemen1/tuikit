package tuikit

import (
	"time"

	"github.com/aclemen1/tuikit/when"
)

// Values are the answers of a submitted form, by field id. A field hidden by
// ShowIf, or left empty, has no value.
type Values struct {
	m map[string]value
}

type value struct {
	raw       string
	on        bool
	moment    when.Moment
	at        time.Time
	duration  time.Duration
	durations []time.Duration
}

func (v Values) get(id string) value {
	if v.m == nil {
		return value{}
	}
	return v.m[id]
}

func (v *Values) set(id string, x value) {
	if v.m == nil {
		v.m = map[string]value{}
	}
	v.m[id] = x
}

// Has tells whether the field has a value.
func (v Values) Has(id string) bool {
	_, ok := v.m[id]
	return ok
}

// String is the text of a field as typed (trimmed), the chosen option of a
// Choice, the value of a Ref, a Clock as 15:04, a Date as 2006-01-02 or
// 2006-01-02T15:04, a Bool as « oui » or « non ».
func (v Values) String(id string) string { return v.get(id).raw }

// Bool is the state of a Bool field.
func (v Values) Bool(id string) bool { return v.get(id).on }

// Moment is the date of a Date field, as typed: a date alone keeps AllDay.
func (v Values) Moment(id string) (when.Moment, bool) {
	x, ok := v.m[id]
	return x.moment, ok && x.moment.Date != ""
}

// Time is the date of a Date field placed in time, a date alone taking the
// field's Clock.
func (v Values) Time(id string) time.Time { return v.get(id).at }

// Duration is the value of a Duration field.
func (v Values) Duration(id string) time.Duration { return v.get(id).duration }

// Durations is the value of a Durations field.
func (v Values) Durations(id string) []time.Duration { return v.get(id).durations }

// Errors are messages shown under fields, by field id.
type Errors map[string]string
