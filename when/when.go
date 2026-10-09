// Package when reads dates, clock times and durations typed in free French
// (« vendredi prochain », « demain 14h », « 12.11 », « +3j », « 1h30 ») or in
// ISO and RFC 3339, and describes them back in French.
package when

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrEmpty is returned for a blank input.
var ErrEmpty = errors.New("vide")

// Moment is a point in time given as a date alone or as a date and a time.
type Moment struct {
	Date   string // 2006-01-02
	Clock  string // 15:04, empty for a date alone
	AllDay bool
}

// String is the stored form: 2026-11-15 or 2026-11-15T14:00.
func (m Moment) String() string {
	if m.AllDay {
		return m.Date
	}
	return m.Date + "T" + m.Clock
}

// Time places the moment in loc; a date alone takes defaultClock (15:04), or
// midnight when defaultClock is empty.
func (m Moment) Time(loc *time.Location, defaultClock string) time.Time {
	c := m.Clock
	if m.AllDay {
		c = defaultClock
	}
	if c != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04", m.Date+" "+c, loc); err == nil {
			return t
		}
	}
	t, _ := time.ParseInLocation("2006-01-02", m.Date, loc)
	return t
}

func dateOnly(t time.Time) Moment {
	return Moment{Date: t.Format("2006-01-02"), AllDay: true}
}

func dateClock(t time.Time) Moment {
	return Moment{Date: t.Format("2006-01-02"), Clock: t.Format("15:04")}
}

// fold lowercases s, drops French accents and squeezes spaces.
var foldReplacer = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i", "ô", "o", "ö", "o", "ù", "u", "û", "u", "ü", "u", "ç", "c",
	"’", "'", " ", " ",
)

func fold(s string) string {
	return strings.Join(strings.Fields(foldReplacer.Replace(strings.ToLower(s))), " ")
}

var clockRe = regexp.MustCompile(`^(\d{1,2})(?:h(\d{2})?|:(\d{2}))$`)

// ParseClock reads a time of day: 14h30, 14h, 14:30, 9:05, midi, minuit.
// It returns 15:04.
func ParseClock(s string) (string, error) {
	f := fold(s)
	switch f {
	case "":
		return "", ErrEmpty
	case "midi":
		return "12:00", nil
	case "minuit":
		return "00:00", nil
	}
	f = strings.ReplaceAll(f, " ", "")
	m := clockRe.FindStringSubmatch(f)
	if m == nil {
		return "", fmt.Errorf("heure %q illisible : essayez 14h30 ou 14:30", s)
	}
	h, _ := strconv.Atoi(m[1])
	mi := 0
	if m[2] != "" {
		mi, _ = strconv.Atoi(m[2])
	}
	if m[3] != "" {
		mi, _ = strconv.Atoi(m[3])
	}
	if h > 23 || mi > 59 {
		return "", fmt.Errorf("heure %q hors de 00:00–23:59", s)
	}
	return fmt.Sprintf("%02d:%02d", h, mi), nil
}

const day = 24 * time.Hour

var unitNames = map[string]time.Duration{
	"min": time.Minute, "mn": time.Minute, "minute": time.Minute, "minutes": time.Minute,
	"h": time.Hour, "heure": time.Hour, "heures": time.Hour,
	"j": day, "d": day, "jour": day, "jours": day, "day": day, "days": day,
	"w": 7 * day, "sem": 7 * day, "semaine": 7 * day, "semaines": 7 * day, "week": 7 * day, "weeks": 7 * day,
}

var amountRe = regexp.MustCompile(`^(\d+)\s*([a-z]*)\.?\s*`)

// ParseDuration reads a duration: 30min, 2h, 1h30, 1 h 30, 90 min, 3j, 7d,
// 2 semaines, 2w, 1j 2h. Days count 24 hours.
func ParseDuration(s string) (time.Duration, error) {
	f := fold(s)
	if f == "" {
		return 0, ErrEmpty
	}
	rest := f
	var total time.Duration
	var last time.Duration
	for rest != "" {
		m := amountRe.FindStringSubmatch(rest)
		if m == nil {
			return 0, fmt.Errorf("durée %q illisible : essayez 1h30, 45min, 3j ou 2sem", s)
		}
		n, _ := strconv.Atoi(m[1])
		unit, ok := unitNames[m[2]]
		switch {
		case m[2] == "" && last == time.Hour:
			unit, ok = time.Minute, true
		case !ok:
			return 0, fmt.Errorf("durée %q illisible : essayez 1h30, 45min, 3j ou 2sem", s)
		}
		total += time.Duration(n) * unit
		last = unit
		rest = rest[len(m[0]):]
	}
	return total, nil
}

// ParseDurations reads a comma-separated list of durations: « 30d, 7d, 1d ».
func ParseDurations(s string) ([]time.Duration, error) {
	if strings.TrimSpace(s) == "" {
		return nil, ErrEmpty
	}
	var out []time.Duration
	for part := range strings.SplitSeq(s, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		d, err := ParseDuration(part)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// FormatDuration writes d in the compact form ParseDuration reads back:
// 30d, 2h, 45min, 1h30.
func FormatDuration(d time.Duration) string {
	switch {
	case d%day == 0:
		return fmt.Sprintf("%dd", d/day)
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d < time.Hour:
		return fmt.Sprintf("%dmin", d/time.Minute)
	default:
		return fmt.Sprintf("%dh%02d", d/time.Hour, (d%time.Hour)/time.Minute)
	}
}

// Words writes d in French words: « 2 jours », « 1 h 30 », « 45 min ».
func Words(d time.Duration) string {
	switch {
	case d%(7*day) == 0 && d >= 7*day:
		return plural(int(d/(7*day)), "semaine", "semaines")
	case d%day == 0 && d >= day:
		return plural(int(d/day), "jour", "jours")
	case d < time.Hour:
		return fmt.Sprintf("%d min", d/time.Minute)
	case d%time.Hour == 0:
		return fmt.Sprintf("%d h", d/time.Hour)
	default:
		return fmt.Sprintf("%d h %02d", d/time.Hour, (d%time.Hour)/time.Minute)
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// Before moves t back by a duration; days and weeks keep the wall-clock time
// across a change of daylight saving time.
func Before(t time.Time, s string) (time.Time, error) {
	d, err := ParseDuration(s)
	if err != nil {
		return t, err
	}
	return BeforeBy(t, d), nil
}

// BeforeBy is Before for a parsed duration.
func BeforeBy(t time.Time, d time.Duration) time.Time {
	if d%day == 0 {
		return t.AddDate(0, 0, -int(d/day))
	}
	return t.Add(-d)
}

var monthRe = regexp.MustCompile(`^(\d+)\s*(mois|ans?|annees?|y)$`)

// delay reads « +3j », « dans 2 semaines », « 3d », « +1h30 », « dans 2 mois ».
// A bare amount needs a unit that is not an hour, so that « 14h » stays a clock.
func delay(f string, now time.Time) (Moment, bool) {
	rest, prefixed := f, false
	switch {
	case strings.HasPrefix(rest, "+"):
		rest, prefixed = strings.TrimSpace(rest[1:]), true
	case strings.HasPrefix(rest, "dans "):
		rest, prefixed = rest[len("dans "):], true
	}
	if m := monthRe.FindStringSubmatch(rest); m != nil {
		n, _ := strconv.Atoi(m[1])
		if m[2] == "mois" {
			return dateOnly(now.AddDate(0, n, 0)), true
		}
		return dateOnly(now.AddDate(n, 0, 0)), true
	}
	if !prefixed && clockRe.MatchString(strings.ReplaceAll(rest, " ", "")) {
		return Moment{}, false
	}
	d, err := ParseDuration(rest)
	if err != nil || d == 0 {
		return Moment{}, false
	}
	if d%day == 0 {
		return dateOnly(now.AddDate(0, 0, int(d/day))), true
	}
	days := int(d / day)
	return dateClock(now.AddDate(0, 0, days).Add(d - time.Duration(days)*day)), true
}

var weekdayNames = map[string]time.Weekday{
	"dimanche": time.Sunday, "dim": time.Sunday,
	"lundi": time.Monday, "lun": time.Monday,
	"mardi": time.Tuesday, "mar": time.Tuesday,
	"mercredi": time.Wednesday, "mer": time.Wednesday,
	"jeudi": time.Thursday, "jeu": time.Thursday,
	"vendredi": time.Friday, "ven": time.Friday,
	"samedi": time.Saturday, "sam": time.Saturday,
}

var monthNames = map[string]time.Month{
	"janvier": 1, "janv": 1, "jan": 1,
	"fevrier": 2, "fevr": 2, "fev": 2,
	"mars": 3, "mar": 3,
	"avril": 4, "avr": 4,
	"mai":     5,
	"juin":    6,
	"juillet": 7, "juil": 7,
	"aout":      8,
	"septembre": 9, "sept": 9, "sep": 9,
	"octobre": 10, "oct": 10,
	"novembre": 11, "nov": 11,
	"decembre": 12, "dec": 12,
}

// nextWeekday is the first wd strictly after today, or from today on when
// inclusive.
func nextWeekday(now time.Time, wd time.Weekday, inclusive bool) time.Time {
	n := (int(wd) - int(now.Weekday()) + 7) % 7
	if n == 0 && !inclusive {
		n = 7
	}
	return now.AddDate(0, 0, n)
}

var numDateRe = regexp.MustCompile(`^(\d{1,2})[./](\d{1,2})(?:[./](\d{2}|\d{4}))?\.?$`)
var nameDateRe = regexp.MustCompile(`^(\d{1,2})(?:er)? ([a-z]+)\.?(?: (\d{4}))?$`)

// calendarDate builds a date; without a year it takes the next occurrence
// from today on.
func calendarDate(now time.Time, d int, mo time.Month, year int, hasYear bool) (time.Time, bool) {
	y := now.Year()
	if hasYear {
		y = year
	}
	t := time.Date(y, mo, d, 0, 0, 0, 0, now.Location())
	if t.Day() != d || t.Month() != mo {
		return time.Time{}, false
	}
	if !hasYear {
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		if t.Before(today) {
			t = time.Date(y+1, mo, d, 0, 0, 0, 0, now.Location())
			if t.Day() != d {
				return time.Time{}, false
			}
		}
	}
	return t, true
}

// datePart reads the date half of an input, without its time of day.
func datePart(f string, now time.Time) (time.Time, bool) {
	switch f {
	case "aujourd'hui", "aujourdhui", "auj", "today":
		return now, true
	case "demain", "tomorrow":
		return now.AddDate(0, 0, 1), true
	case "apres-demain", "apres demain":
		return now.AddDate(0, 0, 2), true
	case "hier", "yesterday":
		return now.AddDate(0, 0, -1), true
	case "semaine prochaine", "la semaine prochaine":
		return nextWeekday(now, time.Monday, false), true
	case "mois prochain", "le mois prochain":
		return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()), true
	case "fin du mois", "fin de mois":
		return time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()), true
	}
	words := strings.Fields(strings.ReplaceAll(f, ".", ""))
	if len(words) > 0 {
		this := false
		if words[0] == "ce" {
			this, words = true, words[1:]
		}
		if len(words) > 0 {
			if wd, ok := weekdayNames[words[0]]; ok {
				switch strings.Join(words[1:], " ") {
				case "":
					return nextWeekday(now, wd, this), true
				case "prochain":
					if !this {
						return nextWeekday(now, wd, false), true
					}
				case "en huit":
					if !this {
						return nextWeekday(now, wd, false).AddDate(0, 0, 7), true
					}
				}
			}
		}
	}
	if m := numDateRe.FindStringSubmatch(f); m != nil {
		d, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		year, hasYear := 0, m[3] != ""
		if hasYear {
			year, _ = strconv.Atoi(m[3])
			if len(m[3]) == 2 {
				year += 2000
			}
		}
		if mo < 1 || mo > 12 {
			return time.Time{}, false
		}
		return calendarDate(now, d, time.Month(mo), year, hasYear)
	}
	if m := nameDateRe.FindStringSubmatch(f); m != nil {
		mo, ok := monthNames[m[2]]
		if !ok {
			return time.Time{}, false
		}
		d, _ := strconv.Atoi(m[1])
		year, hasYear := 0, m[3] != ""
		if hasYear {
			year, _ = strconv.Atoi(m[3])
		}
		return calendarDate(now, d, mo, year, hasYear)
	}
	return time.Time{}, false
}

var isoLayouts = []struct {
	layout string
	clock  bool
}{
	{"2006-01-02T15:04", true},
	{"2006-01-02 15:04", true},
	{"2006-01-02", false},
}

// Parse reads a moment typed freely: ISO (2026-11-15, 2026-11-15 14:00),
// RFC 3339, Swiss dates (15.11.2026, 15.11, 15/11/26), named dates
// (12 novembre, 1er déc.), words (aujourd'hui, demain, après-demain, hier,
// vendredi, vendredi prochain, ce vendredi, lundi en huit, semaine prochaine,
// mois prochain, fin du mois), delays (+3j, 3d, 2w, dans 2 semaines,
// dans 1 mois, +2h, dans 1h30) and a time alone (14h30: today, or tomorrow
// once passed). A date may end with a time: « demain 14h », « vendredi à 9h30 ».
func Parse(s string, now time.Time) (Moment, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Moment{}, ErrEmpty
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return dateClock(t.In(now.Location())), nil
	}
	for _, l := range isoLayouts {
		if t, err := time.ParseInLocation(l.layout, raw, now.Location()); err == nil {
			if l.clock {
				return dateClock(t), nil
			}
			return dateOnly(t), nil
		}
	}
	f := fold(raw)
	if c, err := ParseClock(strings.TrimPrefix(f, "a ")); err == nil {
		t, _ := time.ParseInLocation("2006-01-02 15:04", now.Format("2006-01-02")+" "+c, now.Location())
		if t.Before(now) {
			t = t.AddDate(0, 0, 1)
		}
		return dateClock(t), nil
	}
	if m, ok := delay(f, now); ok {
		return m, nil
	}
	datePhrase, clock := f, ""
	if words := strings.Fields(f); len(words) > 1 {
		if c, err := ParseClock(words[len(words)-1]); err == nil {
			clock = c
			words = words[:len(words)-1]
			if n := len(words); words[n-1] == "a" {
				words = words[:n-1]
			}
			datePhrase = strings.Join(words, " ")
		}
	}
	if isoT, err := time.ParseInLocation("2006-01-02", datePhrase, now.Location()); err == nil && clock != "" {
		return Moment{Date: isoT.Format("2006-01-02"), Clock: clock}, nil
	}
	t, ok := datePart(datePhrase, now)
	if !ok {
		return Moment{}, fmt.Errorf("date %q illisible : essayez demain, vendredi, 12.11, 12 novembre, +3j ou 2026-11-12", raw)
	}
	if clock != "" {
		return Moment{Date: t.Format("2006-01-02"), Clock: clock}, nil
	}
	return dateOnly(t), nil
}

// Horizon reads the end of a window: a delay from now (30d) or a date; a day
// ends at 23:59:59.
func Horizon(s string, now time.Time, defaultClock string) (time.Time, error) {
	if d, err := ParseDuration(strings.TrimPrefix(strings.TrimSpace(s), "+")); err == nil {
		if d%day == 0 {
			y, mo, da := now.AddDate(0, 0, int(d/day)).Date()
			return time.Date(y, mo, da, 23, 59, 59, 0, now.Location()), nil
		}
		return now.Add(d), nil
	}
	m, err := Parse(s, now)
	if err != nil {
		return time.Time{}, err
	}
	if m.AllDay {
		t, _ := time.ParseInLocation("2006-01-02", m.Date, now.Location())
		return t.AddDate(0, 0, 1).Add(-time.Second), nil
	}
	return m.Time(now.Location(), defaultClock), nil
}

var weekdays = []string{"dim.", "lun.", "mar.", "mer.", "jeu.", "ven.", "sam."}

// Day renders a date as « mar. 06.10.2026 ».
func Day(t time.Time) string {
	return weekdays[t.Weekday()] + " " + t.Format("02.01.2006")
}

// Until says how far t is from now in calendar days, in French: « dans 7 jours »,
// « demain », « aujourd'hui », « hier », « il y a 3 jours ».
func Until(t, now time.Time) string {
	y1, m1, d1 := now.Date()
	y2, m2, d2 := t.Date()
	a := time.Date(y1, m1, d1, 0, 0, 0, 0, time.UTC)
	b := time.Date(y2, m2, d2, 0, 0, 0, 0, time.UTC)
	days := int(b.Sub(a).Hours() / 24)
	switch {
	case days == 0:
		return "aujourd'hui"
	case days == 1:
		return "demain"
	case days == -1:
		return "hier"
	case days > 1:
		return fmt.Sprintf("dans %d jours", days)
	default:
		return fmt.Sprintf("il y a %d jours", -days)
	}
}

// Describe renders a moment for a live preview: « ven. 16.10.2026 14:30 · dans
// 7 jours ». A date alone shows defaultClock in parentheses, or no time when
// defaultClock is empty.
func Describe(m Moment, now time.Time, defaultClock string) string {
	t := m.Time(now.Location(), defaultClock)
	s := Day(t)
	switch {
	case !m.AllDay:
		s += " " + m.Clock
	case defaultClock != "":
		s += " (" + defaultClock + ")"
	}
	return s + " · " + Until(t, now)
}
