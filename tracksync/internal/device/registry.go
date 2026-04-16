package device

import "fmt"

// Constructor creates a new Device instance.
type Constructor func() Device

var registry = map[string]Constructor{}

// Register adds a device type to the global registry.
// Panics if the type name is already registered.
func Register(typeName string, ctor Constructor) {
	if _, exists := registry[typeName]; exists {
		panic(fmt.Sprintf("device type %q already registered", typeName))
	}
	registry[typeName] = ctor
}

// Get returns a new Device for the given type name.
func Get(typeName string) (Device, bool) {
	ctor, ok := registry[typeName]
	if !ok {
		return nil, false
	}
	return ctor(), true
}

// RegisteredTypes returns all registered device type names.
func RegisteredTypes() []string {
	types := make([]string, 0, len(registry))
	for t := range registry {
		types = append(types, t)
	}
	return types
}
