package converter

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"strconv"
	"time"
)

func init() {
	RegisterParser("gpx_1.1", &gpxParser{})
}

type gpxParser struct{}

// GPX XML structures

type gpxFile struct {
	XMLName xml.Name `xml:"gpx"`
	Tracks  []gpxTrk `xml:"trk"`
}

type gpxTrk struct {
	Name     string       `xml:"name"`
	Segments []gpxTrkSeg  `xml:"trkseg"`
}

type gpxTrkSeg struct {
	Points []gpxTrkPt `xml:"trkpt"`
}

type gpxTrkPt struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Ele  *string `xml:"ele"`
	Time *string `xml:"time"`
	Sat  *string `xml:"sat"`
	HDOP *string `xml:"hdop"`
	VDOP *string `xml:"vdop"`
	PDOP *string `xml:"pdop"`
	Fix  *string `xml:"fix"`
}

// gpxTimeFormats lists xsd:dateTime layouts in order of preference.
// GPX 1.1 uses xsd:dateTime which allows fractional seconds and
// timestamps without timezone offsets (interpreted as UTC).
var gpxTimeFormats = []string{
	time.RFC3339Nano,               // 2025-01-15T08:30:00.123Z / +01:00
	"2006-01-02T15:04:05.999999999", // fractional seconds, no offset (UTC)
	"2006-01-02T15:04:05",           // no fractions, no offset (UTC)
}

func parseGPXTime(s string) (time.Time, error) {
	for _, layout := range gpxTimeFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format: %q", s)
}

func (p *gpxParser) Parse(data []byte) ([]Track, error) {
	var gpx gpxFile
	if err := xml.Unmarshal(data, &gpx); err != nil {
		return nil, fmt.Errorf("invalid GPX: %w", err)
	}

	var tracks []Track
	for _, trk := range gpx.Tracks {
		track := Track{Name: trk.Name}
		for _, seg := range trk.Segments {
			segment := Segment{}
			for _, pt := range seg.Points {
				point := Point{
					Lat: pt.Lat,
					Lon: pt.Lon,
				}

				if pt.Ele != nil {
					if v, err := strconv.ParseFloat(*pt.Ele, 64); err == nil {
						point.Elevation = &v
					} else {
						slog.Warn("gpx: invalid ele, ignoring", "value", *pt.Ele, "error", err)
					}
				}
				if pt.Time != nil {
					if t, err := parseGPXTime(*pt.Time); err == nil {
						point.Time = &t
					} else {
						slog.Warn("gpx: invalid time, ignoring", "value", *pt.Time, "error", err)
					}
				}
				if pt.Sat != nil {
					if v, err := strconv.Atoi(*pt.Sat); err == nil {
						point.Satellites = &v
					} else {
						slog.Warn("gpx: invalid sat, ignoring", "value", *pt.Sat, "error", err)
					}
				}
				if pt.HDOP != nil {
					if v, err := strconv.ParseFloat(*pt.HDOP, 64); err == nil {
						point.HDOP = &v
					} else {
						slog.Warn("gpx: invalid hdop, ignoring", "value", *pt.HDOP, "error", err)
					}
				}
				if pt.VDOP != nil {
					if v, err := strconv.ParseFloat(*pt.VDOP, 64); err == nil {
						point.VDOP = &v
					} else {
						slog.Warn("gpx: invalid vdop, ignoring", "value", *pt.VDOP, "error", err)
					}
				}
				if pt.PDOP != nil {
					if v, err := strconv.ParseFloat(*pt.PDOP, 64); err == nil {
						point.PDOP = &v
					} else {
						slog.Warn("gpx: invalid pdop, ignoring", "value", *pt.PDOP, "error", err)
					}
				}
				if pt.Fix != nil {
					point.Fix = pt.Fix
				}

				segment.Points = append(segment.Points, point)
			}
			track.Segments = append(track.Segments, segment)
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}
