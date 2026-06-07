package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// markedTrack builds a single-segment track from points described as
// (lat, marker) pairs. Marker "" is an ordinary trackpoint.
func markedTrack(pts ...struct {
	lat    float64
	marker string
}) Track {
	seg := Segment{}
	for _, p := range pts {
		seg.Points = append(seg.Points, Point{Lat: p.lat, Marker: p.marker})
	}
	return Track{Segments: []Segment{seg}}
}

type mp = struct {
	lat    float64
	marker string
}

// markerSet builds the lookup splitTrack expects from a list of marker values.
func markerSet(markers ...string) map[string]bool {
	set := make(map[string]bool, len(markers))
	for _, m := range markers {
		set[m] = true
	}
	return set
}

func TestSplitTrack_NoMarkersIsNoOp(t *testing.T) {
	in := markedTrack(mp{1, ""}, mp{2, "C"}, mp{3, ""})
	legs := splitTrack(in, markerSet(), false)
	require.Len(t, legs, 1)
	assert.Len(t, legs[0].Segments[0].Points, 3)
}

func TestSplitTrack_MarkerStart(t *testing.T) {
	// walk, POI(C), bus, POI(C), walk; G is present but not configured.
	in := markedTrack(mp{1, ""}, mp{2, "C"}, mp{3, ""}, mp{4, "C"}, mp{5, "G"}, mp{6, ""})
	legs := splitTrack(in, markerSet("C"), false)
	require.Len(t, legs, 3)
	assert.Len(t, legs[0].Segments[0].Points, 1)               // walk: pt1
	require.Len(t, legs[1].Segments[0].Points, 2)              // bus: marker pt2 + pt3
	assert.InDelta(t, 2, legs[1].Segments[0].Points[0].Lat, 0) // marker begins new leg
	require.Len(t, legs[2].Segments[0].Points, 3)              // walk: marker pt4 + G pt5 + pt6
	assert.InDelta(t, 4, legs[2].Segments[0].Points[0].Lat, 0)
}

func TestSplitTrack_MarkerEnd(t *testing.T) {
	in := markedTrack(mp{1, ""}, mp{2, "C"}, mp{3, ""}, mp{4, "C"}, mp{5, ""})
	legs := splitTrack(in, markerSet("C"), true)
	require.Len(t, legs, 3)
	require.Len(t, legs[0].Segments[0].Points, 2)              // walk pt1 + marker pt2 (ends leg)
	assert.InDelta(t, 2, legs[0].Segments[0].Points[1].Lat, 0) // marker stays at end
	assert.Len(t, legs[1].Segments[0].Points, 2)               // pt3 + marker pt4
	assert.Len(t, legs[2].Segments[0].Points, 1)               // pt5
}

func TestSplitTrack_MultipleMarkers(t *testing.T) {
	in := markedTrack(mp{1, ""}, mp{2, "C"}, mp{3, ""}, mp{4, "G"}, mp{5, ""})
	legs := splitTrack(in, markerSet("C", "G"), false)
	require.Len(t, legs, 3)
}

func TestSplitTrack_DropsEmptyLeadingMarker(t *testing.T) {
	// A marker as the very first point must not yield an empty leading leg, so the
	// track is not actually divided.
	in := markedTrack(mp{1, "C"}, mp{2, ""})
	legs := splitTrack(in, markerSet("C"), false)
	require.Len(t, legs, 1)
	assert.Len(t, legs[0].Segments[0].Points, 2)
}

func TestSplitTrack_PreservesNameAndSegments(t *testing.T) {
	// Two recording segments; a marker in the second splits it into a new leg.
	in := Track{
		Name: "trip",
		Segments: []Segment{
			{Points: []Point{{Lat: 1}, {Lat: 2}}},
			{Points: []Point{{Lat: 3}, {Lat: 4, Marker: "C"}, {Lat: 5}}},
		},
	}
	legs := splitTrack(in, markerSet("C"), false)
	require.Len(t, legs, 2)
	// First leg keeps both leading segments (gap preserved), name carried over.
	assert.Equal(t, "trip", legs[0].Name)
	require.Len(t, legs[0].Segments, 2)
	assert.Len(t, legs[0].Segments[0].Points, 2)
	assert.Len(t, legs[0].Segments[1].Points, 1) // pt3 before the marker
	// Second leg starts at the marker.
	assert.Equal(t, "trip", legs[1].Name)
	require.Len(t, legs[1].Segments, 1)
	assert.Len(t, legs[1].Segments[0].Points, 2) // marker pt4 + pt5
}
