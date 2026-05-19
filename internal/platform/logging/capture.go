package logging

import (
	"context"
	"log/slog"
	"sync"
)

// CapturedRecord is a value snapshot of one logged event. Tests
// assert on this rather than on slog.Record so they don't have to
// worry about pcs/levels stability across Go versions.
type CapturedRecord struct {
	Level   slog.Level
	Message string
	Attrs   map[string]any
}

// recordStore is the shared, goroutine-safe sink behind a Capture
// handler. It is held by pointer so that handlers cloned by slog via
// WithAttrs / WithGroup keep writing into the SAME buffer — without
// this, `logger.With(...)` would silently drop records into an
// orphaned copy (the bug this design exists to prevent).
type recordStore struct {
	mu      sync.Mutex
	records []CapturedRecord
}

func (s *recordStore) append(r CapturedRecord) {
	s.mu.Lock()
	s.records = append(s.records, r)
	s.mu.Unlock()
}

func (s *recordStore) snapshot() []CapturedRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CapturedRecord, len(s.records))
	copy(out, s.records)
	return out
}

func (s *recordStore) reset() {
	s.mu.Lock()
	s.records = nil
	s.mu.Unlock()
}

// Capture is a slog.Handler that stores every record in memory for
// inspection by tests. It is goroutine-safe and clone-safe.
//
//	cap := logging.NewCapture(slog.LevelDebug)
//	ctx := logging.WithLogger(context.Background(), cap.Logger())
//	// ... exercise code ...
//	cap.Records()  // assert on this
type Capture struct {
	level  slog.Level
	store  *recordStore
	attrs  []slog.Attr
	groups []string
}

// NewCapture returns a Handler that retains every record at >= level.
func NewCapture(level slog.Level) *Capture {
	return &Capture{level: level, store: &recordStore{}}
}

// Logger returns a slog.Logger wired to this capture handler.
func (c *Capture) Logger() *slog.Logger { return slog.New(c) }

// Records returns a copy of the captured records so callers can
// iterate without racing the next emission.
func (c *Capture) Records() []CapturedRecord { return c.store.snapshot() }

// Reset drops every captured record.
func (c *Capture) Reset() { c.store.reset() }

// FindByMessage returns the first record whose message equals `msg`.
func (c *Capture) FindByMessage(msg string) (CapturedRecord, bool) {
	for _, r := range c.Records() {
		if r.Message == msg {
			return r, true
		}
	}
	return CapturedRecord{}, false
}

// FindAllByMessage returns every record matching `msg`.
func (c *Capture) FindAllByMessage(msg string) []CapturedRecord {
	out := []CapturedRecord{}
	for _, r := range c.Records() {
		if r.Message == msg {
			out = append(out, r)
		}
	}
	return out
}

// ─── slog.Handler implementation ─────────────────────────────────────

func (c *Capture) Enabled(_ context.Context, level slog.Level) bool {
	return level >= c.level
}

func (c *Capture) Handle(_ context.Context, r slog.Record) error {
	attrs := map[string]any{}
	prefix := ""
	for _, g := range c.groups {
		if prefix == "" {
			prefix = g
		} else {
			prefix += "." + g
		}
	}
	for _, a := range c.attrs {
		flatten(attrs, prefix, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		flatten(attrs, prefix, a)
		return true
	})
	c.store.append(CapturedRecord{
		Level:   r.Level,
		Message: r.Message,
		Attrs:   attrs,
	})
	return nil
}

func (c *Capture) WithAttrs(as []slog.Attr) slog.Handler {
	clone := c.clone()
	clone.attrs = append(clone.attrs, as...)
	return clone
}

func (c *Capture) WithGroup(name string) slog.Handler {
	if name == "" {
		return c
	}
	clone := c.clone()
	clone.groups = append(clone.groups, name)
	return clone
}

// clone shares the record store (so every derived handler writes to
// the same buffer) but copies the attr/group context.
func (c *Capture) clone() *Capture {
	return &Capture{
		level:  c.level,
		store:  c.store,
		attrs:  append([]slog.Attr{}, c.attrs...),
		groups: append([]string{}, c.groups...),
	}
}

func flatten(out map[string]any, prefix string, a slog.Attr) {
	key := a.Key
	if prefix != "" {
		key = prefix + "." + key
	}
	if a.Value.Kind() == slog.KindGroup {
		for _, sub := range a.Value.Group() {
			flatten(out, key, sub)
		}
		return
	}
	out[key] = a.Value.Any()
}
