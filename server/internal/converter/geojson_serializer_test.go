package converter

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeoJSONSerializer_FullPoint(t *testing.T) {
	ts := time.Date(2025, 1, 15, 8, 30, 0, 0, time.UTC)
	ele := 38.5
	spd := 3.5
	crs := 180.0
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
				Time: &ts, Elevation: &ele, Speed: &spd, Course: &crs,
				Satellites: &sats, HDOP: &hdop, VDOP: &vdop, PDOP: &pdop, Fix: &fix,
			}},
		}},
	}}

	data, ext, err := (&geojsonSerializer{}).Serialize(tracks)
	require.NoError(t, err)
	assert.Equal(t, ".geojson", ext)

	var fc geojsonFeatureCollection
	require.NoError(t, json.Unmarshal(data, &fc))

	assert.Equal(t, "FeatureCollection", fc.Type)
	require.Len(t, fc.Features, 1)

	f := fc.Features[0]
	assert.Equal(t, "Feature", f.Type)
	assert.Equal(t, "Point", f.Geometry.Type)
	assert.Equal(t, []float64{13.3694, 52.5249, 38.5}, f.Geometry.Coordinates)

	assert.Equal(t, "Test Track", f.Properties["track_name"])
	assert.Equal(t, "2025-01-15T08:30:00Z", f.Properties["time"])
	assert.InDelta(t, 3.5, f.Properties["speed"].(float64), 0.01)
	assert.InDelta(t, 180.0, f.Properties["course"].(float64), 0.01)
	assert.InDelta(t, 12.0, f.Properties["satellites"].(float64), 0.01)
	assert.InDelta(t, 0.9, f.Properties["hdop"].(float64), 0.01)
	assert.InDelta(t, 1.2, f.Properties["vdop"].(float64), 0.01)
	assert.InDelta(t, 1.5, f.Properties["pdop"].(float64), 0.01)
	assert.Equal(t, "3d", f.Properties["fix"])
}

func TestGeoJSONSerializer_MinimalPoint(t *testing.T) {
	tracks := []Track{{
		Segments: []Segment{{
			Points: []Point{{Lat: 50.0, Lon: 10.0}},
		}},
	}}

	data, _, err := (&geojsonSerializer{}).Serialize(tracks)
	require.NoError(t, err)

	var fc geojsonFeatureCollection
	require.NoError(t, json.Unmarshal(data, &fc))

	f := fc.Features[0]
	assert.Equal(t, []float64{10.0, 50.0}, f.Geometry.Coordinates)
	// No optional properties when absent
	assert.NotContains(t, f.Properties, "time")
	assert.NotContains(t, f.Properties, "speed")
	assert.NotContains(t, f.Properties, "track_name")
}

func TestGeoJSONSerializer_MultipleTracksAndSegments(t *testing.T) {
	tracks := []Track{
		{
			Name: "Track A",
			Segments: []Segment{
				{Points: []Point{{Lat: 1, Lon: 2}, {Lat: 3, Lon: 4}}},
				{Points: []Point{{Lat: 5, Lon: 6}}},
			},
		},
		{
			Name: "Track B",
			Segments: []Segment{
				{Points: []Point{{Lat: 7, Lon: 8}}},
			},
		},
	}

	data, _, err := (&geojsonSerializer{}).Serialize(tracks)
	require.NoError(t, err)

	var fc geojsonFeatureCollection
	require.NoError(t, json.Unmarshal(data, &fc))

	assert.Len(t, fc.Features, 4)
	assert.Equal(t, "Track A", fc.Features[0].Properties["track_name"])
	assert.Equal(t, "Track B", fc.Features[3].Properties["track_name"])
}

func TestGeoJSONSerializer_EmptyTracks(t *testing.T) {
	data, _, err := (&geojsonSerializer{}).Serialize([]Track{})
	require.NoError(t, err)

	var fc geojsonFeatureCollection
	require.NoError(t, json.Unmarshal(data, &fc))

	assert.Equal(t, "FeatureCollection", fc.Type)
	assert.Empty(t, fc.Features)
}

func TestGeoJSONSerializer_NoElevation(t *testing.T) {
	tracks := []Track{{
		Segments: []Segment{{
			Points: []Point{{Lat: 50.0, Lon: 10.0}},
		}},
	}}

	data, _, err := (&geojsonSerializer{}).Serialize(tracks)
	require.NoError(t, err)

	var fc geojsonFeatureCollection
	require.NoError(t, json.Unmarshal(data, &fc))

	// 2D coordinates when no elevation
	assert.Len(t, fc.Features[0].Geometry.Coordinates, 2)
}
