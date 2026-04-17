package converter

import (
	"encoding/xml"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGPXSerializer_FullPoint(t *testing.T) {
	ts := time.Date(2025, 1, 15, 8, 30, 0, 0, time.UTC)
	ele := 38.5
	sats := 12
	hdop := 0.9
	vdop := 1.2
	pdop := 1.5
	fix := "3d"

	tracks := []Track{{
		Name: "Test Track",
		Segments: []Segment{{
			Points: []Point{{
				Lat: 52.5249, Lon: 13.3694,
				Time: &ts, Elevation: &ele,
				Satellites: &sats, HDOP: &hdop, VDOP: &vdop, PDOP: &pdop, Fix: &fix,
			}},
		}},
	}}

	data, ext, err := (&gpxSerializer{}).Serialize(tracks)
	require.NoError(t, err)
	assert.Equal(t, ".gpx", ext)

	// Parse back to verify
	var gpx gpxFile
	require.NoError(t, xml.Unmarshal(data, &gpx))

	require.Len(t, gpx.Tracks, 1)
	assert.Equal(t, "Test Track", gpx.Tracks[0].Name)
	require.Len(t, gpx.Tracks[0].Segments, 1)
	require.Len(t, gpx.Tracks[0].Segments[0].Points, 1)

	pt := gpx.Tracks[0].Segments[0].Points[0]
	assert.Equal(t, 52.5249, pt.Lat)
	assert.Equal(t, 13.3694, pt.Lon)
	assert.Equal(t, "38.5", *pt.Ele)
	assert.Equal(t, "2025-01-15T08:30:00Z", *pt.Time)
	assert.Equal(t, "12", *pt.Sat)
	assert.Equal(t, "0.9", *pt.HDOP)
	assert.Equal(t, "1.2", *pt.VDOP)
	assert.Equal(t, "1.5", *pt.PDOP)
	assert.Equal(t, "3d", *pt.Fix)
}

func TestGPXSerializer_MinimalPoint(t *testing.T) {
	tracks := []Track{{
		Segments: []Segment{{
			Points: []Point{{Lat: 50.0, Lon: 10.0}},
		}},
	}}

	data, _, err := (&gpxSerializer{}).Serialize(tracks)
	require.NoError(t, err)

	var gpx gpxFile
	require.NoError(t, xml.Unmarshal(data, &gpx))

	pt := gpx.Tracks[0].Segments[0].Points[0]
	assert.Equal(t, 50.0, pt.Lat)
	assert.Equal(t, 10.0, pt.Lon)
	assert.Nil(t, pt.Ele)
	assert.Nil(t, pt.Time)
	assert.Nil(t, pt.Sat)
}

func TestGPXSerializer_RoundTrip(t *testing.T) {
	ts := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	ele := 100.0
	sats := 8
	fix := "2d"

	original := []Track{{
		Name: "Round Trip",
		Segments: []Segment{{
			Points: []Point{
				{Lat: 1.0, Lon: 2.0, Time: &ts, Elevation: &ele, Satellites: &sats, Fix: &fix},
				{Lat: 3.0, Lon: 4.0},
			},
		}},
	}}

	data, _, err := (&gpxSerializer{}).Serialize(original)
	require.NoError(t, err)

	parsed, err := (&gpxParser{}).Parse(data)
	require.NoError(t, err)

	require.Len(t, parsed, 1)
	assert.Equal(t, "Round Trip", parsed[0].Name)
	require.Len(t, parsed[0].Segments[0].Points, 2)

	p1 := parsed[0].Segments[0].Points[0]
	assert.Equal(t, 1.0, p1.Lat)
	assert.Equal(t, 2.0, p1.Lon)
	assert.InDelta(t, 100.0, *p1.Elevation, 0.1)
	assert.Equal(t, ts, *p1.Time)
	assert.Equal(t, 8, *p1.Satellites)
	assert.Equal(t, "2d", *p1.Fix)
}

func TestGPXSerializer_XMLHeader(t *testing.T) {
	tracks := []Track{{Segments: []Segment{{Points: []Point{{Lat: 1, Lon: 2}}}}}}
	data, _, err := (&gpxSerializer{}).Serialize(tracks)
	require.NoError(t, err)
	assert.Contains(t, string(data), `<?xml version="1.0" encoding="UTF-8"?>`)
	assert.Contains(t, string(data), `xmlns="http://www.topografix.com/GPX/1/1"`)
}

func TestGPXSerializer_Empty(t *testing.T) {
	data, _, err := (&gpxSerializer{}).Serialize([]Track{})
	require.NoError(t, err)
	assert.Contains(t, string(data), "<gpx")
}
