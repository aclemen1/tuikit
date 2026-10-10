package complete

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aclemen1/tuikit"
)

const dossiersJSON = `{"ok":true,"result":[
{"id":"P-0013","alias":"MNEMO-CLI","title":"CLI et serveur MCP pour la mémoire"},
{"id":"P-0019","alias":"OFFICE-CLI","title":"Développement d'office"},
{"id":"P-0024","alias":"MAMAN","title":"Suivi de Jeannette"},
{"id":"P-0049","title":"tuikit : composants de saisie"}]}`

const contactsJSON = `[{"alias":"JMR","aliases":["JEAN-MARC"],"name":"Jean-Marc Rossier","org":"Gérance Rossier"},
{"alias":"AC","aliases":["ALAIN"],"name":"Alain Clément"}]`

const tasksJSON = `{"ok":true,"result":{"items":[{"id":"PT-0007","title":"Relancer le notaire"}]}}`

// echo is a source command printing s; each test gives its own s, so the
// shared cache never mixes them.
func echo(s string) []string { return []string{"printf", "%s", s} }

func values(items []tuikit.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Value
	}
	return out
}

func ready(t *testing.T, sources ...Source) *Completer {
	t.Helper()
	c := New(sources...)
	if !c.Wait(5 * time.Second) {
		t.Fatal("sources did not answer")
	}
	if err := c.Err(); err != nil {
		t.Fatal(err)
	}
	return c
}

var office = Source{Run: echo(dossiersJSON), Prefix: "office:", Value: "id", Label: "title", Match: []string{"alias", "title"}}
var contacts = Source{Run: echo(contactsJSON), Prefix: "contact:", Value: "alias", Label: "name", Match: []string{"aliases", "org"}}
var tasks = Source{Run: echo(tasksJSON), Prefix: "task:", Value: "id", Label: "title"}
var fixed = Source{Static: []Entry{{"desk", "le desk"}, {"new", "nouveau dossier"}}}

func TestSearchFields(t *testing.T) {
	c := ready(t, office, contacts, tasks, fixed)
	cases := []struct {
		q    string
		want []string
	}{
		{"P-0019", []string{"office:P-0019"}},
		{"0019", []string{"office:P-0019"}},
		{"maman", []string{"office:P-0024"}},
		{"jeannette", []string{"office:P-0024"}},
		{"memoire", []string{"office:P-0013"}},
		{"jean-marc", []string{"contact:JMR"}},
		{"gerance", []string{"contact:JMR"}},
		{"clément", []string{"contact:AC"}},
		{"notaire", []string{"task:PT-0007"}},
		{"desk", []string{"desk"}},
		{"cli office", []string{"office:P-0019"}},
		{"office:P-00", []string{"office:P-0013", "office:P-0019", "office:P-0024", "office:P-0049"}},
		{"OFFICE:p-0049", []string{"office:P-0049"}},
		{"contact:j", []string{"contact:JMR"}},
		{"zzz", []string{}},
	}
	for _, cs := range cases {
		if got := values(c.Complete(cs.q)); strings.Join(got, ",") != strings.Join(cs.want, ",") {
			t.Errorf("Complete(%q) = %v, want %v", cs.q, got, cs.want)
		}
	}
	if n := len(c.Complete("")); n != 9 {
		t.Errorf("Complete(\"\") gave %d items, want 9", n)
	}
	if got := c.Complete("0019")[0].Label; got != "Développement d'office" {
		t.Errorf("label = %q", got)
	}
}

func TestRanking(t *testing.T) {
	src := Source{Run: echo(`[{"id":"P-0100","title":"Office d'été"},{"id":"P-0001","title":"Bureau","alias":"OFFICE"},{"id":"OFFICE","title":"exact"}]`),
		Value: "id", Label: "title", Match: []string{"alias"}}
	c := ready(t, src)
	if got := values(c.Complete("office")); strings.Join(got, ",") != "OFFICE,P-0001,P-0100" {
		t.Fatalf("order = %v", got)
	}
}

func TestNoPrefixSource(t *testing.T) {
	s := office
	s.Prefix = ""
	c := ready(t, s)
	if got := values(c.Complete("maman")); len(got) != 1 || got[0] != "P-0024" {
		t.Fatalf("got %v", got)
	}
}

func TestNeverBlocks(t *testing.T) {
	slow := Source{Run: []string{"sh", "-c", "sleep 1; echo '[{\"id\":\"SLOW-1\"}]'"}, Value: "id"}
	c := New(slow)
	start := time.Now()
	if got := c.Complete("slow"); len(got) != 0 {
		t.Fatalf("rows before the command answered: %v", got)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("Complete waited for the command")
	}
	if !c.Wait(5 * time.Second) {
		t.Fatal("command did not answer")
	}
	if got := values(c.Complete("slow")); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}

func TestCacheAndTTL(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "n")
	run := []string{"sh", "-c", "echo x >> " + counter + "; echo '[{\"id\":\"C-1\"}]'"}
	ready(t, Source{Run: run, Value: "id"})
	ready(t, Source{Run: run, Value: "id"}).Complete("c")
	if n := runs(t, counter); n != 1 {
		t.Fatalf("command ran %d times, want 1 (shared cache)", n)
	}
	old := TTL
	TTL = 0
	defer func() { TTL = old }()
	c := New(Source{Run: run, Value: "id"})
	c.Wait(5 * time.Second)
	if got := values(c.Complete("c")); len(got) != 1 {
		t.Fatalf("stale rows not served: %v", got)
	}
	if n := runs(t, counter); n < 2 {
		t.Fatalf("command ran %d times, want a new run once stale", n)
	}
}

func runs(t *testing.T, path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "x")
}

func TestFailingCommand(t *testing.T) {
	c := New(Source{Run: []string{"false"}, Value: "id"}, fixed)
	c.Wait(5 * time.Second)
	if c.Err() == nil {
		t.Fatal("no error for a failing command")
	}
	if got := values(c.Complete("desk")); len(got) != 1 {
		t.Fatalf("static entries lost: %v", got)
	}
	c = New(Source{Run: echo("pas du json"), Value: "id"})
	c.Wait(5 * time.Second)
	if c.Err() == nil {
		t.Fatal("no error for unreadable JSON")
	}
}

func TestSharedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "refs.yaml")
	data := filepath.Join(dir, "dossiers.json")
	if err := os.WriteFile(data, []byte(dossiersJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := `sources:
  office:
    run: [cat, "` + data + `"]
    prefix: "office:"
    value: id
    label: title
    match: [alias, title]
  fixed:
    static:
      - {value: desk, label: le desk}
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TUIKIT_REFS", path)
	if File() != path {
		t.Fatalf("File = %s", File())
	}
	c, err := Load("office", "fixed")
	if err != nil {
		t.Fatal(err)
	}
	c.Wait(5 * time.Second)
	if got := values(c.Complete("maman")); len(got) != 1 || got[0] != "office:P-0024" {
		t.Fatalf("got %v", got)
	}
	s, err := Lookup("office")
	if err != nil || s.Prefix != "office:" || s.Match[0] != "alias" {
		t.Fatalf("Lookup = %+v, %v", s, err)
	}
	if _, err := Load("absent"); err == nil {
		t.Fatal("unknown source accepted")
	}
}

func TestDefaultFile(t *testing.T) {
	t.Setenv("TUIKIT_REFS", "")
	t.Setenv("XDG_CONFIG_HOME", "/x")
	if File() != "/x/tuikit/refs.yaml" {
		t.Fatalf("File = %s", File())
	}
}

func TestLabelOnOneLine(t *testing.T) {
	c := ready(t, Source{Run: echo(`[{"id":"PN-1","body":"ligne 1\n\n- ligne 2"}]`), Value: "id", Label: "body"})
	if got := c.Complete("PN-1")[0].Label; got != "ligne 1 - ligne 2" {
		t.Fatalf("label = %q", got)
	}
}
