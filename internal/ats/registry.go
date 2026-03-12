package ats

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
)

// ApplicationData contains all pre-prepared data needed to fill an application.
type ApplicationData struct {
	ResumeFilePath   string            // Local path to tailored resume PDF
	CoverLetterText  string            // Cover letter content
	FullName         string
	Email            string
	Phone            string
	LinkedIn         string
	Website          string
	FormAnswers      map[string]string // Pre-answered form questions (question -> answer)
	AdditionalFields map[string]string // Any extra fields
}

// ApplicationResult contains the outcome of an application attempt.
type ApplicationResult struct {
	Success       bool
	ScreenshotURL string // S3 key of submission screenshot
	ErrorMessage  string
	ATSType       string
	StepsCompleted []string // Audit trail of steps completed
}

// Plugin is the interface that all ATS plugins must implement.
type Plugin interface {
	// Name returns the ATS name (e.g., "greenhouse", "lever", "workday")
	Name() string

	// Detect checks if a given URL belongs to this ATS.
	Detect(url string) bool

	// Apply fills out and submits a job application.
	// It receives a browser page context and pre-prepared application data.
	Apply(ctx context.Context, applyURL string, data *ApplicationData) (*ApplicationResult, error)

	// ValidateURL checks if the URL is a valid application URL for this ATS.
	ValidateURL(url string) bool
}

// Registry manages ATS plugin registration and lookup.
type Registry struct {
	plugins map[string]Plugin
	mu      sync.RWMutex
	log     zerolog.Logger
}

// NewRegistry creates a new plugin registry.
func NewRegistry(log zerolog.Logger) *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
		log:     log,
	}
}

// Register adds a plugin to the registry.
func (r *Registry) Register(plugin Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := plugin.Name()
	r.plugins[name] = plugin
	r.log.Info().Str("ats", name).Msg("ATS plugin registered")
}

// Get retrieves a plugin by name.
func (r *Registry) Get(name string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, ok := r.plugins[name]
	if !ok {
		return nil, fmt.Errorf("ATS plugin not found: %s", name)
	}
	return plugin, nil
}

// DetectATS tries to identify which ATS a URL belongs to.
func (r *Registry) DetectATS(url string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, plugin := range r.plugins {
		if plugin.Detect(url) {
			return plugin, true
		}
	}
	return nil, false
}

// ListPlugins returns the names of all registered plugins.
func (r *Registry) ListPlugins() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}
