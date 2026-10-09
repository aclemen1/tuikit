package when

import (
	"errors"
	"testing"
	"time"
)

func zurich(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// now is vendredi 9 octobre 2026, 17:37, Europe/Zurich.
func now(t *testing.T) time.Time {
	return time.Date(2026, 10, 9, 17, 37, 0, 0, zurich(t))
}

func TestParse(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2026-11-15", "2026-11-15"},
		{"2026-11-15 14:00", "2026-11-15T14:00"},
		{"2026-11-15T14:00", "2026-11-15T14:00"},
		{"2026-11-15 14h", "2026-11-15T14:00"},
		{"2026-11-15T13:00:00Z", "2026-11-15T14:00"},
		{"15.11.2026", "2026-11-15"},
		{"15.11.2026 14:00", "2026-11-15T14:00"},
		{"5.1.2027", "2027-01-05"},
		{"15/11/26", "2026-11-15"},
		{"12.11", "2026-11-12"},
		{"12.11.", "2026-11-12"},
		{"12.11 14:00", "2026-11-12T14:00"},
		{"09.10", "2026-10-09"},
		{"05.10", "2027-10-05"},
		{"12 novembre", "2026-11-12"},
		{"12 Nov.", "2026-11-12"},
		{"1er décembre", "2026-12-01"},
		{"1er dec 2027", "2027-12-01"},
		{"3 mars 2027 à 8h15", "2027-03-03T08:15"},
		{"aujourd'hui", "2026-10-09"},
		{"Aujourd’hui", "2026-10-09"},
		{"auj", "2026-10-09"},
		{"today", "2026-10-09"},
		{"demain", "2026-10-10"},
		{"tomorrow", "2026-10-10"},
		{"après-demain", "2026-10-11"},
		{"hier", "2026-10-08"},
		{"demain 14h", "2026-10-10T14:00"},
		{"demain à 9h30", "2026-10-10T09:30"},
		{"lundi", "2026-10-12"},
		{"Lundi", "2026-10-12"},
		{"lun.", "2026-10-12"},
		{"vendredi", "2026-10-16"},
		{"vendredi prochain", "2026-10-16"},
		{"ce vendredi", "2026-10-09"},
		{"ce lundi", "2026-10-12"},
		{"lundi en huit", "2026-10-19"},
		{"vendredi à 9h30", "2026-10-16T09:30"},
		{"mardi 10:00", "2026-10-13T10:00"},
		{"semaine prochaine", "2026-10-12"},
		{"mois prochain", "2026-11-01"},
		{"fin du mois", "2026-10-31"},
		{"+3j", "2026-10-12"},
		{"3j", "2026-10-12"},
		{"3d", "2026-10-12"},
		{"+ 3 jours", "2026-10-12"},
		{"dans 3 jours", "2026-10-12"},
		{"2w", "2026-10-23"},
		{"+2sem", "2026-10-23"},
		{"dans 2 semaines", "2026-10-23"},
		{"dans 1 mois", "2026-11-09"},
		{"+2 mois", "2026-12-09"},
		{"dans 1 an", "2027-10-09"},
		{"+2h", "2026-10-09T19:37"},
		{"dans 1h30", "2026-10-09T19:07"},
		{"30min", "2026-10-09T18:07"},
		{"+1j 2h", "2026-10-10T19:37"},
		{"18h", "2026-10-09T18:00"},
		{"à 18h", "2026-10-09T18:00"},
		{"14h30", "2026-10-10T14:30"},
		{"midi", "2026-10-10T12:00"},
	}
	for _, c := range cases {
		m, err := Parse(c.in, now(t))
		if err != nil {
			t.Errorf("Parse(%q): %v", c.in, err)
			continue
		}
		if m.String() != c.want {
			t.Errorf("Parse(%q) = %s, want %s", c.in, m, c.want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, in := range []string{"bientôt", "32.10", "12.13", "31 février", "vendredi dernier", "25h", "12h75", "ce vendredi en huit"} {
		if m, err := Parse(in, now(t)); err == nil {
			t.Errorf("Parse(%q) = %s, want an error", in, m)
		}
	}
	if _, err := Parse("  ", now(t)); !errors.Is(err, ErrEmpty) {
		t.Errorf("Parse(blank) = %v, want ErrEmpty", err)
	}
}

func TestParseClock(t *testing.T) {
	cases := map[string]string{"14h30": "14:30", "14h": "14:00", "14:30": "14:30", "9:05": "09:05", "9h05": "09:05", "14 h 30": "14:30", "midi": "12:00", "minuit": "00:00"}
	for in, want := range cases {
		got, err := ParseClock(in)
		if err != nil || got != want {
			t.Errorf("ParseClock(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"14", "24h", "14h60", "14h3", "x"} {
		if got, err := ParseClock(in); err == nil {
			t.Errorf("ParseClock(%q) = %q, want an error", in, got)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"30min":      30 * time.Minute,
		"45 min":     45 * time.Minute,
		"2h":         2 * time.Hour,
		"1h30":       90 * time.Minute,
		"1 h 30":     90 * time.Minute,
		"1h30min":    90 * time.Minute,
		"90min":      90 * time.Minute,
		"3j":         3 * day,
		"7d":         7 * day,
		"2w":         14 * day,
		"2 semaines": 14 * day,
		"1j 2h":      26 * time.Hour,
		"1 jour":     day,
		"10 minutes": 10 * time.Minute,
	}
	for in, want := range cases {
		got, err := ParseDuration(in)
		if err != nil || got != want {
			t.Errorf("ParseDuration(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"3", "2m", "1h30x", "jours", "-2d"} {
		if got, err := ParseDuration(in); err == nil {
			t.Errorf("ParseDuration(%q) = %v, want an error", in, got)
		}
	}
}

func TestParseDurations(t *testing.T) {
	got, err := ParseDurations("30d, 7d,1d")
	if err != nil || len(got) != 3 || got[0] != 30*day || got[1] != 7*day || got[2] != day {
		t.Fatalf("ParseDurations = %v, %v", got, err)
	}
	if _, err := ParseDurations("30d, x"); err == nil {
		t.Error("ParseDurations accepted x")
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{30 * day: "30d", 2 * time.Hour: "2h", 45 * time.Minute: "45min", 90 * time.Minute: "1h30", 26 * time.Hour: "26h"}
	for d, want := range cases {
		if got := FormatDuration(d); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", d, got, want)
		}
		back, err := ParseDuration(want)
		if err != nil || back != d {
			t.Errorf("ParseDuration(%q) = %v, %v; want %v", want, back, err, d)
		}
	}
}

func TestWords(t *testing.T) {
	cases := map[time.Duration]string{day: "1 jour", 3 * day: "3 jours", 14 * day: "2 semaines", 90 * time.Minute: "1 h 30", 2 * time.Hour: "2 h", 45 * time.Minute: "45 min"}
	for d, want := range cases {
		if got := Words(d); got != want {
			t.Errorf("Words(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestBeforeKeepsWallClockAcrossDST(t *testing.T) {
	term := time.Date(2026, 10, 30, 9, 0, 0, 0, zurich(t))
	got, err := Before(term, "7d")
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 23, 9, 0, 0, 0, zurich(t)); !got.Equal(want) {
		t.Errorf("Before = %v, want %v", got, want)
	}
}

func TestMomentTime(t *testing.T) {
	m := Moment{Date: "2027-03-31", AllDay: true}
	if got := m.Time(zurich(t), "09:00"); got.Hour() != 9 || got.Day() != 31 {
		t.Errorf("Time = %v", got)
	}
	if got := m.Time(zurich(t), ""); got.Hour() != 0 {
		t.Errorf("Time without clock = %v", got)
	}
}

func TestDescribe(t *testing.T) {
	cases := []struct{ in, clock, want string }{
		{"31.03.2027", "09:00", "mer. 31.03.2027 (09:00) · dans 173 jours"},
		{"31.03.2027", "", "mer. 31.03.2027 · dans 173 jours"},
		{"vendredi prochain 14h30", "09:00", "ven. 16.10.2026 14:30 · dans 7 jours"},
		{"demain", "09:00", "sam. 10.10.2026 (09:00) · demain"},
		{"hier", "", "jeu. 08.10.2026 · hier"},
	}
	for _, c := range cases {
		m, err := Parse(c.in, now(t))
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.in, err)
		}
		if got := Describe(m, now(t), c.clock); got != c.want {
			t.Errorf("Describe(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHorizon(t *testing.T) {
	got, err := Horizon("30d", now(t), "09:00")
	if err != nil || got.Format("2006-01-02 15:04:05") != "2026-11-08 23:59:59" {
		t.Errorf("Horizon(30d) = %v, %v", got, err)
	}
	got, err = Horizon("12.11", now(t), "09:00")
	if err != nil || got.Format("2006-01-02 15:04:05") != "2026-11-12 23:59:59" {
		t.Errorf("Horizon(12.11) = %v, %v", got, err)
	}
}
