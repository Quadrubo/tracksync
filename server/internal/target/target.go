package target

import (
	"fmt"
	"time"
)

// Target receives GPS track files.
type Target interface {
	// Type returns the target type identifier (e.g. "dawarich").
	Type() string
	// Send forwards a GPS track file to the target.
	Send(filename string, data []byte) error
}

// Constructor creates a new Target from the given config.
type Constructor func(cfg Config) (Target, error)

// Config holds the target-agnostic configuration.
type Config struct {
	URL        string
	APIKey     string
	APIKeyFile string
	Timeout    time.Duration
}

var registry = map[string]Constructor{}

// Register adds a target type to the global registry.
func Register(typeName string, ctor Constructor) {
	if _, exists := registry[typeName]; exists {
		panic(fmt.Sprintf("target type %q already registered", typeName))
	}
	registry[typeName] = ctor
}

// Get creates a new Target for the given type name and config.
func Get(typeName string, cfg Config) (Target, error) {
	ctor, ok := registry[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown target type %q", typeName)
	}
	return ctor(cfg)
}
