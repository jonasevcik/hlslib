package hls

import (
	"fmt"
	"math"
	"strings"
)

// HLSValidator checks structural validity of HLS playlists.
type HLSValidator struct{}

func NewHLSValidator() *HLSValidator {
	return &HLSValidator{}
}

// ValidateMediaPlaylist checks structural validity of a VOD media playlist.
func (v *HLSValidator) ValidateMediaPlaylist(p *MediaPlaylist) error {
	if p.Version < 6 {
		return fmt.Errorf("version must be >= 6 for fMP4")
	}
	if p.TargetDuration < 1 {
		return fmt.Errorf("target duration must be >= 1")
	}
	if p.PlaylistType != "VOD" && p.PlaylistType != "EVENT" {
		return fmt.Errorf("invalid playlist type %q: must be VOD or EVENT (RFC 8216 §4.3.3.5)", p.PlaylistType)
	}
	if p.MapURI == "" {
		return fmt.Errorf("EXT-X-MAP URI is required for fMP4")
	}
	if len(p.Segments) == 0 {
		return fmt.Errorf("no segments in playlist")
	}
	for i, seg := range p.Segments {
		if seg.Duration <= 0 {
			return fmt.Errorf("segment %d: duration must be > 0", i)
		}
		if seg.URI == "" {
			return fmt.Errorf("segment %d: URI is empty", i)
		}
		if strings.HasPrefix(seg.URI, "/") {
			return fmt.Errorf("segment %d: absolute URI not allowed: %s", i, seg.URI)
		}
		rounded := int(math.Round(seg.Duration))
		if rounded > p.TargetDuration {
			return fmt.Errorf("segment %d: rounded duration %d exceeds target duration %d", i, rounded, p.TargetDuration)
		}
		if seg.ByteRange != nil && seg.ByteRange.Start == nil && i == 0 {
			return fmt.Errorf("segment %d: offsetless byte range not allowed for first segment", i)
		}
	}
	if !p.EndList {
		return fmt.Errorf("EXT-X-ENDLIST is required for VOD")
	}
	return nil
}

// ValidateMasterPlaylist checks structural validity of a master playlist.
func (v *HLSValidator) ValidateMasterPlaylist(p *MasterPlaylist) error {
	if p.Version < 6 {
		return fmt.Errorf("version must be >= 6")
	}
	if len(p.Variants) == 0 {
		return fmt.Errorf("no variants in master playlist")
	}
	for i, variant := range p.Variants {
		if variant.Bandwidth == 0 {
			return fmt.Errorf("variant %d: BANDWIDTH is required", i)
		}
		if variant.Codecs == "" {
			return fmt.Errorf("variant %d: CODECS is required", i)
		}
		if variant.Resolution == "" {
			return fmt.Errorf("variant %d: RESOLUTION is required", i)
		}
		if variant.FrameRate <= 0 {
			return fmt.Errorf("variant %d: FRAME-RATE must be > 0", i)
		}
		if variant.URI == "" {
			return fmt.Errorf("variant %d: URI is empty", i)
		}
		if strings.HasPrefix(variant.URI, "/") {
			return fmt.Errorf("variant %d: absolute URI not allowed: %s", i, variant.URI)
		}
	}
	return nil
}

// ValidateCrossRendition checks that all playlists share the same EXT-X-TARGETDURATION
// and EXT-X-PLAYLIST-TYPE, as required by the HLS spec.
func (v *HLSValidator) ValidateCrossRendition(playlists []*MediaPlaylist) error {
	var refTarget int
	var refType string
	for i, p := range playlists {
		if p == nil {
			continue
		}
		if i == 0 {
			refTarget = p.TargetDuration
			refType = p.PlaylistType
			continue
		}
		if p.TargetDuration != refTarget {
			return fmt.Errorf("playlist %d has target duration %d, expected %d", i, p.TargetDuration, refTarget)
		}
		if p.PlaylistType != refType {
			return fmt.Errorf("playlist %d has playlist type %q, expected %q", i, p.PlaylistType, refType)
		}
	}
	return nil
}

// ValidateSampleDurationsForFPS returns an error if any sample implies a frame rate more
// than 2× declaredFPS. timescale is the video track timescale (e.g. 12288).
func ValidateSampleDurationsForFPS(durations []uint32, timescale uint32, declaredFPS float64) error {
	if len(durations) == 0 || timescale == 0 || declaredFPS <= 0 {
		return nil
	}
	var minDur uint32
	for _, d := range durations {
		if d > 0 && (minDur == 0 || d < minDur) {
			minDur = d
		}
	}
	if minDur == 0 {
		return nil
	}
	effectiveFPS := float64(timescale) / float64(minDur)
	if effectiveFPS > declaredFPS*2.0 {
		return fmt.Errorf("sample duration %d at timescale %d yields effective fps %.2f, exceeds 2× declared fps %.2f",
			minDur, timescale, effectiveFPS, declaredFPS)
	}
	return nil
}
