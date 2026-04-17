package converter

import "time"

// Track is the universal internal representation of a GPS track.
// It is a superset of all attributes across supported formats.
type Track struct {
	Name     string
	Segments []Segment
}

// Segment is a contiguous sequence of track points.
// A break in recording starts a new segment.
type Segment struct {
	Points []Point
}

// Point represents a single GPS fix with all possible attributes.
// Optional fields use pointers to distinguish "absent" from "zero".
type Point struct {
	Lat float64
	Lon float64

	Time      *time.Time
	Elevation *float64 // meters
	Speed     *float64 // m/s
	Course    *float64 // degrees from true north, 0-360

	Satellites *int
	HDOP       *float64
	VDOP       *float64
	PDOP       *float64
	Fix        *string // "none", "2d", "3d", "dgps", "pps"
}
