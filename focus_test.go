package tuikit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func focusAt(t *testing.T) (*Focus, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state", "office", "focus.json")
	t.Setenv("OFFICE_FOCUS", path)
	f := NewFocus("task")
	f.now = func() time.Time { return time.Date(2026, 10, 12, 10, 0, 0, 0, time.FixedZone("CEST", 7200)) }
	return f, path
}

// fire delivers the tick a Set would deliver after FocusDelay, and runs the
// write it asks for.
func fire(t *testing.T, f *Focus, gen int) {
	t.Helper()
	cmd, ok := f.Update(focusTickMsg{f: f, gen: gen})
	if !ok {
		t.Fatal("focus ignored its tick")
	}
	if cmd != nil {
		cmd()
	}
}

func readFocus(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestFocusDebounceKeepsLast(t *testing.T) {
	f, path := focusAt(t)
	for _, ref := range []string{"task:PT-0001", "task:PT-0002", "task:PT-0003"} {
		if f.Set(ref, "titre "+ref, "perso") == nil {
			t.Fatal("Set gave no delay")
		}
	}
	fire(t, f, 1)
	fire(t, f, 2)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("a stale selection was written")
	}
	fire(t, f, 3)
	got := readFocus(t, path)
	want := map[string]string{"ref": "task:PT-0003", "title": "titre task:PT-0003", "tool": "task", "sphere": "perso", "at": "2026-10-12T10:00:00+02:00"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temporary file left: %v", entries)
	}
}

func TestFocusSameRefNoWrite(t *testing.T) {
	f, path := focusAt(t)
	f.Set("task:PT-0001", "a", "perso")
	fire(t, f, 1)
	os.Remove(path)
	if f.Set("task:PT-0001", "a renamed", "perso") != nil {
		t.Fatal("same ref started a tick")
	}
	f.Set("task:PT-0002", "b", "perso")
	f.Set("task:PT-0001", "a", "perso")
	fire(t, f, 3)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("back to the written ref wrote again")
	}
	if f.Set("", "", "") != nil {
		t.Fatal("empty ref started a tick")
	}
}

func TestFocusFailureIsSilent(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	os.WriteFile(blocker, nil, 0o644)
	t.Setenv("OFFICE_FOCUS", filepath.Join(blocker, "focus.json"))
	f := NewFocus("task")
	f.Set("task:PT-0001", "a", "perso")
	fire(t, f, 1)
}

func TestFocusIgnoresOthers(t *testing.T) {
	f := NewFocus("task")
	if _, ok := f.Update(focusTickMsg{f: NewFocus("note"), gen: 1}); ok {
		t.Fatal("took the tick of another focus")
	}
	if _, ok := f.Update(tea.KeyPressMsg{Code: 'x'}); ok {
		t.Fatal("took a key")
	}
}

func TestFocusFileDefault(t *testing.T) {
	t.Setenv("OFFICE_FOCUS", "")
	home, _ := os.UserHomeDir()
	if FocusFile() != filepath.Join(home, ".local", "state", "office", "focus.json") {
		t.Fatalf("FocusFile = %s", FocusFile())
	}
}

func TestFocusTickWaitsDelay(t *testing.T) {
	old := FocusDelay
	FocusDelay = 20 * time.Millisecond
	defer func() { FocusDelay = old }()
	f, _ := focusAt(t)
	start := time.Now()
	msg := f.Set("task:PT-0001", "a", "perso")()
	if time.Since(start) < 20*time.Millisecond {
		t.Fatal("tick came before the delay")
	}
	if tm, ok := msg.(focusTickMsg); !ok || tm.gen != 1 {
		t.Fatalf("msg = %#v", msg)
	}
}
