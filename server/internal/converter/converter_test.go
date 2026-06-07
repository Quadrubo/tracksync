package converter

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectUsedFields_AllFields(t *testing.T) {
	ts := time.Now()
	ele := 100.0
	spd := 5.0
	crs := 90.0
	sats := 10
	hdop := 1.0
	vdop := 1.0
	pdop := 1.0
	fix := "3d"

	track := &Track{
		Segments: []Segment{{
			Points: []Point{{
				Lat: 1, Lon: 2,
				Time: &ts, Elevation: &ele, Speed: &spd, Course: &crs,
				Satellites: &sats, HDOP: &hdop, VDOP: &vdop, PDOP: &pdop, Fix: &fix,
			}},
		}},
	}

	fields := detectUsedFields(track)
	assert.True(t, fields[FieldLat])
	assert.True(t, fields[FieldLon])
	assert.True(t, fields[FieldTime])
	assert.True(t, fields[FieldElevation])
	assert.True(t, fields[FieldSpeed])
	assert.True(t, fields[FieldCourse])
	assert.True(t, fields[FieldSatellites])
	assert.True(t, fields[FieldHDOP])
	assert.True(t, fields[FieldVDOP])
	assert.True(t, fields[FieldPDOP])
	assert.True(t, fields[FieldFix])
}

func TestDetectUsedFields_MinimalPoint(t *testing.T) {
	track := &Track{
		Segments: []Segment{{
			Points: []Point{{Lat: 1, Lon: 2}},
		}},
	}

	fields := detectUsedFields(track)
	assert.True(t, fields[FieldLat])
	assert.True(t, fields[FieldLon])
	assert.False(t, fields[FieldSpeed])
	assert.False(t, fields[FieldTime])
}

func TestSelectBestFormat_PrefersFieldCoverage(t *testing.T) {
	// Track with speed data - GeoJSON should win over GPX
	usedFields := map[Field]bool{
		FieldLat:   true,
		FieldLon:   true,
		FieldTime:  true,
		FieldSpeed: true,
	}

	best := selectBestFormat(usedFields, []string{"gpx_1.1", "geojson"})
	assert.Equal(t, "geojson", best)
}

func TestSelectBestFormat_TieBreaksByOrder(t *testing.T) {
	// Only lat/lon/time - both GPX and GeoJSON cover these equally.
	// First in list wins.
	usedFields := map[Field]bool{
		FieldLat:  true,
		FieldLon:  true,
		FieldTime: true,
	}

	best := selectBestFormat(usedFields, []string{"gpx_1.1", "geojson"})
	assert.Equal(t, "gpx_1.1", best)
}

func TestSelectBestFormat_NoSerializer(t *testing.T) {
	usedFields := map[Field]bool{FieldLat: true, FieldLon: true}
	// columbus-csv has capabilities but no serializer registered
	best := selectBestFormat(usedFields, []string{"columbus-csv"})
	assert.Equal(t, "", best)
}

func TestSelectBestFormat_NoAcceptedFormats(t *testing.T) {
	usedFields := map[Field]bool{FieldLat: true}
	best := selectBestFormat(usedFields, []string{})
	assert.Equal(t, "", best)
}

func TestSelectBestFormat_UnknownFormat(t *testing.T) {
	usedFields := map[Field]bool{FieldLat: true}
	best := selectBestFormat(usedFields, []string{"unknown"})
	assert.Equal(t, "", best)
}

func TestConvert_GPXPassthrough(t *testing.T) {
	gpxData := []byte(`<?xml version="1.0"?>
<gpx version="1.1">
  <trk><trkseg>
    <trkpt lat="52.5249" lon="13.3694"><ele>500</ele><time>2025-01-01T00:00:00Z</time></trkpt>
  </trkseg></trk>
</gpx>`)

	// GPX → GPX with passthrough enabled: return original bytes
	outs, err := Convert("gpx_1.1", gpxData, []string{"gpx_1.1", "geojson"}, "track.gpx", true, MarkerOptions{})
	require.NoError(t, err)
	require.Len(t, outs, 1)
	assert.Equal(t, "gpx_1.1", outs[0].Format)
	assert.Equal(t, "track.gpx", outs[0].Filename)
	assert.Equal(t, gpxData, outs[0].Data, "passthrough should return original bytes")
}

func TestConvert_PassthroughSkippedWhenSplitOccurs(t *testing.T) {
	// A split that actually divides the tracks restructures them, so the original
	// (unsplit) bytes must not be returned via passthrough even when source and
	// target formats match.
	csvData := []byte(`INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
1,T,260417,110529,52.0N,13.0E,38,1.4,333
2,C,260417,110530,52.1N,13.1E,38,30.0,9
3,T,260417,110531,52.2N,13.2E,38,30.0,13
`)

	outs, err := Convert("columbus-csv", csvData, []string{"gpx_1.1"}, "track.csv", true, MarkerOptions{
		Rules:               []MarkerRule{{Marker: "C", Functionality: MarkerSplit}},
		SplitMarkerPosition: "start",
	})
	require.NoError(t, err)
	require.Len(t, outs, 1)
	assert.Equal(t, 2, strings.Count(string(outs[0].Data), "<trk>"), "split must produce two tracks, not passthrough")
}

func TestConvert_PassthroughWhenConfiguredSplitDoesNotFire(t *testing.T) {
	// A split rule is configured but the data carries no matching marker, so the
	// tracks are unchanged and passthrough still returns the original bytes.
	gpxData := []byte(`<?xml version="1.0"?>
<gpx version="1.1">
  <trk><trkseg>
    <trkpt lat="52.5249" lon="13.3694"><ele>500</ele><time>2025-01-01T00:00:00Z</time></trkpt>
  </trkseg></trk>
</gpx>`)

	outs, err := Convert("gpx_1.1", gpxData, []string{"gpx_1.1", "geojson"}, "track.gpx", true, MarkerOptions{
		Rules:               []MarkerRule{{Marker: "C", Functionality: MarkerSplit}},
		SplitMarkerPosition: "start",
	})
	require.NoError(t, err)
	require.Len(t, outs, 1)
	assert.Equal(t, "gpx_1.1", outs[0].Format)
	assert.Equal(t, gpxData, outs[0].Data, "no split fired: passthrough returns original bytes")
}

func TestConvert_GPXReserialized(t *testing.T) {
	gpxData := []byte(`<?xml version="1.0"?>
<gpx version="1.1">
  <trk><trkseg>
    <trkpt lat="52.5249" lon="13.3694"><ele>500</ele><time>2025-01-01T00:00:00Z</time></trkpt>
  </trkseg></trk>
</gpx>`)

	// GPX → GPX without passthrough: re-serialized
	outs, err := Convert("gpx_1.1", gpxData, []string{"gpx_1.1", "geojson"}, "track.gpx", false, MarkerOptions{})
	require.NoError(t, err)
	require.Len(t, outs, 1)
	assert.Equal(t, "gpx_1.1", outs[0].Format)
	assert.Equal(t, "track.gpx", outs[0].Filename)
	assert.NotEqual(t, gpxData, outs[0].Data, "should re-serialize, not passthrough")
}

func TestConvert_GPXToGeoJSON_WhenSpeedInTarget(t *testing.T) {
	// GPX has no speed, but the track only has lat/lon/time/ele - both formats
	// cover these fields equally, so first accepted format wins.
	gpxData := []byte(`<?xml version="1.0"?>
<gpx version="1.1">
  <trk><trkseg>
    <trkpt lat="52.5249" lon="13.3694"><ele>500</ele><time>2025-01-01T00:00:00Z</time></trkpt>
  </trkseg></trk>
</gpx>`)

	// GeoJSON listed first: wins the tie
	outs, err := Convert("gpx_1.1", gpxData, []string{"geojson", "gpx_1.1"}, "track.gpx", false, MarkerOptions{})
	require.NoError(t, err)
	require.Len(t, outs, 1)
	assert.Equal(t, "geojson", outs[0].Format)
	assert.Equal(t, "track.geojson", outs[0].Filename)
	assert.NotEqual(t, gpxData, outs[0].Data)
}

func TestConvert_NoParser(t *testing.T) {
	_, err := Convert("unknown", []byte("data"), []string{"gpx_1.1"}, "f.txt", false, MarkerOptions{})
	assert.ErrorContains(t, err, "no parser")
}

func TestConvert_NoSerializer(t *testing.T) {
	gpxData := []byte(`<?xml version="1.0"?><gpx version="1.1"><trk><trkseg><trkpt lat="1" lon="2"/></trkseg></trk></gpx>`)
	_, err := Convert("gpx_1.1", gpxData, []string{"columbus-csv"}, "f.gpx", false, MarkerOptions{})
	assert.ErrorContains(t, err, "no serializer")
}

func TestConvert_FilenameExtensionReplaced(t *testing.T) {
	gpxData := []byte(`<?xml version="1.0"?>
<gpx version="1.1"><trk><trkseg><trkpt lat="1" lon="2"/></trkseg></trk></gpx>`)

	outs, err := Convert("gpx_1.1", gpxData, []string{"geojson"}, "my.track.gpx", false, MarkerOptions{})
	require.NoError(t, err)
	require.Len(t, outs, 1)
	assert.Equal(t, "my.track.geojson", outs[0].Filename)
}

func TestReplaceExtension(t *testing.T) {
	assert.Equal(t, "track.geojson", replaceExtension("track.gpx", ".geojson"))
	assert.Equal(t, "my.track.geojson", replaceExtension("my.track.gpx", ".geojson"))
	assert.Equal(t, "noext.geojson", replaceExtension("noext", ".geojson"))
}

func TestConvert_SplitColumbusCSVToMultipleGPXTracks(t *testing.T) {
	csvData := []byte(`INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
1,T,260417,110529,52.0N,13.0E,38,1.4,333
2,C,260417,110530,52.1N,13.1E,38,30.0,9
3,T,260417,110531,52.2N,13.2E,38,30.0,13
`)

	// Restrict accepted formats to GPX so the split is observable as <trk> elements.
	outs, err := Convert("columbus-csv", csvData, []string{"gpx_1.1"}, "track.csv", false, MarkerOptions{Rules: []MarkerRule{{Marker: "C", Functionality: MarkerSplit}}, SplitMarkerPosition: "start"})
	require.NoError(t, err)
	require.Len(t, outs, 1, "tracks mode: one file")
	assert.Equal(t, "gpx_1.1", outs[0].Format)
	assert.Equal(t, 2, strings.Count(string(outs[0].Data), "<trk>"), "expected two tracks after split")
}

func TestConvert_SplitFilesMode_OneFilePerTrack(t *testing.T) {
	csvData := []byte(`INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
1,T,260417,110529,52.0N,13.0E,38,1.4,333
2,C,260417,110530,52.1N,13.1E,38,30.0,9
3,T,260417,110531,52.2N,13.2E,38,30.0,13
`)

	outs, err := Convert("columbus-csv", csvData, []string{"gpx_1.1"}, "track.csv", false, MarkerOptions{
		Rules:               []MarkerRule{{Marker: "C", Functionality: MarkerSplit}},
		SplitMarkerPosition: "start",
		SplitMode:           "files",
	})
	require.NoError(t, err)
	require.Len(t, outs, 2, "files mode: one file per track")
	assert.Equal(t, "track-1.gpx", outs[0].Filename)
	assert.Equal(t, "track-2.gpx", outs[1].Filename)
	// Each file holds exactly one track.
	assert.Equal(t, 1, strings.Count(string(outs[0].Data), "<trk>"))
	assert.Equal(t, 1, strings.Count(string(outs[1].Data), "<trk>"))
}

func TestConvert_FilesMode_SingleTrackNoSuffix(t *testing.T) {
	// files mode but no split: a single track keeps the plain filename.
	csvData := []byte(`INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
1,T,260417,110529,52.0N,13.0E,38,1.4,333
2,T,260417,110530,52.1N,13.1E,38,1.4,9
`)
	outs, err := Convert("columbus-csv", csvData, []string{"gpx_1.1"}, "track.csv", false, MarkerOptions{SplitMode: "files"})
	require.NoError(t, err)
	require.Len(t, outs, 1)
	assert.Equal(t, "track.gpx", outs[0].Filename)
}

func TestConvert_FilesMode_DoesNotSplitSourceNativeTracks(t *testing.T) {
	// Two source-native tracks and a split rule configured, but no point carries
	// the marker: files mode must not split native tracks into separate files when
	// no split actually fired.
	gpxData := []byte(`<gpx version="1.1">` +
		`<trk><trkseg><trkpt lat="1" lon="2"/></trkseg></trk>` +
		`<trk><trkseg><trkpt lat="3" lon="4"/></trkseg></trk>` +
		`</gpx>`)

	outs, err := Convert("gpx_1.1", gpxData, []string{"gpx_1.1"}, "track.gpx", false, MarkerOptions{
		Rules:               []MarkerRule{{Marker: "C", Functionality: MarkerSplit}},
		SplitMarkerPosition: "start",
		SplitMode:           "files",
	})
	require.NoError(t, err)
	require.Len(t, outs, 1, "no split fired: a single file regardless of files mode")
	assert.Equal(t, "track.gpx", outs[0].Filename)
	assert.Equal(t, 2, strings.Count(string(outs[0].Data), "<trk>"), "both source tracks kept in one file")
}

func TestConvert_NoSplitColumbusCSVSingleGPXTrack(t *testing.T) {
	csvData := []byte(`INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
1,T,260417,110529,52.0N,13.0E,38,1.4,333
2,C,260417,110530,52.1N,13.1E,38,30.0,9
`)

	outs, err := Convert("columbus-csv", csvData, []string{"gpx_1.1"}, "track.csv", false, MarkerOptions{})
	require.NoError(t, err)
	require.Len(t, outs, 1)
	assert.Equal(t, 1, strings.Count(string(outs[0].Data), "<trk>"), "no split tags configured: single track")
}

func TestMergeUsedFields(t *testing.T) {
	spd := 5.0
	ele := 100.0

	tracks := []Track{
		{Segments: []Segment{{Points: []Point{{Lat: 1, Lon: 2, Speed: &spd}}}}},
		{Segments: []Segment{{Points: []Point{{Lat: 3, Lon: 4, Elevation: &ele}}}}},
	}

	fields := mergeUsedFields(tracks)
	assert.True(t, fields[FieldSpeed])
	assert.True(t, fields[FieldElevation])
	assert.True(t, fields[FieldLat])
}
