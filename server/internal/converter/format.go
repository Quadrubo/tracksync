package converter

// Field represents a track point attribute that a format may support.
type Field int

const (
	FieldLat Field = iota
	FieldLon
	FieldTime
	FieldElevation
	FieldSpeed
	FieldCourse
	FieldSatellites
	FieldHDOP
	FieldVDOP
	FieldPDOP
	FieldFix
	fieldCount // must remain last
)

// FormatCapability describes which fields a format can encode.
type FormatCapability struct {
	Fields map[Field]bool
}

var formatCapabilities = map[string]FormatCapability{
	"gpx_1.1": {
		Fields: map[Field]bool{
			FieldLat:        true,
			FieldLon:        true,
			FieldTime:       true,
			FieldElevation:  true,
			FieldSatellites: true,
			FieldHDOP:       true,
			FieldVDOP:       true,
			FieldPDOP:       true,
			FieldFix:        true,
			// No standard speed/course in GPX 1.1
		},
	},
	"geojson": {
		Fields: map[Field]bool{
			FieldLat:        true,
			FieldLon:        true,
			FieldTime:       true,
			FieldElevation:  true,
			FieldSpeed:      true,
			FieldCourse:     true,
			FieldSatellites: true,
			FieldHDOP:       true,
			FieldVDOP:       true,
			FieldPDOP:       true,
			FieldFix:        true,
		},
	},
	"columbus-csv": {
		Fields: map[Field]bool{
			FieldLat:       true,
			FieldLon:       true,
			FieldTime:      true,
			FieldElevation: true,
			FieldSpeed:     true,
			FieldCourse:    true,
		},
	},
}

// detectUsedFields scans a track and returns the set of optional fields
// that have at least one non-nil value.
func detectUsedFields(track *Track) map[Field]bool {
	used := map[Field]bool{
		FieldLat: true,
		FieldLon: true,
	}
	for _, seg := range track.Segments {
		for _, p := range seg.Points {
			if p.Time != nil {
				used[FieldTime] = true
			}
			if p.Elevation != nil {
				used[FieldElevation] = true
			}
			if p.Speed != nil {
				used[FieldSpeed] = true
			}
			if p.Course != nil {
				used[FieldCourse] = true
			}
			if p.Satellites != nil {
				used[FieldSatellites] = true
			}
			if p.HDOP != nil {
				used[FieldHDOP] = true
			}
			if p.VDOP != nil {
				used[FieldVDOP] = true
			}
			if p.PDOP != nil {
				used[FieldPDOP] = true
			}
			if p.Fix != nil {
				used[FieldFix] = true
			}
			if len(used) == int(fieldCount) {
				return used
			}
		}
	}
	return used
}

// selectBestFormat picks the accepted format that preserves the most fields
// from the parsed track. Ties are broken by order in acceptedFormats
// (target's preference order).
func selectBestFormat(usedFields map[Field]bool, acceptedFormats []string) string {
	bestFormat := ""
	bestScore := -1

	for _, format := range acceptedFormats {
		cap, ok := formatCapabilities[format]
		if !ok {
			continue
		}
		if _, ok := GetSerializer(format); !ok {
			continue
		}
		score := 0
		for field := range usedFields {
			if cap.Fields[field] {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestFormat = format
		}
	}
	return bestFormat
}
