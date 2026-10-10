// Package complete proposes references to tuikit's Ref and Refs fields from
// sources declared once: commands that print JSON (office ls, contact ls…)
// and fixed entries. Commands run in the background and their rows stay in a
// cache shared by the process, so typing never waits.
package complete

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/aclemen1/tuikit"
)

// Entry is a fixed proposal of a source.
type Entry struct {
	Value string `yaml:"value" json:"value"`
	Label string `yaml:"label,omitempty" json:"label,omitempty"`
}

// Source proposes references: the rows of a command printing JSON ({ok,
// result: [...]}, {ok, result: {items: [...]}} or a bare list), or fixed
// entries. Value and Label name fields of a row; Match names more fields
// searched (a list field is searched item by item); Prefix goes before the
// value inserted.
type Source struct {
	Run    []string `yaml:"run,omitempty" json:"run,omitempty"`
	Static []Entry  `yaml:"static,omitempty" json:"static,omitempty"`
	Prefix string   `yaml:"prefix,omitempty" json:"prefix,omitempty"`
	Value  string   `yaml:"value,omitempty" json:"value,omitempty"`
	Label  string   `yaml:"label,omitempty" json:"label,omitempty"`
	Match  []string `yaml:"match,omitempty" json:"match,omitempty"`
}

// TTL is how long the rows of a command stay fresh; older rows are still
// served while the command runs again.
var TTL = 5 * time.Minute

// Timeout bounds one run of a command.
var Timeout = 10 * time.Second

type cached struct {
	rows    []map[string]any
	at      time.Time
	err     error
	loading bool
	done    chan struct{}
}

var cache = struct {
	sync.Mutex
	m map[string]*cached
}{m: map[string]*cached{}}

func key(run []string) string { return strings.Join(run, "\x00") }

// rows returns what the cache holds for run, and starts a run in the
// background when it holds nothing fresh.
func rows(run []string) []map[string]any {
	cache.Lock()
	defer cache.Unlock()
	c := cache.m[key(run)]
	if c == nil {
		c = &cached{}
		cache.m[key(run)] = c
	}
	if !c.loading && (c.at.IsZero() || time.Since(c.at) > TTL) {
		c.loading = true
		c.done = make(chan struct{})
		go load(run, c)
	}
	return c.rows
}

func load(run []string, c *cached) {
	got, err := fetch(run)
	cache.Lock()
	if err == nil {
		c.rows = got
	}
	c.err, c.at, c.loading = err, time.Now(), false
	close(c.done)
	cache.Unlock()
}

func fetch(run []string) ([]map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, run[0], run[1:]...).Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", strings.Join(run, " "), err)
	}
	return decode(out)
}

func decode(out []byte) ([]map[string]any, error) {
	var env struct {
		OK     *bool           `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	raw := json.RawMessage(out)
	if json.Unmarshal(out, &env) == nil && env.OK != nil {
		raw = env.Result
	}
	var list []map[string]any
	if json.Unmarshal(raw, &list) == nil {
		return list, nil
	}
	var wrapped struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("JSON illisible : %w", err)
	}
	return wrapped.Items, nil
}

// Completer searches its sources; its Complete method is a tuikit.Completer.
type Completer struct {
	sources []Source
}

// New starts the commands of the sources at once, so that rows are there by
// the first key.
func New(sources ...Source) *Completer {
	for _, s := range sources {
		if len(s.Run) > 0 {
			rows(s.Run)
		}
	}
	return &Completer{sources: sources}
}

// Wait blocks until every command has answered once, or d has passed; it
// tells whether all answered. A TUI does not need it.
func (c *Completer) Wait(d time.Duration) bool {
	deadline := time.After(d)
	for _, s := range c.sources {
		if len(s.Run) == 0 {
			continue
		}
		rows(s.Run)
		cache.Lock()
		e := cache.m[key(s.Run)]
		done, loading := e.done, e.loading
		cache.Unlock()
		if !loading {
			continue
		}
		select {
		case <-done:
		case <-deadline:
			return false
		}
	}
	return true
}

// Err is the last error of a command of the sources, nil when all answered.
func (c *Completer) Err() error {
	cache.Lock()
	defer cache.Unlock()
	var errs []error
	for _, s := range c.sources {
		if e := cache.m[key(s.Run)]; len(s.Run) > 0 && e != nil && e.err != nil {
			errs = append(errs, e.err)
		}
	}
	return errors.Join(errs...)
}

var foldReplacer = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i", "ô", "o", "ö", "o", "ù", "u", "û", "u", "ü", "u", "ç", "c",
)

func fold(s string) string { return foldReplacer.Replace(strings.ToLower(s)) }

// strs reads a field as strings: a scalar gives one, a list its items.
func strs(v any) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return []string{x}
	case []any:
		var out []string
		for _, e := range x {
			out = append(out, strs(e)...)
		}
		return out
	case float64:
		return []string{fmt.Sprint(x)}
	default:
		return []string{fmt.Sprint(x)}
	}
}

type candidate struct {
	item   tuikit.Item
	keys   string   // folded value, label and match fields, for the search
	starts []string // folded value and match fields, for the ranking
}

func (s Source) candidates() []candidate {
	var out []candidate
	add := func(value, label string, extra []string) {
		if value == "" {
			return
		}
		starts := []string{fold(value)}
		for _, e := range extra {
			starts = append(starts, fold(e))
		}
		out = append(out, candidate{
			item:   tuikit.Item{Value: s.Prefix + value, Label: strings.Join(strings.Fields(label), " ")},
			keys:   fold(value + " " + label + " " + strings.Join(extra, " ")),
			starts: starts,
		})
	}
	for _, e := range s.Static {
		add(e.Value, e.Label, nil)
	}
	if len(s.Run) == 0 || s.Value == "" {
		return out
	}
	for _, r := range rows(s.Run) {
		v := strs(r[s.Value])
		if len(v) == 0 {
			continue
		}
		label := ""
		if s.Label != "" {
			label = strings.Join(strs(r[s.Label]), ", ")
		}
		var extra []string
		for _, m := range s.Match {
			extra = append(extra, strs(r[m])...)
		}
		add(v[0], label, extra)
	}
	return out
}

// Complete proposes the items matching q: each word of q must appear in the
// value, the label or a Match field, whatever the case and accents. The
// prefix of a source may be typed (« office:P-00 »); a query naming another
// prefix leaves the source out. Exact values come first, then values or
// Match fields starting with q, then the rest.
func (c *Completer) Complete(q string) []tuikit.Item {
	q = strings.TrimSpace(q)
	type scored struct {
		item  tuikit.Item
		score int
		order int
	}
	var found []scored
	seen := map[string]bool{}
	for _, s := range c.sources {
		qq := q
		if s.Prefix != "" {
			if rest, ok := cutPrefixFold(q, s.Prefix); ok {
				qq = rest
			} else if strings.Contains(q, ":") {
				continue
			}
		}
		fq := fold(qq)
		words := strings.Fields(fq)
		for _, cand := range s.candidates() {
			if seen[cand.item.Value] || !containsAll(cand.keys, words) {
				continue
			}
			score := 2
			switch {
			case fq != "" && cand.starts[0] == fq:
				score = 0
			case fq != "" && slices.ContainsFunc(cand.starts, func(k string) bool { return strings.HasPrefix(k, fq) }):
				score = 1
			}
			seen[cand.item.Value] = true
			found = append(found, scored{cand.item, score, len(found)})
		}
	}
	slices.SortStableFunc(found, func(a, b scored) int { return a.score - b.score })
	out := make([]tuikit.Item, len(found))
	for i, f := range found {
		out[i] = f.item
	}
	return out
}

func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
		return s[len(prefix):], true
	}
	return s, false
}

func containsAll(keys string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(keys, w) {
			return false
		}
	}
	return true
}

// File is the shared file of named sources: $TUIKIT_REFS, else
// $XDG_CONFIG_HOME/tuikit/refs.yaml, else ~/.config/tuikit/refs.yaml.
func File() string {
	if p := os.Getenv("TUIKIT_REFS"); p != "" {
		return p
	}
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "tuikit", "refs.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "tuikit", "refs.yaml")
}

// Lookup reads the source name from the shared file; change its Prefix
// before New to insert another form (no prefix in a « to whom » field).
func Lookup(name string) (Source, error) {
	all, err := sharedSources()
	if err != nil {
		return Source{}, err
	}
	s, ok := all[name]
	if !ok {
		return Source{}, fmt.Errorf("%s : pas de source %q", File(), name)
	}
	return s, nil
}

// Use names a source of the shared file in a tool's configuration, with
// its own prefix when Prefix is set (an empty string inserts the bare value).
// In YAML or JSON it is a name, or {source: office, prefix: ""}.
type Use struct {
	Source string  `yaml:"source" json:"source"`
	Prefix *string `yaml:"prefix,omitempty" json:"prefix,omitempty"`
}

// Uses are the sources of one field: complete: {refs: [office, contact]}.
type Uses []Use

func (u *Use) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		u.Source = n.Value
		return nil
	}
	type plain Use
	return n.Decode((*plain)(u))
}

func (u *Use) UnmarshalJSON(b []byte) error {
	var name string
	if json.Unmarshal(b, &name) == nil {
		u.Source = name
		return nil
	}
	type plain Use
	return json.Unmarshal(b, (*plain)(u))
}

// sharedSources reads every source of the shared file.
func sharedSources() (map[string]Source, error) {
	data, err := os.ReadFile(File())
	if err != nil {
		return nil, err
	}
	var f struct {
		Sources map[string]Source `yaml:"sources"`
	}
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s : %w", File(), err)
	}
	return f.Sources, nil
}

// FromConfig builds a completer on the sources a tool's configuration names,
// and lists those missing from the shared file instead of failing: the TUI
// shows them and keeps the others. An unreadable file makes every source
// missing.
func FromConfig(uses Uses) (*Completer, []string) {
	all, _ := sharedSources()
	var sources []Source
	var missing []string
	for _, u := range uses {
		s, ok := all[u.Source]
		if !ok {
			missing = append(missing, u.Source)
			continue
		}
		if u.Prefix != nil {
			s.Prefix = *u.Prefix
		}
		sources = append(sources, s)
	}
	return New(sources...), missing
}

// LoadNames is FromConfig for names alone.
func LoadNames(names []string) (*Completer, []string) {
	uses := make(Uses, len(names))
	for i, n := range names {
		uses[i] = Use{Source: n}
	}
	return FromConfig(uses)
}

// Load builds a completer on sources named in the shared file; it fails on
// the first missing one, where FromConfig and LoadNames go on.
func Load(names ...string) (*Completer, error) {
	var sources []Source
	for _, n := range names {
		s, err := Lookup(n)
		if err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return New(sources...), nil
}
