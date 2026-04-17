package converter

import "fmt"

// Serializer converts the universal Track model into a specific output format.
type Serializer interface {
	// Serialize converts tracks to bytes and returns the file extension (e.g. ".geojson").
	Serialize(tracks []Track) ([]byte, string, error)
}

var serializers = map[string]Serializer{}

// RegisterSerializer registers a serializer for a format. Panics if already registered.
func RegisterSerializer(format string, s Serializer) {
	if _, exists := serializers[format]; exists {
		panic(fmt.Sprintf("serializer for format %q already registered", format))
	}
	serializers[format] = s
}

// GetSerializer returns the serializer for the given format, if registered.
func GetSerializer(format string) (Serializer, bool) {
	s, ok := serializers[format]
	return s, ok
}
