package converter

import (
	"encoding/xml"
	"fmt"
)

func init() {
	RegisterSerializer("gpx_1.1", &gpxSerializer{})
}

type gpxSerializer struct{}

type gpxOutput struct {
	XMLName xml.Name      `xml:"gpx"`
	Version string        `xml:"version,attr"`
	Creator string        `xml:"creator,attr"`
	XMLNS   string        `xml:"xmlns,attr"`
	Tracks  []gpxTrkOut   `xml:"trk"`
}

type gpxTrkOut struct {
	Name     string          `xml:"name,omitempty"`
	Segments []gpxTrkSegOut  `xml:"trkseg"`
}

type gpxTrkSegOut struct {
	Points []gpxTrkPtOut `xml:"trkpt"`
}

type gpxTrkPtOut struct {
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Ele  *string `xml:"ele,omitempty"`
	Time *string `xml:"time,omitempty"`
	Sat  *string `xml:"sat,omitempty"`
	HDOP *string `xml:"hdop,omitempty"`
	VDOP *string `xml:"vdop,omitempty"`
	PDOP *string `xml:"pdop,omitempty"`
	Fix  *string `xml:"fix,omitempty"`
}

func (s *gpxSerializer) Serialize(tracks []Track) ([]byte, string, error) {
	gpx := gpxOutput{
		Version: "1.1",
		Creator: "tracksync",
		XMLNS:   "http://www.topografix.com/GPX/1/1",
	}

	for _, track := range tracks {
		trk := gpxTrkOut{Name: track.Name}
		for _, seg := range track.Segments {
			trkSeg := gpxTrkSegOut{}
			for _, pt := range seg.Points {
				trkPt := gpxTrkPtOut{
					Lat: pt.Lat,
					Lon: pt.Lon,
				}

				if pt.Elevation != nil {
					v := fmt.Sprintf("%g", *pt.Elevation)
					trkPt.Ele = &v
				}
				if pt.Time != nil {
					v := pt.Time.UTC().Format("2006-01-02T15:04:05Z")
					trkPt.Time = &v
				}
				if pt.Satellites != nil {
					v := fmt.Sprintf("%d", *pt.Satellites)
					trkPt.Sat = &v
				}
				if pt.HDOP != nil {
					v := fmt.Sprintf("%g", *pt.HDOP)
					trkPt.HDOP = &v
				}
				if pt.VDOP != nil {
					v := fmt.Sprintf("%g", *pt.VDOP)
					trkPt.VDOP = &v
				}
				if pt.PDOP != nil {
					v := fmt.Sprintf("%g", *pt.PDOP)
					trkPt.PDOP = &v
				}
				if pt.Fix != nil {
					trkPt.Fix = pt.Fix
				}

				trkSeg.Points = append(trkSeg.Points, trkPt)
			}
			trk.Segments = append(trk.Segments, trkSeg)
		}
		gpx.Tracks = append(gpx.Tracks, trk)
	}

	data, err := xml.MarshalIndent(gpx, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("encoding GPX: %w", err)
	}

	out := append([]byte(xml.Header), data...)
	return out, ".gpx", nil
}
