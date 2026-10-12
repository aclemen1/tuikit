package tuikit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
)

// FocusDelay is how long a selection must stay before it is written.
var FocusDelay = 300 * time.Millisecond

// Focus writes the selection of a TUI to the shared focus file, read by the
// other tools: only the last selection after FocusDelay, only when its ref
// changed, atomically, and silently on failure.
type Focus struct {
	tool    string
	gen     int
	pending focusRecord
	written string
	now     func() time.Time
}

type focusRecord struct {
	Ref    string `json:"ref"`
	Title  string `json:"title"`
	Tool   string `json:"tool"`
	Sphere string `json:"sphere"`
	At     string `json:"at"`
}

type focusTickMsg struct {
	f   *Focus
	gen int
}

// FocusFile is the shared focus file: $OFFICE_FOCUS, else
// ~/.local/state/office/focus.json.
func FocusFile() string {
	if p := os.Getenv("OFFICE_FOCUS"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "office", "focus.json")
}

// NewFocus is the focus of the tool named tool (task, office…).
func NewFocus(tool string) *Focus { return &Focus{tool: tool, now: time.Now} }

// Set records the selection at each change: ref is <outil>:<id>. It returns
// the delay of the debounce, or nil when the ref is already the one written
// and nothing waits.
func (f *Focus) Set(ref, title, sphere string) tea.Cmd {
	if ref == "" || (ref == f.written && f.pending.Ref == "") {
		f.pending = focusRecord{}
		return nil
	}
	f.gen++
	f.pending = focusRecord{Ref: ref, Title: title, Tool: f.tool, Sphere: sphere}
	gen := f.gen
	return tea.Tick(FocusDelay, func(time.Time) tea.Msg { return focusTickMsg{f: f, gen: gen} })
}

// Update takes the debounce ticks of this focus; ok is false for any other
// message.
func (f *Focus) Update(msg tea.Msg) (tea.Cmd, bool) {
	t, ok := msg.(focusTickMsg)
	if !ok || t.f != f {
		return nil, false
	}
	if t.gen != f.gen || f.pending.Ref == "" {
		return nil, true
	}
	rec := f.pending
	f.pending = focusRecord{}
	if rec.Ref == f.written {
		return nil, true
	}
	f.written = rec.Ref
	rec.At = f.now().Format(time.RFC3339)
	return func() tea.Msg { writeFocus(rec); return nil }, true
}

func writeFocus(rec focusRecord) {
	path := FocusFile()
	data, err := json.Marshal(rec)
	if err != nil {
		return
	}
	dir := filepath.Dir(path)
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".focus-*.json")
	if err != nil {
		return
	}
	_, werr := tmp.Write(append(data, '\n'))
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), path) != nil {
		os.Remove(tmp.Name())
	}
}
