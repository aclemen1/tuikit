package tuikit

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// clock is a settable time for a Busy.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newTestBusy() (*Busy, *clock) {
	c := &clock{t: time.Date(2026, 10, 10, 14, 0, 0, 0, time.UTC)}
	b := NewBusy()
	b.now = c.now
	return b, c
}

// split runs the commands of cmd, keeping the job messages and dropping the
// ticks without waiting for them.
func split(t *testing.T, cmd tea.Cmd) (done []tea.Msg, ticks int) {
	t.Helper()
	if cmd == nil {
		return nil, 0
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-ch:
	case <-time.After(50 * time.Millisecond):
		return nil, 1 // a tick sleeping
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			d, n := split(t, c)
			done = append(done, d...)
			ticks += n
		}
		return done, ticks
	}
	if _, ok := msg.(busyTickMsg); ok {
		return nil, 1
	}
	if msg == nil {
		return nil, 0
	}
	return []tea.Msg{msg}, 0
}

func TestBusyRunAndEnd(t *testing.T) {
	b, c := newTestBusy()
	if b.View() != "" {
		t.Fatal("idle registry shows something")
	}
	release := make(chan struct{})
	cmd1 := b.Run("confier PT-0012", func() (string, error) { <-release; return "confiée à P-0019", nil })
	b.Run("relecture task", func() (string, error) { return "", nil })
	if v := plain(b.View()); v != "⠋ 2 en cours · confier PT-0012" {
		t.Fatalf("view = %q", v)
	}
	close(release)
	done, ticks := split(t, cmd1)
	if len(done) != 1 || ticks != 1 {
		t.Fatalf("done %v, ticks %d", done, ticks)
	}
	b.Update(done[0])
	if v := plain(b.View()); v != "⠋ relecture task" {
		t.Fatalf("view = %q", v)
	}
	b.jobs[1].end = c.t
	if v := plain(b.View()); v != "✓ confiée à P-0019" {
		t.Fatalf("view = %q", v)
	}
	b.Update(busyTickMsg{b: b})
	c.t = c.t.Add(DoneFor + time.Second)
	if cmd, ok := b.Update(busyTickMsg{b: b}); !ok || cmd != nil || b.ticking {
		t.Fatal("tick kept going with nothing to show")
	}
	if b.View() != "" {
		t.Fatalf("view after the end = %q", plain(b.View()))
	}
}

func TestBusyFailureStaysUntilRead(t *testing.T) {
	b, c := newTestBusy()
	cmd := b.Run("confier PT-0012", func() (string, error) { return "", errors.New("dossier inconnu\ndétail") })
	done, _ := split(t, cmd)
	b.Update(done[0])
	c.t = c.t.Add(time.Hour)
	if v := plain(b.View()); v != "✗ confier PT-0012 : dossier inconnu" || b.Unread() != 1 {
		t.Fatalf("view = %q", v)
	}
	l := b.List()
	l.Resize(70, 10)
	if v := plain(l.View()); !strings.Contains(v, "✗ confier PT-0012") || !strings.Contains(v, "détail") {
		t.Fatalf("list = %q", v)
	}
	if b.Unread() != 0 || b.View() != "" {
		t.Fatal("opening the list did not mark the failure read")
	}
}

func TestBusyWrapForwardsMessage(t *testing.T) {
	type reloaded struct{ n int }
	b, _ := newTestBusy()
	cmd := b.Wrap("relecture task", func() tea.Msg { return reloaded{3} })
	done, _ := split(t, cmd)
	next, ok := b.Update(done[0])
	if !ok {
		t.Fatal("registry ignored its message")
	}
	fwd, _ := split(t, next)
	if len(fwd) != 1 || fwd[0].(reloaded).n != 3 {
		t.Fatalf("forwarded %v", fwd)
	}
	cmd = b.Wrap("relecture note", func() tea.Msg { return errors.New("note: exit 1") })
	done, _ = split(t, cmd)
	b.Update(done[0])
	if b.Unread() != 1 {
		t.Fatal("an error message is not a failure")
	}
}

func TestBusyIgnoresOtherMessages(t *testing.T) {
	b, _ := newTestBusy()
	other := NewBusy()
	if _, ok := b.Update(tea.KeyPressMsg{Code: 'x', Text: "x"}); ok {
		t.Fatal("took a key")
	}
	if _, ok := b.Update(busyTickMsg{b: other}); ok {
		t.Fatal("took the tick of another registry")
	}
}

func TestBusyListInModal(t *testing.T) {
	b, _ := newTestBusy()
	for i := range 60 {
		b.Run("travail", func() (string, error) { return "", nil })
		b.jobs[len(b.jobs)-1].end = b.jobs[len(b.jobs)-1].start.Add(time.Duration(i) * time.Millisecond)
	}
	if len(b.jobs) != busyKeep {
		t.Fatalf("%d jobs kept", len(b.jobs))
	}
	h := open(t, "Travaux", b.List())
	h.press("down", "down")
	h.typ("!")
	if !h.canceled() {
		t.Fatal("! did not close the list")
	}
}
