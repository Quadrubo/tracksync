package converter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTransformer struct {
	key string
	val string
}

func (s stubTransformer) TransformTracks(format string, files [][]Track) (changed bool) {
	for _, tracks := range files {
		for i := range tracks {
			if tracks[i].Properties == nil {
				tracks[i].Properties = map[string]string{}
			}
			tracks[i].Properties[s.key] = s.val
			changed = true
		}
	}
	return changed
}

func TestConvert_AppliesTransformer(t *testing.T) {
	gpxData := []byte(`<gpx version="1.1"><trk><trkseg><trkpt lat="1" lon="2"><time>2025-01-01T00:00:00Z</time></trkpt></trkseg></trk></gpx>`)

	outs, err := Convert("gpx_1.1", gpxData, []string{"geojson"}, "t.gpx", false, MarkerOptions{}, stubTransformer{key: "tracker_id", val: "abc"})
	require.NoError(t, err)
	require.Len(t, outs, 1)
	assert.Contains(t, string(outs[0].Data), `"tracker_id":"abc"`)
}

func TestConvert_TransformerSkipsPassthrough(t *testing.T) {
	gpxData := []byte(`<gpx version="1.1"><trk><trkseg><trkpt lat="1" lon="2"><time>2025-01-01T00:00:00Z</time></trkpt></trkseg></trk></gpx>`)

	// Passthrough would return the original bytes, but a transformer that mutates
	// the tracks must force re-serialization.
	outs, err := Convert("gpx_1.1", gpxData, []string{"gpx_1.1", "geojson"}, "t.gpx", true, MarkerOptions{}, stubTransformer{key: "x", val: "y"})
	require.NoError(t, err)
	require.Len(t, outs, 1)
	assert.NotEqual(t, gpxData, outs[0].Data)
}

func TestGeoJSONSerializer_EmitsProperties(t *testing.T) {
	tracks := []Track{{
		Properties: map[string]string{"tracker_id": "abc"},
		Segments:   []Segment{{Points: []Point{{Lat: 1, Lon: 2}}}},
	}}

	data, _, err := (&geojsonSerializer{}).Serialize(tracks)
	require.NoError(t, err)

	var fc geojsonFeatureCollection
	require.NoError(t, json.Unmarshal(data, &fc))
	assert.Equal(t, "abc", fc.Features[0].Properties["tracker_id"])
}
