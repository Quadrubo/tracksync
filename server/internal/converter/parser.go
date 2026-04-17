package converter

import "fmt"

// Parser converts raw file bytes in a specific format into the universal Track model.
type Parser interface {
	Parse(data []byte) ([]Track, error)
}

var parsers = map[string]Parser{}

// RegisterParser registers a parser for a format. Panics if already registered.
func RegisterParser(format string, p Parser) {
	if _, exists := parsers[format]; exists {
		panic(fmt.Sprintf("parser for format %q already registered", format))
	}
	parsers[format] = p
}

// GetParser returns the parser for the given format, if registered.
func GetParser(format string) (Parser, bool) {
	p, ok := parsers[format]
	return p, ok
}
