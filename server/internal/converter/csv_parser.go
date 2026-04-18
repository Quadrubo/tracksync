package converter

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func init() {
	RegisterParser("columbus-csv", &csvParser{})
}

// csvParser parses Columbus P-10 Pro CSV files.
//
// Format: INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
// Example: 1,T,260417,110529,52.4194759N,13.3076437E,62,1.4,333
//
// - DATE is yymmdd (UTC)
// - TIME is hhmmss (UTC)
// - LATITUDE: decimal degrees with N/S suffix
// - LONGITUDE: decimal degrees with E/W suffix
// - HEIGHT: meters
// - SPEED: km/h
// - HEADING: degrees
// - TAG: T=trackpoint, C=POI, D=second POI, G=wake-up point (ignored, all rows are treated as trackpoints)
type csvParser struct{}

func (p *csvParser) Parse(data []byte) ([]Track, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV: %w", err)
	}

	if len(records) < 2 {
		return []Track{}, nil
	}

	// Skip header row
	records = records[1:]

	seg := Segment{}
	for i, record := range records {
		if len(record) < 9 {
			return nil, fmt.Errorf("row %d: expected 9 columns, got %d", i+2, len(record))
		}

		point, err := parseCSVRow(record, i+2)
		if err != nil {
			return nil, err
		}

		seg.Points = append(seg.Points, point)
	}

	if len(seg.Points) == 0 {
		return []Track{}, nil
	}

	return []Track{{Segments: []Segment{seg}}}, nil
}

func parseCSVRow(record []string, lineNum int) (Point, error) {
	var point Point

	// Parse latitude (e.g. "52.4194759N")
	lat, err := parseCoordinate(record[4])
	if err != nil {
		return point, fmt.Errorf("row %d: latitude: %w", lineNum, err)
	}
	point.Lat = lat

	// Parse longitude (e.g. "13.3076437E")
	lon, err := parseCoordinate(record[5])
	if err != nil {
		return point, fmt.Errorf("row %d: longitude: %w", lineNum, err)
	}
	point.Lon = lon

	// Parse date + time (yymmdd + hhmmss, UTC)
	t, err := parseDateTime(record[2], record[3])
	if err != nil {
		return point, fmt.Errorf("row %d: datetime: %w", lineNum, err)
	}
	point.Time = &t

	// Parse height (meters)
	if record[6] != "" {
		h, err := strconv.ParseFloat(record[6], 64)
		if err != nil {
			return point, fmt.Errorf("row %d: height: %w", lineNum, err)
		}
		point.Elevation = &h
	}

	// Parse speed (km/h -> m/s)
	if record[7] != "" {
		sKmh, err := strconv.ParseFloat(record[7], 64)
		if err != nil {
			return point, fmt.Errorf("row %d: speed: %w", lineNum, err)
		}
		sMs := sKmh / 3.6
		point.Speed = &sMs
	}

	// Parse heading (degrees)
	if record[8] != "" {
		h, err := strconv.ParseFloat(record[8], 64)
		if err != nil {
			return point, fmt.Errorf("row %d: heading: %w", lineNum, err)
		}
		point.Course = &h
	}

	return point, nil
}

// parseCoordinate parses "52.4194759N" or "13.3076437E" into signed decimal degrees.
func parseCoordinate(s string) (float64, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("coordinate too short: %q", s)
	}

	dir := s[len(s)-1]
	numStr := s[:len(s)-1]

	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing %q: %w", s, err)
	}

	switch dir {
	case 'S', 'W':
		return -val, nil
	case 'N', 'E':
		return val, nil
	default:
		return 0, fmt.Errorf("unknown direction %q in %q", string(dir), s)
	}
}

// parseDateTime parses yymmdd + hhmmss into time.Time (UTC).
func parseDateTime(dateStr, timeStr string) (time.Time, error) {
	if len(dateStr) != 6 {
		return time.Time{}, fmt.Errorf("expected 6-char date, got %q", dateStr)
	}
	if len(timeStr) != 6 {
		return time.Time{}, fmt.Errorf("expected 6-char time, got %q", timeStr)
	}

	year, err := strconv.Atoi(dateStr[0:2])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid year in %q", dateStr)
	}
	year += 2000 // 2-digit year
	month, err := strconv.Atoi(dateStr[2:4])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month in %q", dateStr)
	}
	day, err := strconv.Atoi(dateStr[4:6])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day in %q", dateStr)
	}

	hour, err := strconv.Atoi(timeStr[0:2])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid hour in %q", timeStr)
	}
	min, err := strconv.Atoi(timeStr[2:4])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid minute in %q", timeStr)
	}
	sec, err := strconv.Atoi(timeStr[4:6])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid second in %q", timeStr)
	}

	return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC), nil
}
