package ats_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/ats"
)

// stubPlugin is a minimal ats.Plugin implementation for registry tests.
type stubPlugin struct {
	name       string
	detectFunc func(url string) bool
}

func (s *stubPlugin) Name() string { return s.name }
func (s *stubPlugin) Detect(url string) bool {
	if s.detectFunc != nil {
		return s.detectFunc(url)
	}
	return strings.Contains(url, s.name)
}
func (s *stubPlugin) ValidateURL(url string) bool { return s.Detect(url) }
func (s *stubPlugin) Apply(_ context.Context, _ string, _ *ats.ApplicationData) (*ats.ApplicationResult, error) {
	return &ats.ApplicationResult{Success: true, ATSType: s.name}, nil
}

func newTestRegistry(t *testing.T) *ats.Registry {
	t.Helper()
	return ats.NewRegistry(zerolog.Nop())
}

// ── NewRegistry ───────────────────────────────────────────────────────────────

func TestNewRegistry_NotNil(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
}

func TestNewRegistry_EmptyPluginList(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)
	if names := r.ListPlugins(); len(names) != 0 {
		t.Errorf("expected 0 plugins, got %d: %v", len(names), names)
	}
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister_AddsPlugin(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)
	r.Register(&stubPlugin{name: "greenhouse"})
	names := r.ListPlugins()
	if len(names) != 1 || names[0] != "greenhouse" {
		t.Errorf("expected [greenhouse], got %v", names)
	}
}

func TestRegister_MultiplePlugins(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)
	for _, name := range []string{"greenhouse", "lever", "workday"} {
		r.Register(&stubPlugin{name: name})
	}
	names := r.ListPlugins()
	if len(names) != 3 {
		t.Errorf("expected 3 plugins, got %d: %v", len(names), names)
	}
}

func TestRegister_OverwritesSameName(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)
	r.Register(&stubPlugin{name: "greenhouse"})
	r.Register(&stubPlugin{name: "greenhouse"}) // overwrite
	names := r.ListPlugins()
	if len(names) != 1 {
		t.Errorf("expected 1 plugin after overwrite, got %d", len(names))
	}
}

// ── Get ───────────────────────────────────────────────────────────────────────

func TestGet_FoundPlugin(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)
	r.Register(&stubPlugin{name: "lever"})
	p, err := r.Get("lever")
	if err != nil {
		t.Fatalf("Get(lever): unexpected error: %v", err)
	}
	if p.Name() != "lever" {
		t.Errorf("Get(lever).Name() = %q; want %q", p.Name(), "lever")
	}
}

func TestGet_NotFound(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("Get(nonexistent): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention plugin name, got: %v", err)
	}
}

func TestGet_EmptyRegistry(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)
	_, err := r.Get("anything")
	if err == nil {
		t.Fatal("expected error from empty registry")
	}
}

// ── DetectATS ─────────────────────────────────────────────────────────────────

func TestDetectATS_MatchFound(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)
	r.Register(&stubPlugin{
		name:       "greenhouse",
		detectFunc: func(url string) bool { return strings.Contains(url, "greenhouse.io") },
	})

	p, ok := r.DetectATS("https://boards.greenhouse.io/company/jobs/123")
	if !ok {
		t.Fatal("DetectATS: expected match, got false")
	}
	if p.Name() != "greenhouse" {
		t.Errorf("DetectATS returned plugin %q; want %q", p.Name(), "greenhouse")
	}
}

func TestDetectATS_NoMatch(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)
	r.Register(&stubPlugin{
		name:       "greenhouse",
		detectFunc: func(url string) bool { return strings.Contains(url, "greenhouse.io") },
	})

	_, ok := r.DetectATS("https://unknown.example.com/jobs/1")
	if ok {
		t.Fatal("DetectATS: expected no match, got true")
	}
}

func TestDetectATS_EmptyRegistry(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)
	_, ok := r.DetectATS("https://boards.greenhouse.io/jobs/1")
	if ok {
		t.Fatal("DetectATS on empty registry: expected false, got true")
	}
}

// ── ListPlugins ───────────────────────────────────────────────────────────────

func TestListPlugins_ReturnsAllNames(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)
	expected := map[string]bool{"greenhouse": true, "lever": true, "workday": true}
	for name := range expected {
		r.Register(&stubPlugin{name: name})
	}

	names := r.ListPlugins()
	if len(names) != len(expected) {
		t.Fatalf("ListPlugins returned %d names, want %d", len(names), len(expected))
	}
	for _, n := range names {
		if !expected[n] {
			t.Errorf("unexpected plugin name in list: %q", n)
		}
	}
}

// ── Concurrent safety ─────────────────────────────────────────────────────────

func TestRegistry_ConcurrentRegisterAndGet(t *testing.T) {
	t.Parallel()
	r := newTestRegistry(t)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			r.Register(&stubPlugin{name: fmt.Sprintf("plugin-%d", i)})
		}
		close(done)
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = r.ListPlugins()
		}
	}()

	<-done
	// No race condition = pass
}
