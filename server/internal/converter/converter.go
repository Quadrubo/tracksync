package converter

import (
	"fmt"
	"strings"
)

// OutputFile is a single converted file produced by Convert.
type OutputFile struct {
	Data     []byte
	Format   string
	Filename string
}

// Convert parses data in sourceFormat, applies markers, selects the best target
// format from acceptedFormats, and serializes the tracks into one or more output
// files.
//
// When passthrough is true and sourceFormat matches the best target format, the
// original data is returned unchanged; otherwise it is re-serialized. With
// markers.SplitMode == "files" each track is serialized into its own file.
func Convert(sourceFormat string, data []byte, acceptedFormats []string, originalFilename string, passthrough bool, markers MarkerOptions) ([]OutputFile, error) {
	parser, ok := GetParser(sourceFormat)
	if !ok {
		return nil, fmt.Errorf("no parser for format %q", sourceFormat)
	}

	tracks, err := parser.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", sourceFormat, err)
	}

	result := applyMarkers(tracks, markers)

	// Determine which fields the parsed tracks actually contain.
	usedFields := mergeUsedFields(result.Tracks())

	bestFormat := selectBestFormat(usedFields, acceptedFormats)
	if bestFormat == "" {
		return nil, fmt.Errorf("no serializer available for any accepted format: %v", acceptedFormats)
	}

	// Passthrough only when nothing was restructured; a split rewrites the tracks.
	if passthrough && bestFormat == sourceFormat && !result.Modified {
		return []OutputFile{{Data: data, Format: bestFormat, Filename: originalFilename}}, nil
	}

	serializer, ok := GetSerializer(bestFormat)
	if !ok {
		return nil, fmt.Errorf("no serializer for format %q", bestFormat)
	}

	// Suffix filenames only when split across multiple files.
	multiFile := len(result.Files) > 1
	out := make([]OutputFile, 0, len(result.Files))
	for i, fileTracks := range result.Files {
		fileData, ext, err := serializer.Serialize(fileTracks)
		if err != nil {
			return nil, fmt.Errorf("serializing to %s: %w", bestFormat, err)
		}
		filename := replaceExtension(originalFilename, ext)
		if multiFile {
			filename = indexedFilename(filename, i+1)
		}
		out = append(out, OutputFile{Data: fileData, Format: bestFormat, Filename: filename})
	}
	return out, nil
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

// indexedFilename inserts "-n" before the extension, e.g. track.geojson -> track-1.geojson.
func indexedFilename(filename string, n int) string {
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		return fmt.Sprintf("%s-%d%s", filename[:idx], n, filename[idx:])
	}
	return fmt.Sprintf("%s-%d", filename, n)
}
