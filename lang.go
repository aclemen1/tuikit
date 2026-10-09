package tuikit

import "github.com/aclemen1/tuikit/when"

// Language is the language of the texts tuikit shows: questions, help lines,
// errors, date readings.
type Language = when.Language

const (
	French  = when.French
	English = when.English
)

var lang = French

// SetLanguage picks the language of tuikit and of its when package, once at
// the start of the TUI; French by default. Dates are read in French and in
// English whatever it is.
func SetLanguage(l Language) {
	lang = l
	when.SetLanguage(l)
}

func tr(fr, en string) string {
	if lang == English {
		return en
	}
	return fr
}
