package converter

// splitTrack divides one track at points whose Marker is in markerSet, returning
// its legs (one leg means it was not divided). markerEndsTrack keeps the marked
// point on the previous leg ("end") rather than starting the new one ("start").
// Segment boundaries and the track name are preserved.
func splitTrack(track Track, markerSet map[string]bool, markerEndsTrack bool) []Track {
	var legs []Track
	cur := Track{Name: track.Name}
	seg := Segment{}

	flushSeg := func() {
		if len(seg.Points) > 0 {
			cur.Segments = append(cur.Segments, seg)
		}
		seg = Segment{}
	}
	flushLeg := func() {
		flushSeg()
		if len(cur.Segments) > 0 {
			legs = append(legs, cur)
		}
		cur = Track{Name: track.Name}
	}

	for _, origSeg := range track.Segments {
		for _, pt := range origSeg.Points {
			isSplit := pt.Marker != "" && markerSet[pt.Marker]
			if isSplit && markerEndsTrack {
				// Marker ends the current leg; the next point starts a new one.
				seg.Points = append(seg.Points, pt)
				flushLeg()
				continue
			}
			if isSplit {
				// Marker begins a new leg.
				flushLeg()
			}
			seg.Points = append(seg.Points, pt)
		}
		// Preserve the original recording gap as a segment boundary.
		flushSeg()
	}
	flushLeg()

	return legs
}
