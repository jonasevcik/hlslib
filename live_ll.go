package hls

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// RenditionReport carries the last-known state of a sibling rendition for
// EXT-X-RENDITION-REPORT (RFC 8216bis §11.2). Include one entry per rendition
// that is not the one being rendered. LastPart < 0 omits LAST-PART (use for
// non-LL renditions that have no partial segments).
type RenditionReport struct {
	URI      string // relative playlist URI
	LastMSN  int    // last known Media Sequence Number
	LastPart int    // last known part index; -1 → omit LAST-PART
}

// LivePartByteRange is one partial segment described as a byte range within its parent segment file.
type LivePartByteRange struct {
	URI         string // same URI as the parent segment
	ByteOffset  int64
	ByteLength  int64
	DurationMs  int64
	Independent bool // true → emit INDEPENDENT=YES
}

// LLLiveSegment is a completed segment with its constituent parts.
type LLLiveSegment struct {
	TfdtValue  int64
	WallClock  time.Time
	DurationMs int64
	SizeBytes  int64
	URI        string
	Parts      []LivePartByteRange
}

// LLLiveMediaPlaylist is an LL-HLS live media playlist with partial segment support.
// It is safe for concurrent use.
type LLLiveMediaPlaylist struct {
	mu             sync.Mutex
	targetDuration int   // ceiling segment duration (segmentLengthSec + 1)
	partTargetMs   int64 // target part duration in ms
	mapURI         string
	segments       []LLLiveSegment
	pendingParts   []LivePartByteRange // parts of the in-progress (not yet finalized) segment
	pendingWall    time.Time           // wall clock of the in-progress segment's first frame
	// preloadHintURI and preloadHintByteStart describe the next expected part.
	preloadHintURI       string
	preloadHintByteStart int64
	mediaSequence        int
	ended                bool
}

// NewLLLiveMediaPlaylist creates an LL-HLS media playlist.
// targetDurationSec should be segmentLengthSec + 1.
// partTargetMs is LowLatencyConfig.PartTargetDurationMs.
func NewLLLiveMediaPlaylist(targetDurationSec int, partTargetMs int64, mapURI string) *LLLiveMediaPlaylist {
	return &LLLiveMediaPlaylist{
		targetDuration: targetDurationSec,
		partTargetMs:   partTargetMs,
		mapURI:         mapURI,
	}
}

// AddPart appends a completed part to the in-progress segment's pending list.
// wallClock is used to set pendingWall on the first part of each segment.
func (p *LLLiveMediaPlaylist) AddPart(part LivePartByteRange, wallClock time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pendingParts) == 0 {
		p.pendingWall = wallClock
	}
	p.pendingParts = append(p.pendingParts, part)
}

// SetPreloadHint updates the EXT-X-PRELOAD-HINT for the next expected part.
func (p *LLLiveMediaPlaylist) SetPreloadHint(uri string, byteStart int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.preloadHintURI = uri
	p.preloadHintByteStart = byteStart
}

// End marks the playlist as finished. The next Render call will include
// #EXT-X-ENDLIST and will omit EXT-X-PRELOAD-HINT (RFC 8216 §4.3.3.4).
// End is idempotent.
func (p *LLLiveMediaPlaylist) End() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ended = true
	p.preloadHintURI = ""
	p.preloadHintByteStart = 0
}

// CommitSegment moves pending parts into a new completed segment and clears pending state.
func (p *LLLiveMediaPlaylist) CommitSegment(tfdtValue int64, wallClock time.Time, durationMs, sizeBytes int64, uri string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	parts := make([]LivePartByteRange, len(p.pendingParts))
	copy(parts, p.pendingParts)
	p.segments = append(p.segments, LLLiveSegment{
		TfdtValue:  tfdtValue,
		WallClock:  wallClock,
		DurationMs: durationMs,
		SizeBytes:  sizeBytes,
		URI:        uri,
		Parts:      parts,
	})
	p.pendingParts = p.pendingParts[:0]
	p.pendingWall = time.Time{}
	p.preloadHintURI = ""
	p.preloadHintByteStart = 0
}

// Trim removes segments whose WallClock is older than dvr from the front of the window.
func (p *LLLiveMediaPlaylist) Trim(dvr time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cutoff := time.Now().Add(-dvr)
	for len(p.segments) > 0 && p.segments[0].WallClock.Before(cutoff) {
		p.segments = p.segments[1:]
		p.mediaSequence++
	}
}

// Segments returns a copy of the current completed segment window (thread-safe).
func (p *LLLiveMediaPlaylist) Segments() []LLLiveSegment {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]LLLiveSegment, len(p.segments))
	copy(out, p.segments)
	return out
}

// MediaSequence returns the current EXT-X-MEDIA-SEQUENCE value.
func (p *LLLiveMediaPlaylist) MediaSequence() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mediaSequence
}

// CurrentMSN returns the MSN of the in-progress (not-yet-finalized) segment
// and the number of pending parts committed to it so far (0 if none).
func (p *LLLiveMediaPlaylist) CurrentMSN() (msn int, pendingPartCount int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mediaSequence + len(p.segments), len(p.pendingParts)
}

// CanSkipCount returns the number of leading segments eligible for EXT-X-SKIP
// (those whose WallClock is older than 6 × targetDuration seconds, per bis §9.5).
func (p *LLLiveMediaPlaylist) CanSkipCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	threshold := time.Now().Add(-time.Duration(6*p.targetDuration) * time.Second)
	n := 0
	for _, seg := range p.segments {
		if seg.WallClock.Before(threshold) {
			n++
		} else {
			break
		}
	}
	return n
}

// Render produces the M3U8 text for the current window snapshot.
// skipSegments > 0 produces a Playlist Delta Update (bis §9.5): the first
// skipSegments entries are replaced with EXT-X-SKIP. Pass 0 for a full playlist.
// reports contains one RenditionReport per sibling rendition; pass nil when
// there are no siblings or reports are unavailable.
func (p *LLLiveMediaPlaylist) Render(skipSegments int, reports []RenditionReport) string {
	p.mu.Lock()
	segs := make([]LLLiveSegment, len(p.segments))
	copy(segs, p.segments)
	pending := make([]LivePartByteRange, len(p.pendingParts))
	copy(pending, p.pendingParts)
	pendingWall := p.pendingWall
	seq := p.mediaSequence
	hintURI := p.preloadHintURI
	hintByteStart := p.preloadHintByteStart
	ended := p.ended
	p.mu.Unlock()

	partTargetSec := float64(p.partTargetMs) / 1000.0
	holdBack := 3 * p.targetDuration
	partHoldBack := 3.0*partTargetSec + 0.001 // 1 ms above the 3× minimum avoids FP boundary in validators
	canSkipUntil := float64(6 * p.targetDuration)

	var buf strings.Builder
	fmt.Fprintf(&buf, "#EXTM3U\n")
	fmt.Fprintf(&buf, "#EXT-X-VERSION:9\n")
	fmt.Fprintf(&buf, "#EXT-X-TARGETDURATION:%d\n", p.targetDuration)
	fmt.Fprintf(&buf, "#EXT-X-MEDIA-SEQUENCE:%d\n", seq)
	fmt.Fprintf(&buf, "#EXT-X-PART-INF:PART-TARGET=%.6f\n", partTargetSec)
	fmt.Fprintf(&buf, "#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES,CAN-SKIP-UNTIL=%.6f,PART-HOLD-BACK=%.6f,HOLD-BACK=%d\n",
		canSkipUntil, partHoldBack, holdBack)

	if p.mapURI != "" {
		fmt.Fprintf(&buf, "#EXT-X-MAP:URI=\"%s\"\n", p.mapURI)
	}

	// Playlist Delta Update: replace leading segments with EXT-X-SKIP (bis §9.5).
	if skipSegments > 0 {
		if skipSegments > len(segs) {
			skipSegments = len(segs)
		}
		fmt.Fprintf(&buf, "#EXT-X-SKIP:SKIPPED-SEGMENTS=%d\n", skipSegments)
		segs = segs[skipSegments:]
	}

	// Spec (draft-pantos-hls-rfc8216bis-22 §9.11): EXT-X-PART tags must be
	// omitted from all but the most recently completed segment.
	lastIdx := len(segs) - 1
	for i, seg := range segs {
		fmt.Fprintf(&buf, "#EXT-X-PROGRAM-DATE-TIME:%s\n",
			seg.WallClock.UTC().Format("2006-01-02T15:04:05.000Z"))
		if i == lastIdx {
			for _, part := range seg.Parts {
				fmt.Fprintf(&buf, "%s", renderPart(part))
			}
		}
		fmt.Fprintf(&buf, "#EXTINF:%.6f,\n", float64(seg.DurationMs)/1000.0)
		fmt.Fprintf(&buf, "%s\n", seg.URI)
	}

	if ended {
		fmt.Fprintf(&buf, "#EXT-X-ENDLIST\n")
	} else {
		// In-progress segment: pending parts + preload hint (no EXTINF yet).
		if len(pending) > 0 {
			fmt.Fprintf(&buf, "#EXT-X-PROGRAM-DATE-TIME:%s\n",
				pendingWall.UTC().Format("2006-01-02T15:04:05.000Z"))
			for _, part := range pending {
				fmt.Fprintf(&buf, "%s", renderPart(part))
			}
		}
		if hintURI != "" {
			fmt.Fprintf(&buf, "#EXT-X-PRELOAD-HINT:TYPE=PART,URI=\"%s\",BYTERANGE-START=%d\n",
				hintURI, hintByteStart)
		}
	}

	for _, r := range reports {
		if r.LastPart >= 0 {
			fmt.Fprintf(&buf, "#EXT-X-RENDITION-REPORT:URI=\"%s\",LAST-MSN=%d,LAST-PART=%d\n",
				r.URI, r.LastMSN, r.LastPart)
		} else {
			fmt.Fprintf(&buf, "#EXT-X-RENDITION-REPORT:URI=\"%s\",LAST-MSN=%d\n",
				r.URI, r.LastMSN)
		}
	}

	return buf.String()
}

// renderPart renders a single EXT-X-PART line.
func renderPart(part LivePartByteRange) string {
	attrs := fmt.Sprintf("DURATION=%.6f,URI=\"%s\",BYTERANGE=\"%d@%d\"",
		float64(part.DurationMs)/1000.0,
		part.URI,
		part.ByteLength,
		part.ByteOffset,
	)
	if part.Independent {
		attrs += ",INDEPENDENT=YES"
	}
	return "#EXT-X-PART:" + attrs + "\n"
}
