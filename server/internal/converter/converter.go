package converter

import (
	"fmt"
	"strings"
)

// Convert parses data in sourceFormat, selects the best target format from
// acceptedFormats, and serializes the tracks. Returns converted data, chosen
// format, and the new filename.
//
// When passthrough is true and sourceFormat matches the best target format,
// the original data is returned unchanged. Otherwise, data is always
// re-serialized to produce normalized output.
func Convert(sourceFormat string, data []byte, acceptedFormats []string, originalFilename string, passthrough bool) ([]byte, string, string, error) {
	parser, ok := GetParser(sourceFormat)
	if !ok {
		return nil, "", "", fmt.Errorf("no parser for format %q", sourceFormat)
	}

	tracks, err := parser.Parse(data)
	if err != nil {
		return nil, "", "", fmt.Errorf("parsing %s: %w", sourceFormat, err)
	}

	// Determine which fields the parsed tracks actually contain.
	usedFields := mergeUsedFields(tracks)

	bestFormat := selectBestFormat(usedFields, acceptedFormats)
	if bestFormat == "" {
		return nil, "", "", fmt.Errorf("no serializer available for any accepted format: %v", acceptedFormats)
	}

	// Passthrough: if explicitly enabled and the best format matches the source,
	// return original data without re-serializing.
	if passthrough && bestFormat == sourceFormat {
		return data, bestFormat, originalFilename, nil
	}

	serializer, ok := GetSerializer(bestFormat)
	if !ok {
		return nil, "", "", fmt.Errorf("no serializer for format %q", bestFormat)
	}

	out, ext, err := serializer.Serialize(tracks)
	if err != nil {
		return nil, "", "", fmt.Errorf("serializing to %s: %w", bestFormat, err)
	}

	newFilename := replaceExtension(originalFilename, ext)
	return out, bestFormat, newFilename, nil
}

// mergeUsedFields combines detected fields across all tracks.
func mergeUsedFields(tracks []Track) map[Field]bool {
	merged := map[Field]bool{}
	for i := range tracks {
		for field := range detectUsedFields(&tracks[i]) {
			merged[field] = true
		}
	}
	return merged
}

// replaceExtension replaces the file extension with a new one.
func replaceExtension(filename, newExt string) string {
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		return filename[:idx] + newExt
	}
	return filename + newExt
}
