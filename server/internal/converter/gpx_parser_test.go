package converter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGPXParser_FullTrack(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="test">
  <trk>
    <name>Morning Run</name>
    <trkseg>
      <trkpt lat="52.5249" lon="13.3694">
        <ele>38.5</ele>
        <time>2025-01-15T08:30:00Z</time>
        <sat>12</sat>
        <hdop>0.9</hdop>
        <vdop>1.2</vdop>
        <pdop>1.5</pdop>
        <fix>3d</fix>
      </trkpt>
      <trkpt lat="52.5251" lon="13.3697">
        <ele>39.0</ele>
        <time>2025-01-15T08:30:05Z</time>
      </trkpt>
    </trkseg>
  </trk>
</gpx>`)

	tracks, err := (&gpxParser{}).Parse(data)
	require.NoError(t, err)
	require.Len(t, tracks, 1)

	track := tracks[0]
	assert.Equal(t, "Morning Run", track.Name)
	require.Len(t, track.Segments, 1)
	require.Len(t, track.Segments[0].Points, 2)

	p1 := track.Segments[0].Points[0]
	assert.Equal(t, 52.5249, p1.Lat)
	assert.Equal(t, 13.3694, p1.Lon)
	assert.InDelta(t, 38.5, *p1.Elevation, 0.01)
	assert.Equal(t, time.Date(2025, 1, 15, 8, 30, 0, 0, time.UTC), *p1.Time)
	assert.Equal(t, 12, *p1.Satellites)
	assert.InDelta(t, 0.9, *p1.HDOP, 0.01)
	assert.InDelta(t, 1.2, *p1.VDOP, 0.01)
	assert.InDelta(t, 1.5, *p1.PDOP, 0.01)
	assert.Equal(t, "3d", *p1.Fix)

	// Speed and Course should be nil (not in GPX 1.1)
	assert.Nil(t, p1.Speed)
	assert.Nil(t, p1.Course)

	p2 := track.Segments[0].Points[1]
	assert.Equal(t, 52.5251, p2.Lat)
	assert.Nil(t, p2.Satellites, "optional fields absent in point 2")
}

func TestGPXParser_MultipleSegments(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<gpx version="1.1">
  <trk>
    <trkseg>
      <trkpt lat="1.0" lon="2.0"/>
    </trkseg>
    <trkseg>
      <trkpt lat="3.0" lon="4.0"/>
    </trkseg>
  </trk>
</gpx>`)

	tracks, err := (&gpxParser{}).Parse(data)
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Len(t, tracks[0].Segments, 2)
	assert.Equal(t, 1.0, tracks[0].Segments[0].Points[0].Lat)
	assert.Equal(t, 3.0, tracks[0].Segments[1].Points[0].Lat)
}

func TestGPXParser_MultipleTracks(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<gpx version="1.1">
  <trk><name>A</name><trkseg><trkpt lat="1" lon="2"/></trkseg></trk>
  <trk><name>B</name><trkseg><trkpt lat="3" lon="4"/></trkseg></trk>
</gpx>`)

	tracks, err := (&gpxParser{}).Parse(data)
	require.NoError(t, err)
	require.Len(t, tracks, 2)
	assert.Equal(t, "A", tracks[0].Name)
	assert.Equal(t, "B", tracks[1].Name)
}

func TestGPXParser_EmptyTrack(t *testing.T) {
	data := []byte(`<?xml version="1.0"?><gpx version="1.1"></gpx>`)
	tracks, err := (&gpxParser{}).Parse(data)
	require.NoError(t, err)
	assert.Empty(t, tracks)
}

func TestGPXParser_InvalidXML(t *testing.T) {
	_, err := (&gpxParser{}).Parse([]byte("not xml"))
	assert.Error(t, err)
}

func TestGPXParser_TimeFractionalSeconds(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<gpx version="1.1">
  <trk><trkseg>
    <trkpt lat="1" lon="2"><time>2025-01-15T08:30:00.123Z</time></trkpt>
  </trkseg></trk>
</gpx>`)

	tracks, err := (&gpxParser{}).Parse(data)
	require.NoError(t, err)
	p := tracks[0].Segments[0].Points[0]
	require.NotNil(t, p.Time)
	assert.Equal(t, time.Date(2025, 1, 15, 8, 30, 0, 123000000, time.UTC), *p.Time)
}

func TestGPXParser_TimeNoOffset(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<gpx version="1.1">
  <trk><trkseg>
    <trkpt lat="1" lon="2"><time>2025-01-15T08:30:00</time></trkpt>
  </trkseg></trk>
</gpx>`)

	tracks, err := (&gpxParser{}).Parse(data)
	require.NoError(t, err)
	p := tracks[0].Segments[0].Points[0]
	require.NotNil(t, p.Time)
	assert.Equal(t, time.Date(2025, 1, 15, 8, 30, 0, 0, time.UTC), *p.Time)
}

func TestGPXParser_TimeFractionalNoOffset(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<gpx version="1.1">
  <trk><trkseg>
    <trkpt lat="1" lon="2"><time>2025-01-15T08:30:00.500</time></trkpt>
  </trkseg></trk>
</gpx>`)

	tracks, err := (&gpxParser{}).Parse(data)
	require.NoError(t, err)
	p := tracks[0].Segments[0].Points[0]
	require.NotNil(t, p.Time)
	assert.Equal(t, time.Date(2025, 1, 15, 8, 30, 0, 500000000, time.UTC), *p.Time)
}

func TestGPXParser_TimeWithNonUTCOffset(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<gpx version="1.1">
  <trk><trkseg>
    <trkpt lat="1" lon="2"><time>2025-01-15T09:30:00+01:00</time></trkpt>
  </trkseg></trk>
</gpx>`)

	tracks, err := (&gpxParser{}).Parse(data)
	require.NoError(t, err)
	p := tracks[0].Segments[0].Points[0]
	require.NotNil(t, p.Time)
	assert.Equal(t, time.Date(2025, 1, 15, 8, 30, 0, 0, time.UTC), *p.Time, "should be normalized to UTC")
}

func TestGPXParser_MinimalPoint(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<gpx version="1.1">
  <trk><trkseg><trkpt lat="50.0" lon="10.0"/></trkseg></trk>
</gpx>`)

	tracks, err := (&gpxParser{}).Parse(data)
	require.NoError(t, err)
	p := tracks[0].Segments[0].Points[0]
	assert.Equal(t, 50.0, p.Lat)
	assert.Equal(t, 10.0, p.Lon)
	assert.Nil(t, p.Time)
	assert.Nil(t, p.Elevation)
	assert.Nil(t, p.Satellites)
	assert.Nil(t, p.HDOP)
	assert.Nil(t, p.VDOP)
	assert.Nil(t, p.PDOP)
	assert.Nil(t, p.Fix)
}
