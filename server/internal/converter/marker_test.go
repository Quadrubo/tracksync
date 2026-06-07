package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMarkerRules_Valid(t *testing.T) {
	rules, err := ParseMarkerRules([]string{"C:split", " D : split "})
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, MarkerRule{Marker: "C", Functionality: MarkerSplit}, rules[0])
	assert.Equal(t, MarkerRule{Marker: "D", Functionality: MarkerSplit}, rules[1])
}

func TestParseMarkerRules_Empty(t *testing.T) {
	rules, err := ParseMarkerRules(nil)
	require.NoError(t, err)
	assert.Empty(t, rules)
}

func TestParseMarkerRules_MissingFunctionality(t *testing.T) {
	_, err := ParseMarkerRules([]string{"C"})
	assert.ErrorContains(t, err, "marker:functionality")
}

func TestParseMarkerRules_UnknownFunctionality(t *testing.T) {
	_, err := ParseMarkerRules([]string{"C:teleport"})
	assert.ErrorContains(t, err, "unknown marker functionality")
}

func TestApplyMarkers_SplitFunctionality(t *testing.T) {
	in := []Track{markedTrack(mp{1, ""}, mp{2, "C"}, mp{3, ""})}
	res := applyMarkers(in, MarkerOptions{
		Rules:               []MarkerRule{{Marker: "C", Functionality: MarkerSplit}},
		SplitMarkerPosition: "start",
	})
	assert.True(t, res.Modified)
	// tracks mode (default): both resulting tracks land in a single file.
	require.Len(t, res.Files, 1)
	tracks := res.Tracks()
	require.Len(t, tracks, 2)
	assert.Len(t, tracks[0].Segments[0].Points, 1)
	assert.Len(t, tracks[1].Segments[0].Points, 2)
}

func TestApplyMarkers_SplitFilesMode(t *testing.T) {
	// files mode: each resulting track becomes its own output file.
	in := []Track{markedTrack(mp{1, ""}, mp{2, "C"}, mp{3, ""})}
	res := applyMarkers(in, MarkerOptions{
		Rules:               []MarkerRule{{Marker: "C", Functionality: MarkerSplit}},
		SplitMarkerPosition: "start",
		SplitMode:           "files",
	})
	assert.True(t, res.Modified)
	require.Len(t, res.Files, 2)
	assert.Len(t, res.Files[0], 1)
	assert.Len(t, res.Files[1], 1)
}

func TestApplyMarkers_NoRulesIsNoOp(t *testing.T) {
	in := []Track{markedTrack(mp{1, ""}, mp{2, "C"}, mp{3, ""})}
	res := applyMarkers(in, MarkerOptions{})
	assert.False(t, res.Modified)
	require.Len(t, res.Files, 1)
	tracks := res.Tracks()
	require.Len(t, tracks, 1)
	assert.Len(t, tracks[0].Segments[0].Points, 3)
}

func TestApplyMarkers_ConfiguredButNoMatchingMarker(t *testing.T) {
	// A split rule is configured but no point carries the marker: no split.
	in := []Track{markedTrack(mp{1, ""}, mp{2, "G"}, mp{3, ""})}
	res := applyMarkers(in, MarkerOptions{
		Rules:               []MarkerRule{{Marker: "C", Functionality: MarkerSplit}},
		SplitMarkerPosition: "start",
	})
	assert.False(t, res.Modified)
	require.Len(t, res.Files, 1)
	tracks := res.Tracks()
	require.Len(t, tracks, 1)
	assert.Len(t, tracks[0].Segments[0].Points, 3)
}

func TestApplyMarkers_FilesMode_PreservesSourceOrder(t *testing.T) {
	// One file per track, in source order: an undivided track keeps its slot and a
	// divided track expands into consecutive files where it sits.
	in := []Track{
		markedTrack(mp{10, ""}),                       // untouched, before the split
		markedTrack(mp{1, ""}, mp{2, "C"}, mp{3, ""}), // divided into two legs
		markedTrack(mp{20, ""}),                       // untouched, after the split
	}
	res := applyMarkers(in, MarkerOptions{
		Rules:               []MarkerRule{{Marker: "C", Functionality: MarkerSplit}},
		SplitMarkerPosition: "start",
		SplitMode:           "files",
	})
	assert.True(t, res.Modified)
	require.Len(t, res.Files, 4, "one file per resulting track")
	// Each file holds exactly one track, ordered as in the source.
	for i, f := range res.Files {
		require.Lenf(t, f, 1, "file %d holds a single track", i)
	}
	assert.InDelta(t, 10, res.Files[0][0].Segments[0].Points[0].Lat, 0, "untouched track keeps its leading slot")
	assert.InDelta(t, 1, res.Files[1][0].Segments[0].Points[0].Lat, 0, "first split leg")
	assert.InDelta(t, 2, res.Files[2][0].Segments[0].Points[0].Lat, 0, "second split leg")
	assert.InDelta(t, 20, res.Files[3][0].Segments[0].Points[0].Lat, 0, "trailing untouched track keeps its slot")
}

func TestApplyMarkers_EmptySiblingDoesNotMaskSplit(t *testing.T) {
	// A divided track alongside an empty source track: the empty track must not
	// hide that a split happened.
	in := []Track{
		markedTrack(mp{1, ""}, mp{2, "C"}, mp{3, ""}),
		{}, // empty track: no segments, no points
	}
	res := applyMarkers(in, MarkerOptions{
		Rules:               []MarkerRule{{Marker: "C", Functionality: MarkerSplit}},
		SplitMarkerPosition: "start",
	})
	assert.True(t, res.Modified, "an empty source track must not mask a real split")
	require.Len(t, res.Files, 1) // tracks mode
	assert.Len(t, res.Tracks(), 2, "two legs, empty track contributes nothing")
}

func TestApplyMarkers_OnlyConfiguredMarkerSplits(t *testing.T) {
	// "C" is mapped to split; "G" carries no rule and is left in place.
	in := []Track{markedTrack(mp{1, ""}, mp{2, "G"}, mp{3, "C"}, mp{4, ""})}
	res := applyMarkers(in, MarkerOptions{
		Rules:               []MarkerRule{{Marker: "C", Functionality: MarkerSplit}},
		SplitMarkerPosition: "start",
	})
	assert.True(t, res.Modified)
	tracks := res.Tracks()
	require.Len(t, tracks, 2)
	assert.Len(t, tracks[0].Segments[0].Points, 2) // pt1 + G pt2
	assert.Len(t, tracks[1].Segments[0].Points, 2) // C pt3 + pt4
}
