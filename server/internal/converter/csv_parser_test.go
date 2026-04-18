package converter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleCSV = `INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
1,T,260417,110529,52.5249440N,13.3693610E,38,1.4,333
2,T,260417,110530,52.5249780N,13.3694020E,38,1.4,9
3,T,260417,110531,52.5250120N,13.3694430E,38,1.4,13
`

func TestCSVParser_SampleData(t *testing.T) {
	tracks, err := (&csvParser{}).Parse([]byte(sampleCSV))
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	require.Len(t, tracks[0].Segments, 1)
	require.Len(t, tracks[0].Segments[0].Points, 3)

	p := tracks[0].Segments[0].Points[0]
	assert.InDelta(t, 52.5249440, p.Lat, 0.0000001)
	assert.InDelta(t, 13.3693610, p.Lon, 0.0000001)
	assert.Equal(t, time.Date(2026, 4, 17, 11, 5, 29, 0, time.UTC), *p.Time)
	assert.InDelta(t, 38.0, *p.Elevation, 0.1)
	assert.InDelta(t, 1.4/3.6, *p.Speed, 0.001) // km/h -> m/s
	assert.InDelta(t, 333.0, *p.Course, 0.1)
}

func TestCSVParser_SouthWest(t *testing.T) {
	data := `INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
1,T,200101,120000,33.8688197S,151.2092955W,5,0.0,0
`
	tracks, err := (&csvParser{}).Parse([]byte(data))
	require.NoError(t, err)
	p := tracks[0].Segments[0].Points[0]
	assert.InDelta(t, -33.8688197, p.Lat, 0.0000001)
	assert.InDelta(t, -151.2092955, p.Lon, 0.0000001)
}

func TestCSVParser_SpeedConversion(t *testing.T) {
	data := `INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
1,T,200101,120000,50.0N,10.0E,100,36.0,90
`
	tracks, err := (&csvParser{}).Parse([]byte(data))
	require.NoError(t, err)
	p := tracks[0].Segments[0].Points[0]
	assert.InDelta(t, 10.0, *p.Speed, 0.001) // 36 km/h = 10 m/s
}

func TestCSVParser_HeaderOnly(t *testing.T) {
	data := `INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
`
	tracks, err := (&csvParser{}).Parse([]byte(data))
	require.NoError(t, err)
	assert.Empty(t, tracks)
}

func TestCSVParser_Empty(t *testing.T) {
	tracks, err := (&csvParser{}).Parse([]byte(""))
	require.NoError(t, err)
	assert.Empty(t, tracks)
}

func TestCSVParser_InvalidRow(t *testing.T) {
	data := `INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
1,T,260417
`
	_, err := (&csvParser{}).Parse([]byte(data))
	assert.Error(t, err)
}

func TestCSVParser_InvalidCoordinate(t *testing.T) {
	data := `INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
1,T,260417,110529,invalidN,13.0E,62,1.4,333
`
	_, err := (&csvParser{}).Parse([]byte(data))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "latitude")
}

func TestCSVParser_DateParsing(t *testing.T) {
	data := `INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING
1,T,250101,235959,50.0N,10.0E,100,0.0,0
`
	tracks, err := (&csvParser{}).Parse([]byte(data))
	require.NoError(t, err)
	p := tracks[0].Segments[0].Points[0]
	assert.Equal(t, time.Date(2025, 1, 1, 23, 59, 59, 0, time.UTC), *p.Time)
}

func TestCSVParser_AllFieldsPopulated(t *testing.T) {
	tracks, err := (&csvParser{}).Parse([]byte(sampleCSV))
	require.NoError(t, err)
	p := tracks[0].Segments[0].Points[0]

	assert.NotNil(t, p.Time)
	assert.NotNil(t, p.Elevation)
	assert.NotNil(t, p.Speed)
	assert.NotNil(t, p.Course)
	// CSV doesn't have these
	assert.Nil(t, p.Satellites)
	assert.Nil(t, p.HDOP)
	assert.Nil(t, p.VDOP)
	assert.Nil(t, p.PDOP)
	assert.Nil(t, p.Fix)
}

func TestParseCoordinate(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"52.5249440N", 52.5249440},
		{"52.5249440S", -52.5249440},
		{"13.3693610E", 13.3693610},
		{"13.3693610W", -13.3693610},
		{"0.0N", 0.0},
	}
	for _, tt := range tests {
		v, err := parseCoordinate(tt.input)
		require.NoError(t, err, "input: %s", tt.input)
		assert.InDelta(t, tt.expected, v, 0.0000001, "input: %s", tt.input)
	}
}

func TestParseDateTime(t *testing.T) {
	dt, err := parseDateTime("260417", "110529")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 4, 17, 11, 5, 29, 0, time.UTC), dt)
}
