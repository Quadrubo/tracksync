package dawarich

import (
	"strings"
	"testing"
	"time"

	"github.com/Quadrubo/tracksync/server/internal/converter"
	"github.com/stretchr/testify/assert"
)

func TestTransformTracks_TagsDistinctPerTrack(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)
	files := [][]converter.Track{{
		{Name: "Ride", Segments: []converter.Segment{{Points: []converter.Point{{Lat: 1, Lon: 2, Time: &t1}}}}},
		{Name: "Ride", Segments: []converter.Segment{{Points: []converter.Point{{Lat: 3, Lon: 4, Time: &t2}}}}},
	}}

	changed := (&Dawarich{emitTrackerID: true}).TransformTracks("geojson", files)

	assert.True(t, changed)
	a := files[0][0].Properties["tracker_id"]
	b := files[0][1].Properties["tracker_id"]
	assert.True(t, strings.HasPrefix(a, "tracksync-"))
	assert.NotEqual(t, a, b, "same name but different first point => distinct ids")
}

func TestTransformTracks_DisabledIsNoop(t *testing.T) {
	files := [][]converter.Track{{
		{Segments: []converter.Segment{{Points: []converter.Point{{Lat: 1, Lon: 2}}}}},
	}}

	changed := (&Dawarich{emitTrackerID: false}).TransformTracks("geojson", files)
	assert.False(t, changed)
	assert.Nil(t, files[0][0].Properties)
}

func TestTransformTracks_SkipsTrackWithoutPoints(t *testing.T) {
	files := [][]converter.Track{{{Name: "empty"}}}

	changed := (&Dawarich{emitTrackerID: true}).TransformTracks("geojson", files)
	assert.False(t, changed)
	assert.Nil(t, files[0][0].Properties)
}

func TestTransformTracks_NonGeoJSONIsNoop(t *testing.T) {
	files := [][]converter.Track{{
		{Name: "Ride", Segments: []converter.Segment{{Points: []converter.Point{{Lat: 1, Lon: 2}}}}},
	}}

	changed := (&Dawarich{emitTrackerID: true}).TransformTracks("gpx_1.1", files)
	assert.False(t, changed)
	assert.Nil(t, files[0][0].Properties)
}
