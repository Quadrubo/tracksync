package converter

import (
	"encoding/json"
	"fmt"
)

func init() {
	RegisterSerializer("geojson", &geojsonSerializer{})
}

type geojsonSerializer struct{}

type geojsonFeatureCollection struct {
	Type     string           `json:"type"`
	Features []geojsonFeature `json:"features"`
}

type geojsonFeature struct {
	Type       string                 `json:"type"`
	Geometry   geojsonGeometry        `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

type geojsonGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

func (s *geojsonSerializer) Serialize(tracks []Track) ([]byte, string, error) {
	fc := geojsonFeatureCollection{
		Type:     "FeatureCollection",
		Features: []geojsonFeature{},
	}

	for _, track := range tracks {
		for _, seg := range track.Segments {
			for _, pt := range seg.Points {
				coords := []float64{pt.Lon, pt.Lat}
				if pt.Elevation != nil {
					coords = append(coords, *pt.Elevation)
				}

				props := map[string]interface{}{}

				if track.Name != "" {
					props["track_name"] = track.Name
				}
				if pt.Time != nil {
					props["time"] = pt.Time.UTC().Format("2006-01-02T15:04:05Z")
				}
				if pt.Speed != nil {
					props["speed"] = *pt.Speed
				}
				if pt.Course != nil {
					props["course"] = *pt.Course
				}
				if pt.Satellites != nil {
					props["satellites"] = *pt.Satellites
				}
				if pt.HDOP != nil {
					props["hdop"] = *pt.HDOP
				}
				if pt.VDOP != nil {
					props["vdop"] = *pt.VDOP
				}
				if pt.PDOP != nil {
					props["pdop"] = *pt.PDOP
				}
				if pt.Fix != nil {
					props["fix"] = *pt.Fix
				}

				fc.Features = append(fc.Features, geojsonFeature{
					Type:       "Feature",
					Geometry:   geojsonGeometry{Type: "Point", Coordinates: coords},
					Properties: props,
				})
			}
		}
	}

	data, err := json.Marshal(fc)
	if err != nil {
		return nil, "", fmt.Errorf("encoding GeoJSON: %w", err)
	}

	return data, ".geojson", nil
}
