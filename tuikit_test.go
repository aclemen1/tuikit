package tuikit

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var named = map[string]tea.KeyPressMsg{
	"tab":        {Code: tea.KeyTab},
	"shift+tab":  {Code: tea.KeyTab, Mod: tea.ModShift},
	"enter":      {Code: tea.KeyEnter},
	"esc":        {Code: tea.KeyEscape},
	"ctrl+s":     {Code: 's', Mod: tea.ModCtrl},
	"ctrl+c":     {Code: 'c', Mod: tea.ModCtrl},
	"up":         {Code: tea.KeyUp},
	"down":       {Code: tea.KeyDown},
	"left":       {Code: tea.KeyLeft},
	"right":      {Code: tea.KeyRight},
	"space":      {Code: tea.KeySpace, Text: " "},
	"ctrl+right": {Code: tea.KeyRight, Mod: tea.ModCtrl},
	"ctrl+down":  {Code: tea.KeyDown, Mod: tea.ModCtrl},
	"pgdown":     {Code: tea.KeyPgDown},
	"backspace":  {Code: tea.KeyBackspace},
}

func key(t *testing.T, name string) tea.KeyPressMsg {
	t.Helper()
	k, ok := named[name]
	if !ok {
		t.Fatalf("no key %q", name)
	}
	if got := k.Keystroke(); got != name {
		t.Fatalf("key %q reads %q", name, got)
	}
	return k
}

// msgs runs a command and every command it batches.
func msgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if b, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range b {
			out = append(out, msgs(c)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

type harness struct {
	t    *testing.T
	m    *Modal
	got  []tea.Msg
	keys []string
}

func open(t *testing.T, title string, c Content) *harness {
	return &harness{t: t, m: NewModal(title, c).SetSize(100, 40)}
}

func (h *harness) send(msg tea.Msg) {
	h.got = append(h.got, msgs(h.m.Update(msg))...)
}

func (h *harness) press(names ...string) {
	for _, n := range names {
		h.send(key(h.t, n))
	}
}

func (h *harness) typ(s string) {
	for _, r := range s {
		h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func (h *harness) done() DoneMsg {
	h.t.Helper()
	for _, m := range h.got {
		if d, ok := m.(DoneMsg); ok {
			return d
		}
	}
	h.t.Fatalf("no DoneMsg in %v", h.got)
	return DoneMsg{}
}

func (h *harness) canceled() bool {
	for _, m := range h.got {
		if _, ok := m.(CancelMsg); ok {
			return true
		}
	}
	return false
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;:?]*[a-zA-Z]`)

func plain(s string) string { return ansiRe.ReplaceAllString(s, "") }

func (h *harness) view() string { return plain(h.m.View()) }

func fixedNow() time.Time {
	loc, _ := time.LoadLocation("Europe/Zurich")
	return time.Date(2026, 10, 9, 17, 37, 0, 0, loc)
}

func TestEditorKeepsLongPaste(t *testing.T) {
	var b strings.Builder
	for i := range 500 {
		fmt.Fprintf(&b, "ligne %d avec des accents éàü et un texte assez long pour passer à la ligne dans la zone\r\n", i)
	}
	h := open(t, "Clôture", NewEditor("result", "Résultat", ""))
	h.send(tea.PasteMsg{Content: b.String()})
	h.press("ctrl+s")
	want := strings.ReplaceAll(b.String(), "\r\n", "\n")
	if got := h.done().Values.String("result"); got != want {
		t.Fatalf("paste lost text: %d bytes, want %d", len(got), len(want))
	}
}

func TestEditorEnterInsertsLine(t *testing.T) {
	h := open(t, "Clôture", NewEditor("result", "", "début"))
	h.press("enter")
	h.typ("suite")
	if len(h.got) != 0 || !h.m.Open() {
		t.Fatalf("enter closed the editor: %v", h.got)
	}
	h.press("ctrl+s")
	if got := h.done().Values.String("result"); got != "début\nsuite" {
		t.Fatalf("text = %q", got)
	}
}

func TestEditorShowsScrollBar(t *testing.T) {
	h := open(t, "Note", NewEditor("n", "", strings.Repeat("ligne\n", 200)))
	h.m.SetSize(60, 20)
	if v := h.view(); !strings.Contains(v, "┃") {
		t.Fatalf("no scroll bar:\n%s", v)
	}
}

func TestEscWithoutChangeCancels(t *testing.T) {
	h := open(t, "Clôture", NewEditor("result", "", "x"))
	h.press("esc")
	if !h.canceled() || h.m.Open() {
		t.Fatal("esc did not cancel")
	}
}

func TestEscAfterChangeAsks(t *testing.T) {
	h := open(t, "Clôture", NewEditor("result", "", ""))
	h.typ("abc")
	h.press("esc")
	if h.canceled() || !strings.Contains(h.view(), "Abandonner la saisie ?") {
		t.Fatalf("esc did not ask:\n%s", h.view())
	}
	h.press("esc")
	h.typ("d")
	if h.canceled() || h.m.Content().(*Editor).Text() != "abcd" {
		t.Fatal("esc in the question did not go back to the text")
	}
	h.press("esc")
	h.press("enter")
	if h.canceled() {
		t.Fatal("Non is not the first answer")
	}
	h.press("esc")
	h.typ("o")
	h.press("enter")
	if !h.canceled() || h.m.Open() {
		t.Fatal("Oui did not drop the input")
	}
}

func TestModalTakesEveryKey(t *testing.T) {
	h := open(t, "Titre", NewForm("f", Text("t", "Titre")))
	h.typ("q?/xyz")
	h.press("ctrl+c")
	for _, m := range h.got {
		if _, ok := m.(tea.QuitMsg); ok {
			t.Fatal("a key quit the TUI")
		}
	}
	if !strings.Contains(h.view(), "Abandonner la saisie ?") {
		t.Fatal("ctrl+c with a change did not ask")
	}
}

func dueForm(refs []Item) *Form {
	complete := func(q string) []Item {
		var out []Item
		for _, it := range refs {
			if strings.Contains(strings.ToLower(it.Value+" "+it.Label), strings.ToLower(q)) {
				out = append(out, it)
			}
		}
		return out
	}
	return NewForm("due",
		Choice("sphere", "Sphère", "perso", "pro").Required(),
		Text("title", "Titre").Required(),
		Date("due", "Date").Clock("09:00").Required(),
		Durations("notice", "Préavis").Default("30d,7d,1d").From("due"),
		Choice("term", "Au terme", "aucune", "message", "agent", "commande"),
		TextArea("cmd", "Commande").ShowIf("term", "commande"),
		Choice("channel", "Canal", "auto", "mail", "tell", "push"),
		Ref("ref", "Réf.", complete),
		TextArea("msg", "Message"),
	).Now(fixedNow)
}

var refs = []Item{{"P-0019", "Développement d'office"}, {"P-0040", "Outil d'échéances"}, {"PT-0007", "Relancer la gérance"}}

func TestFormRequiredChoiceHasNoPreselection(t *testing.T) {
	f := dueForm(refs)
	h := open(t, "Nouvelle échéance", f)
	if v := h.view(); !strings.Contains(v, "( ) perso   ( ) pro") {
		t.Fatalf("sphere preselected:\n%s", v)
	}
	h.press("ctrl+s")
	if !h.m.Open() || len(h.got) != 0 {
		t.Fatal("validated without the required fields")
	}
	v := h.view()
	if strings.Count(v, "! obligatoire") != 3 {
		t.Fatalf("errors not under each field:\n%s", v)
	}
	if f.current().id != "sphere" {
		t.Fatalf("focus on %s, want sphere", f.current().id)
	}
	h.press("right")
	if strings.Count(h.view(), "! obligatoire") != 2 {
		t.Fatal("the error did not clear once fixed")
	}
}

func TestFormFullEntry(t *testing.T) {
	f := dueForm(refs)
	h := open(t, "Nouvelle échéance", f)
	h.press("right", "right", "tab")
	h.typ("Résilier le bail")
	h.press("enter")
	h.typ("31.03.2027")
	if v := h.view(); !strings.Contains(v, "→ mer. 31.03.2027 (09:00) · dans 173 jours") {
		t.Fatalf("no live reading:\n%s", v)
	}
	h.press("tab")
	if v := h.view(); !strings.Contains(v, "30d lun. 01.03.2027 · 7d mer. 24.03.2027 · 1d mar. 30.03.2027") {
		t.Fatalf("no notice preview:\n%s", v)
	}
	h.press("tab")
	if strings.Contains(h.view(), "Commande") {
		t.Fatal("Commande shown while Au terme is aucune")
	}
	h.typ("c")
	if !strings.Contains(h.view(), "Commande") {
		t.Fatal("Commande hidden while Au terme is commande")
	}
	h.press("tab")
	h.typ("echo bonjour")
	h.press("enter")
	h.typ("echo fin")
	h.press("tab", "tab")
	h.typ("0019")
	h.press("enter")
	h.typ("Ligne 1")
	h.press("ctrl+s")
	v := h.done().Values
	checks := map[string]string{
		"sphere": "pro", "title": "Résilier le bail", "due": "2027-03-31", "notice": "30d,7d,1d",
		"term": "commande", "cmd": "echo bonjour\necho fin", "channel": "auto", "ref": "P-0019", "msg": "Ligne 1",
	}
	for id, want := range checks {
		if got := v.String(id); got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}
	if at := v.Time("due"); at.Hour() != 9 || at.Day() != 31 {
		t.Errorf("due time = %v", at)
	}
	if m, ok := v.Moment("due"); !ok || !m.AllDay {
		t.Errorf("due moment = %+v", m)
	}
	if d := v.Durations("notice"); len(d) != 3 || d[0] != 30*24*time.Hour {
		t.Errorf("notice = %v", d)
	}
}

func TestFormHiddenFieldHasNoValue(t *testing.T) {
	f := dueForm(refs)
	f.Field("cmd").area.ta.SetValue("rm -rf")
	h := open(t, "x", f)
	h.press("right", "tab")
	h.typ("t")
	h.press("tab")
	h.typ("demain")
	h.press("ctrl+s")
	if h.done().Values.Has("cmd") {
		t.Fatal("hidden Commande kept its value")
	}
}

func TestFormShiftTabAndArrows(t *testing.T) {
	f := NewForm("f", Text("a", "A"), Bool("b", "B"), Text("c", "C"))
	h := open(t, "x", f)
	h.press("down", "down", "down")
	if f.current().id != "c" {
		t.Fatalf("focus %s, want c", f.current().id)
	}
	h.press("shift+tab")
	h.press("space")
	h.press("up")
	if f.current().id != "a" || !f.Field("b").on {
		t.Fatal("shift+tab, space or up failed")
	}
	h.press("enter", "enter", "enter")
	if !h.m.Open() {
		t.Fatal("enter on the last field validated")
	}
}

func TestFormLiveErrorDoesNotBlock(t *testing.T) {
	f := NewForm("f", Date("d", "Date"), Clock("c", "Heure"), Duration("l", "Durée")).Now(fixedNow)
	h := open(t, "x", f)
	h.typ("bientôt")
	if !strings.Contains(h.view(), "illisible") {
		t.Fatalf("no error under the date:\n%s", h.view())
	}
	h.press("tab")
	h.typ("14h30")
	h.press("tab")
	h.typ("1h30")
	v := h.view()
	if !strings.Contains(v, "→ 14:30") || !strings.Contains(v, "→ 1 h 30") {
		t.Fatalf("no reading:\n%s", v)
	}
	h.press("ctrl+s")
	if !h.m.Open() || f.current().id != "d" {
		t.Fatal("an unreadable date validated")
	}
}

func TestFormValidate(t *testing.T) {
	f := NewForm("f", Text("a", "A")).Validate(func(v Values) Errors {
		if v.String("a") == "non" {
			return Errors{"a": "pas non"}
		}
		return nil
	})
	h := open(t, "x", f)
	h.typ("non")
	h.press("ctrl+s")
	if !strings.Contains(h.view(), "! pas non") {
		t.Fatalf("no custom error:\n%s", h.view())
	}
	h.press("backspace")
	if strings.Contains(h.view(), "! pas non") {
		t.Fatal("custom error stayed after a change")
	}
	h.press("ctrl+s")
	if h.done().Values.String("a") != "no" {
		t.Fatal("wrong value")
	}
}

func TestDateCalendar(t *testing.T) {
	f := NewForm("f", Date("d", "Date")).Now(fixedNow)
	h := open(t, "x", f)
	if v := h.view(); !strings.Contains(v, "octobre 2026") || !strings.Contains(v, "lu ma me je ve sa di") {
		t.Fatalf("no calendar:\n%s", v)
	}
	h.press("ctrl+right")
	if got := f.Field("d").in.Value(); got != "10.10.2026" {
		t.Fatalf("ctrl+right = %q", got)
	}
	h.press("ctrl+down", "pgdown")
	if got := f.Field("d").in.Value(); got != "17.11.2026" {
		t.Fatalf("ctrl+down, pgdown = %q", got)
	}
	if !strings.Contains(h.view(), "novembre 2026") {
		t.Fatal("calendar did not follow")
	}
}

func TestRefSuggestions(t *testing.T) {
	f := NewForm("f", Ref("r", "Réf.", func(q string) []Item {
		var out []Item
		for _, it := range refs {
			if strings.Contains(it.Value, strings.ToUpper(q)) {
				out = append(out, it)
			}
		}
		return out
	}).Strict(), Text("n", "Note"))
	h := open(t, "x", f)
	h.typ("p-00")
	v := h.view()
	if !strings.Contains(v, "› P-0019 · Développement d'office") || !strings.Contains(v, "P-0040") {
		t.Fatalf("no suggestions:\n%s", v)
	}
	h.press("down", "enter")
	if got := f.Field("r").in.Value(); got != "P-0040" || f.current().id != "n" {
		t.Fatalf("ref = %q, focus %s", got, f.current().id)
	}
	h.press("shift+tab")
	h.typ("x")
	h.press("tab", "ctrl+s")
	if !strings.Contains(h.view(), "n'est pas proposé") {
		t.Fatalf("strict ref accepted an unknown value:\n%s", h.view())
	}
}

func TestConfirm(t *testing.T) {
	h := open(t, "Abandonner", NewConfirm("drop", "Abandonner PE-0007 ?"))
	h.press("enter")
	if !h.canceled() {
		t.Fatal("Non did not cancel")
	}
	h = open(t, "Abandonner", NewConfirm("drop", "Abandonner PE-0007 ?"))
	h.typ("o")
	h.press("enter")
	if h.done().ID != "drop" {
		t.Fatal("Oui did not validate")
	}
	h = open(t, "Abandonner", NewConfirm("drop", "?").Default(true))
	h.press("left", "right")
	h.press("enter")
	h.done()
}

func TestConfirmTyped(t *testing.T) {
	h := open(t, "Supprimer", NewConfirmTyped("rm", "Supprimer le dossier P-0049 ?", "P-0049"))
	h.typ("P-004")
	h.press("enter")
	if !h.m.Open() || !strings.Contains(h.view(), "ne correspond pas") {
		t.Fatalf("wrong id accepted:\n%s", h.view())
	}
	h.typ("9")
	h.press("enter")
	if h.done().ID != "rm" {
		t.Fatal("right id refused")
	}
}

func TestOverlayCentres(t *testing.T) {
	base := strings.TrimSuffix(strings.Repeat(strings.Repeat("·", 100)+"\n", 30), "\n")
	m := NewModal("Nouveau dossier", NewForm("f", Text("t", "Titre")))
	out := Overlay(base, m, 100, 30)
	lines := strings.Split(plain(out), "\n")
	if len(lines) != 30 {
		t.Fatalf("%d lines, want 30", len(lines))
	}
	if lipgloss.Width(lines[0]) != 100 || !strings.HasPrefix(lines[0], "···") {
		t.Fatalf("first line changed: %q", lines[0])
	}
	if !strings.Contains(plain(out), "Nouveau dossier") {
		t.Fatal("no modal")
	}
	box := plain(m.View())
	bw := lipgloss.Width(strings.Split(box, "\n")[0])
	if bw != 90 {
		t.Fatalf("box width %d, want 90", bw)
	}
	m.open = false
	if Overlay(base, m, 100, 30) != base {
		t.Fatal("closed modal changed the view")
	}
}

func refsForm() *Form {
	return NewForm("note",
		Refs("refs", "Réfs", func(q string) []Item {
			var out []Item
			for _, it := range refs {
				if strings.Contains(strings.ToLower(it.Value), strings.ToLower(q)) {
					out = append(out, it)
				}
			}
			return out
		}).Required(),
		Text("t", "Titre"),
	)
}

func TestRefsAddRemove(t *testing.T) {
	f := refsForm()
	h := open(t, "Nouvelle note", f)
	h.press("ctrl+s")
	if !strings.Contains(h.view(), "! obligatoire") {
		t.Fatal("Refs required accepted no value")
	}
	h.typ("0019")
	h.press("enter")
	if f.current().id != "refs" || !slices.Equal(f.Field("refs").items, []string{"P-0019"}) {
		t.Fatalf("enter did not add: %v", f.Field("refs").items)
	}
	h.typ("pt-")
	h.press("down")
	h.typ(",")
	h.typ("office:P-0019,")
	if got := f.Field("refs").items; !slices.Equal(got, []string{"P-0019", "PT-0007", "office:P-0019"}) {
		t.Fatalf("items = %v", got)
	}
	h.press("backspace")
	if got := f.Field("refs").items; !slices.Equal(got, []string{"P-0019", "PT-0007"}) {
		t.Fatalf("backspace on empty entry: %v", got)
	}
	h.typ("0040")
	if v := h.view(); strings.Contains(v, "› P-0019") || !strings.Contains(v, "[P-0019]") {
		t.Fatalf("chosen value proposed again or not shown:\n%s", v)
	}
	h.press("enter", "enter")
	if f.current().id != "t" {
		t.Fatal("enter on an empty entry did not go on")
	}
	h.press("ctrl+s")
	if got := h.done().Values.Strings("refs"); !slices.Equal(got, []string{"P-0019", "PT-0007", "P-0040"}) {
		t.Fatalf("Strings = %v", got)
	}
}

func TestRefsDefaultAndPaste(t *testing.T) {
	f := NewForm("n", Refs("r", "Réfs", nil).Default([]string{"task:PT-0007", "office:P-0051"}))
	h := open(t, "x", f)
	h.send(tea.PasteMsg{Content: "due:PE-0001, office:P-0051\nnote:N-1, mnemo:"})
	if got := f.Field("r").items; !slices.Equal(got, []string{"task:PT-0007", "office:P-0051", "due:PE-0001", "note:N-1"}) {
		t.Fatalf("items = %v", got)
	}
	if f.Field("r").in.Value() != "mnemo:" {
		t.Fatalf("entry = %q", f.Field("r").in.Value())
	}
	h.press("ctrl+s")
	v := h.done().Values
	if got := v.Strings("r"); len(got) != 5 || got[4] != "mnemo:" {
		t.Fatalf("Strings = %v", got)
	}
	if v.String("r") != "task:PT-0007,office:P-0051,due:PE-0001,note:N-1,mnemo:" {
		t.Fatalf("String = %q", v.String("r"))
	}
}

func TestRequiredLabelKeepsGap(t *testing.T) {
	h := open(t, "x", NewForm("f", Text("t", "Titre").Required().Default("Rappel")))
	if !strings.Contains(h.view(), "Titre *  Rappel") {
		t.Fatalf("no gap after the label:\n%s", h.view())
	}
}

func TestEnglish(t *testing.T) {
	SetLanguage(English)
	defer SetLanguage(French)
	f := NewForm("f", Date("d", "Date").Clock("09:00").Required(), Bool("s", "Secret")).Now(fixedNow)
	h := open(t, "x", f)
	h.press("ctrl+s")
	v := h.view()
	for _, want := range []string{"! required", "October 2026", "mo tu we th fr sa su", "[ ] no", "tab next field · ctrl+s save · esc cancel"} {
		if !strings.Contains(v, want) {
			t.Errorf("no %q in:\n%s", want, v)
		}
	}
	h.typ("next friday")
	if !strings.Contains(h.view(), "→ Fri 16.10.2026 (09:00) · in 7 days") {
		t.Errorf("no English reading:\n%s", h.view())
	}
	h.press("esc")
	if v := h.view(); !strings.Contains(v, "Discard your input?") || !strings.Contains(v, "Yes") {
		t.Errorf("no English question:\n%s", v)
	}
	h.typ("y")
	h.press("enter")
	if !h.canceled() {
		t.Error("y did not answer Yes")
	}
}

func TestDateAllowNone(t *testing.T) {
	f := NewForm("w", Text("who", "Sur qui"), Date("until", "Échéance").AllowNone().Required()).Now(fixedNow)
	h := open(t, "Attente", f)
	h.press("tab")
	h.typ("Jamais")
	if !strings.Contains(h.view(), "→ aucune échéance") {
		t.Fatalf("no reading:\n%s", h.view())
	}
	h.press("ctrl+s")
	v := h.done().Values
	if !v.None("until") || v.String("until") != "none" {
		t.Fatalf("None = %v, String = %q", v.None("until"), v.String("until"))
	}
	if _, ok := v.Moment("until"); ok {
		t.Fatal("none has a moment")
	}
	g := NewForm("w", Date("d", "Date")).Now(fixedNow)
	h = open(t, "x", g)
	h.typ("aucune")
	h.press("ctrl+s")
	if !h.m.Open() {
		t.Fatal("aucune accepted without AllowNone")
	}
}

func pickList() *List {
	return NewList("pick", []Item{
		{Value: "new", Label: "+ nouveau"},
		{Value: "note:PN-0001", Label: "Appeler la gérance"},
		{Value: "task:PT-0007", Label: "Relancer le notaire"},
		{Value: "guard:PG-0002", Label: "Jamais signer sans relire"},
	})
}

func TestListPick(t *testing.T) {
	h := open(t, "Notes", pickList())
	if !strings.Contains(h.view(), "› + nouveau") {
		t.Fatalf("first item not selected:\n%s", h.view())
	}
	h.press("down")
	h.typ("j")
	h.typ("k")
	h.press("enter")
	if got := h.done().Values.String("pick"); got != "note:PN-0001" {
		t.Fatalf("picked %q", got)
	}
}

func TestListFilter(t *testing.T) {
	h := open(t, "Notes", pickList().Default("guard:PG-0002"))
	h.typ("/notai")
	v := h.view()
	if !strings.Contains(v, "› Relancer le notaire") || strings.Contains(v, "gérance") {
		t.Fatalf("filter failed:\n%s", v)
	}
	for range 6 {
		h.press("backspace")
	}
	if h.m.Content().(*List).filtering {
		t.Fatal("backspace on an empty filter did not close it")
	}
	h.typ("/zzz")
	h.press("enter")
	if !h.m.Open() {
		t.Fatal("enter picked in an empty list")
	}
	h.press("esc")
	if !h.canceled() {
		t.Fatal("esc did not cancel")
	}
}

func TestValuesSet(t *testing.T) {
	var v Values
	v.Set("x", "abc")
	if !v.Has("x") || v.String("x") != "abc" {
		t.Fatal("Set did not record")
	}
}
