package converter

import (
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
	data, format, filename, err := Convert("gpx_1.1", gpxData, []string{"gpx_1.1", "geojson"}, "track.gpx", true)
	require.NoError(t, err)
	assert.Equal(t, "gpx_1.1", format)
	assert.Equal(t, "track.gpx", filename)
	assert.Equal(t, gpxData, data, "passthrough should return original bytes")
}

func TestConvert_GPXReserialized(t *testing.T) {
	gpxData := []byte(`<?xml version="1.0"?>
<gpx version="1.1">
  <trk><trkseg>
    <trkpt lat="52.5249" lon="13.3694"><ele>500</ele><time>2025-01-01T00:00:00Z</time></trkpt>
  </trkseg></trk>
</gpx>`)

	// GPX → GPX without passthrough: re-serialized
	data, format, filename, err := Convert("gpx_1.1", gpxData, []string{"gpx_1.1", "geojson"}, "track.gpx", false)
	require.NoError(t, err)
	assert.Equal(t, "gpx_1.1", format)
	assert.Equal(t, "track.gpx", filename)
	assert.NotEqual(t, gpxData, data, "should re-serialize, not passthrough")
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
	data, format, filename, err := Convert("gpx_1.1", gpxData, []string{"geojson", "gpx_1.1"}, "track.gpx", false)
	require.NoError(t, err)
	assert.Equal(t, "geojson", format)
	assert.Equal(t, "track.geojson", filename)
	assert.NotEqual(t, gpxData, data)
}

func TestConvert_NoParser(t *testing.T) {
	_, _, _, err := Convert("unknown", []byte("data"), []string{"gpx_1.1"}, "f.txt", false)
	assert.ErrorContains(t, err, "no parser")
}

func TestConvert_NoSerializer(t *testing.T) {
	gpxData := []byte(`<?xml version="1.0"?><gpx version="1.1"><trk><trkseg><trkpt lat="1" lon="2"/></trkseg></trk></gpx>`)
	_, _, _, err := Convert("gpx_1.1", gpxData, []string{"columbus-csv"}, "f.gpx", false)
	assert.ErrorContains(t, err, "no serializer")
}

func TestConvert_FilenameExtensionReplaced(t *testing.T) {
	gpxData := []byte(`<?xml version="1.0"?>
<gpx version="1.1"><trk><trkseg><trkpt lat="1" lon="2"/></trkseg></trk></gpx>`)

	_, _, filename, err := Convert("gpx_1.1", gpxData, []string{"geojson"}, "my.track.gpx", false)
	require.NoError(t, err)
	assert.Equal(t, "my.track.geojson", filename)
}

func TestReplaceExtension(t *testing.T) {
	assert.Equal(t, "track.geojson", replaceExtension("track.gpx", ".geojson"))
	assert.Equal(t, "my.track.geojson", replaceExtension("my.track.gpx", ".geojson"))
	assert.Equal(t, "noext.geojson", replaceExtension("noext", ".geojson"))
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
