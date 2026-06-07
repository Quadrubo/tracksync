package converter

import (
	"fmt"
	"strings"
)

// MarkerFunctionality is the behavior applied to points carrying a given marker.
type MarkerFunctionality string

const (
	// MarkerSplit starts a new track at the marked point.
	MarkerSplit MarkerFunctionality = "split"
)

// validFunctionalities is the set of recognised marker functionalities.
var validFunctionalities = map[MarkerFunctionality]bool{
	MarkerSplit: true,
}

// MarkerRule assigns a functionality to a point marker value, e.g. {C, split}.
type MarkerRule struct {
	Marker        string
	Functionality MarkerFunctionality
}

// MarkerOptions describes how parsed tracks are post-processed based on point markers.
type MarkerOptions struct {
	Rules []MarkerRule
	// SplitMarkerPosition places a split point at the "start" (default) of the
	// new track or the "end" of the previous one.
	SplitMarkerPosition string
	// SplitMode emits split tracks as one file ("tracks", default) or one file
	// per track ("files").
	SplitMode string
}

// ParseMarkerRules parses "marker:functionality" entries (e.g. "C:split") into MarkerRules.
func ParseMarkerRules(entries []string) ([]MarkerRule, error) {
	var rules []MarkerRule
	for _, e := range entries {
		marker, fn, ok := strings.Cut(e, ":")
		marker = strings.TrimSpace(marker)
		fn = strings.TrimSpace(fn)
		if !ok || marker == "" || fn == "" {
			return nil, fmt.Errorf("invalid marker rule %q: expected \"marker:functionality\"", e)
		}
		f := MarkerFunctionality(fn)
		if !validFunctionalities[f] {
			return nil, fmt.Errorf("unknown marker functionality %q in %q", fn, e)
		}
		rules = append(rules, MarkerRule{Marker: marker, Functionality: f})
	}
	return rules, nil
}

// MarkerResult is the outcome of applying marker rules to parsed tracks.
type MarkerResult struct {
	// Files partitions tracks into output files, one serialized file each.
	Files [][]Track
	// Modified is true when a split restructured the tracks, ruling out passthrough.
	Modified bool
}

// Tracks returns every track across all files, in order.
func (r MarkerResult) Tracks() []Track {
	var all []Track
	for _, f := range r.Files {
		all = append(all, f...)
	}
	return all
}

// applyMarkers splits tracks per the marker rules and partitions them into
// output files.
func applyMarkers(tracks []Track, opts MarkerOptions) MarkerResult {
	markerSet := make(map[string]bool)
	for _, r := range opts.Rules {
		if r.Functionality == MarkerSplit {
			markerSet[r.Marker] = true
		}
	}

	if len(markerSet) == 0 {
		return MarkerResult{Files: [][]Track{tracks}}
	}

	markerEndsTrack := opts.SplitMarkerPosition == "end"

	legsPerTrack := make([][]Track, 0, len(tracks))
	modified := false
	for _, t := range tracks {
		legs := splitTrack(t, markerSet, markerEndsTrack)
		if len(legs) > 1 {
			modified = true
		}
		legsPerTrack = append(legsPerTrack, legs)
	}

	// Nothing divided: keep the original tracks so passthrough stays possible.
	if !modified {
		return MarkerResult{Files: [][]Track{tracks}}
	}

	if opts.SplitMode != "files" {
		var all []Track
		for _, legs := range legsPerTrack {
			all = append(all, legs...)
		}
		return MarkerResult{Files: [][]Track{all}, Modified: true}
	}

	// files mode: one file per leg, in source order.
	var files [][]Track
	for _, legs := range legsPerTrack {
		for _, leg := range legs {
			files = append(files, []Track{leg})
		}
	}
	return MarkerResult{Files: files, Modified: true}
}
