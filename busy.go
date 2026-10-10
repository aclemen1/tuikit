package tuikit

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// BusyKey is the common key that opens the list of recent jobs.
const BusyKey = "!"

// DoneFor is how long the end of a job stays in the status line.
var DoneFor = 4 * time.Second

const (
	busyKeep  = 50
	busyFrame = 100 * time.Millisecond
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Busy registers the background jobs of a TUI and shows them in a status
// line: a spinner while one runs, the end for a few seconds, a failure in red
// until the list is opened. It animates only while it has something to show.
type Busy struct {
	jobs      []*busyJob // newest last
	next      int
	frame     int
	ticking   bool
	shown     *busyJob // last ended job, shown until shownTill
	shownTill time.Time
	now       func() time.Time
}

type busyJob struct {
	id         int
	label      string
	start, end time.Time
	text       string
	err        error
	read       bool
}

func (j *busyJob) running() bool { return j.end.IsZero() }

type busyDoneMsg struct {
	b     *Busy
	id    int
	text  string
	err   error
	inner tea.Msg
	wrap  bool
}

type busyTickMsg struct{ b *Busy }

// NewBusy makes an empty registry.
func NewBusy() *Busy { return &Busy{now: time.Now} }

func (b *Busy) add(label string) *busyJob {
	b.next++
	j := &busyJob{id: b.next, label: label, start: b.now()}
	b.jobs = append(b.jobs, j)
	if len(b.jobs) > busyKeep {
		b.jobs = b.jobs[len(b.jobs)-busyKeep:]
	}
	return j
}

// Run starts fn outside the bubbletea loop under label; fn returns the text
// shown at the end (label when empty) or an error.
func (b *Busy) Run(label string, fn func() (string, error)) tea.Cmd {
	j := b.add(label)
	job := func() tea.Msg {
		text, err := fn()
		return busyDoneMsg{b: b, id: j.id, text: text, err: err}
	}
	return tea.Batch(job, b.startTick())
}

// Wrap follows a tea.Cmd under label; its message goes on to the TUI once
// recorded, and counts as a failure when it is an error.
func (b *Busy) Wrap(label string, cmd tea.Cmd) tea.Cmd {
	j := b.add(label)
	job := func() tea.Msg {
		var msg tea.Msg
		if cmd != nil {
			msg = cmd()
		}
		err, _ := msg.(error)
		return busyDoneMsg{b: b, id: j.id, err: err, inner: msg, wrap: true}
	}
	return tea.Batch(job, b.startTick())
}

func (b *Busy) startTick() tea.Cmd {
	if b.ticking {
		return nil
	}
	b.ticking = true
	return b.tick(busyFrame)
}

func (b *Busy) tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return busyTickMsg{b: b} })
}

// Update takes the messages of the registry; ok is false for any other
// message, which the TUI handles as usual.
func (b *Busy) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case busyDoneMsg:
		if msg.b != b {
			return nil, false
		}
		for _, j := range b.jobs {
			if j.id == msg.id {
				j.end, j.text, j.err = b.now(), msg.text, msg.err
				if j.text == "" {
					j.text = j.label
				}
				if j.err == nil {
					b.shown, b.shownTill = j, j.end.Add(DoneFor)
				}
			}
		}
		var cmd tea.Cmd
		if msg.wrap && msg.inner != nil {
			inner := msg.inner
			cmd = func() tea.Msg { return inner }
		}
		return tea.Batch(cmd, b.startTick()), true
	case busyTickMsg:
		if msg.b != b {
			return nil, false
		}
		switch {
		case b.Running() > 0:
			b.frame = (b.frame + 1) % len(spinnerFrames)
			return b.tick(busyFrame), true
		case b.shown != nil && b.now().Before(b.shownTill):
			return b.tick(b.shownTill.Sub(b.now())), true
		}
		b.shown = nil
		b.ticking = false
		return nil, true
	}
	return nil, false
}

// Running is the number of jobs under way.
func (b *Busy) Running() int {
	n := 0
	for _, j := range b.jobs {
		if j.running() {
			n++
		}
	}
	return n
}

// Unread is the number of failures not yet seen in the list.
func (b *Busy) Unread() int {
	n := 0
	for _, j := range b.jobs {
		if j.err != nil && !j.read {
			n++
		}
	}
	return n
}

func firstLine(err error) string {
	s, _, _ := strings.Cut(err.Error(), "\n")
	return s
}

// View is the status line: « ⠋ 2 en cours · confier PT-0012 », « ✓ confiée à
// P-0019 », « ✗ confier PT-0012 : … » in red; empty when there is nothing.
func (b *Busy) View() string {
	var parts []string
	var oldest *busyJob
	n := 0
	for _, j := range b.jobs {
		if j.running() {
			n++
			if oldest == nil {
				oldest = j
			}
		}
	}
	switch {
	case n == 1:
		parts = append(parts, sFocus.Render(spinnerFrames[b.frame])+" "+oldest.label)
	case n > 1:
		parts = append(parts, sFocus.Render(spinnerFrames[b.frame])+" "+fmt.Sprintf(tr("%d en cours", "%d running"), n)+" · "+oldest.label)
	}
	var lastErr *busyJob
	unread := 0
	for _, j := range b.jobs {
		if j.err != nil && !j.read {
			lastErr = j
			unread++
		}
	}
	if lastErr != nil {
		s := "✗ " + lastErr.label + " : " + truncate(firstLine(lastErr.err), 60)
		if unread > 1 {
			s += fmt.Sprintf(" (+%d)", unread-1)
		}
		parts = append(parts, sError.Render(s))
	}
	if n == 0 && lastErr == nil && b.shown != nil && b.now().Before(b.shownTill) {
		parts = append(parts, sOK.Render("✓ "+b.shown.text))
	}
	return strings.Join(parts, "   ")
}

// List is the content of a modal listing the recent jobs, newest first, with
// their failures in full; opening it marks the failures as read.
func (b *Busy) List() Content {
	for _, j := range b.jobs {
		j.read = true
	}
	return &busyList{b: b}
}

type busyList struct {
	b             *Busy
	offset        int
	width, height int
}

func (l *busyList) ID() string    { return "busy" }
func (l *busyList) Changed() bool { return false }

func (l *busyList) Resize(width, height int) { l.width, l.height = width, max(height, 1) }

func (l *busyList) lines() []string {
	var out []string
	for i := len(l.b.jobs) - 1; i >= 0; i-- {
		j := l.b.jobs[i]
		at := j.start.Format("15:04:05")
		switch {
		case j.running():
			out = append(out, sFocus.Render(at+" ⠿ "+j.label)+sDim.Render(" · "+tr("en cours", "running")))
		case j.err != nil:
			out = append(out, sError.Render(at+" ✗ "+j.label)+sDim.Render(" · "+duration(j.end.Sub(j.start))))
			for line := range strings.SplitSeq(lipgloss.NewStyle().Width(max(l.width-11, 10)).Render(j.err.Error()), "\n") {
				out = append(out, "           "+sError.Render(line))
			}
		default:
			s := at + " ✓ " + j.label
			if j.text != j.label {
				s += " → " + j.text
			}
			out = append(out, truncate(s, l.width-8)+sDim.Render(" · "+duration(j.end.Sub(j.start))))
		}
	}
	if len(out) == 0 {
		out = append(out, sDim.Render(tr("aucun travail récent", "no recent job")))
	}
	return out
}

func duration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	return strings.Replace(fmt.Sprintf("%.1f s", d.Seconds()), ".", tr(",", "."), 1)
}

func (l *busyList) Update(msg tea.Msg) (Action, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return Continue, nil
	}
	n := len(l.lines())
	switch k.Keystroke() {
	case "up", "k":
		l.offset--
	case "down", "j":
		l.offset++
	case "pgup":
		l.offset -= l.height
	case "pgdown":
		l.offset += l.height
	case "enter", "q", BusyKey:
		return Cancel, nil
	}
	l.offset = min(max(l.offset, 0), max(n-l.height, 0))
	return Continue, nil
}

func (l *busyList) View() string {
	lines := l.lines()
	end := min(l.offset+l.height, len(lines))
	return strings.Join(lines[min(l.offset, end):end], "\n")
}

func (l *busyList) Submit() (Values, bool) { return Values{}, true }

func (l *busyList) Keys() string {
	return tr("↑↓ défiler · esc fermer", "↑↓ scroll · esc close")
}
