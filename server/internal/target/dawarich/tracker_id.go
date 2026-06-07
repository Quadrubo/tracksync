package dawarich

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"

	"github.com/Quadrubo/tracksync/server/internal/converter"
)

// TransformTracks tags each track with a stable tracker_id so Dawarich keeps
// split tracks apart instead of re-segmenting points by time gap. GeoJSON-only:
// GPX already imports each <trk> as its own track.
func (d *Dawarich) TransformTracks(format string, files [][]converter.Track) (changed bool) {
	if !d.emitTrackerID || format != "geojson" {
		return false
	}
	for _, tracks := range files {
		for i := range tracks {
			id := trackerID(tracks[i])
			if id == "" {
				continue
			}
			if tracks[i].Properties == nil {
				tracks[i].Properties = map[string]string{}
			}
			tracks[i].Properties["tracker_id"] = id
			changed = true
		}
	}
	return changed
}

// trackerID derives a stable id from the track name and first point, unique per
// track and stable across re-uploads. Empty when the track has no points.
func trackerID(t converter.Track) string {
	first, ok := firstPoint(t)
	if !ok {
		return ""
	}

	key := t.Name + "|"
	if first.Time != nil {
		key += first.Time.UTC().Format("2006-01-02T15:04:05.000000000Z")
	}
	key += fmt.Sprintf("|%.7f|%.7f", first.Lat, first.Lon)

	sum := sha1.Sum([]byte(key))
	return "tracksync-" + hex.EncodeToString(sum[:])[:16]
}

func firstPoint(t converter.Track) (converter.Point, bool) {
	for _, seg := range t.Segments {
		if len(seg.Points) > 0 {
			return seg.Points[0], true
		}
	}
	return converter.Point{}, false
}
