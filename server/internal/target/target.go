package target

import (
	"context"
	"fmt"
	"time"
)

// Target receives GPS track files.
type Target interface {
	// Type returns the target type identifier (e.g. "dawarich").
	Type() string
	// AcceptedFormats returns the file formats this target can ingest,
	// ordered by preference (most preferred first).
	AcceptedFormats() []string
	// Send forwards a GPS track file to the target.
	Send(ctx context.Context, filename string, data []byte) error
}

// Config holds the connection settings shared by every target.
type Config struct {
	URL        string
	APIKey     string
	APIKeyFile string
	Timeout    time.Duration
}

// Constructor creates a Target from the shared connection config and the
// target's own typed config (nil if it has none).
type Constructor func(cfg Config, targetCfg any) (Target, error)

var registry = map[string]Constructor{}

// configPrototypes holds a zero value of each target type's config struct,
// keyed by type name, used to parse its TARGET__<TYPE>__* env vars.
var configPrototypes = map[string]any{}

// Register adds a target type to the global registry.
func Register(typeName string, ctor Constructor) {
	if _, exists := registry[typeName]; exists {
		panic(fmt.Sprintf("target type %q already registered", typeName))
	}
	registry[typeName] = ctor
}

// RegisterConfig declares a target type's config via a zero value of its config
// struct (with `env`/`default` field tags). Targets without one skip this and
// their Constructor receives a nil targetCfg.
func RegisterConfig(typeName string, prototype any) {
	if _, exists := configPrototypes[typeName]; exists {
		panic(fmt.Sprintf("target config %q already registered", typeName))
	}
	configPrototypes[typeName] = prototype
}

// ConfigPrototypes returns a copy of the registered config prototypes, keyed by
// target type.
func ConfigPrototypes() map[string]any {
	out := make(map[string]any, len(configPrototypes))
	for k, v := range configPrototypes {
		out[k] = v
	}
	return out
}

// Get creates a Target for the given type name and config.
func Get(typeName string, cfg Config, targetCfg any) (Target, error) {
	ctor, ok := registry[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown target type %q", typeName)
	}
	return ctor(cfg, targetCfg)
}
