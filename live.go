package hls

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// LiveSegment is one segment in a live DVR window.
type LiveSegment struct {
	TfdtValue  int64     // baseMediaDecodeTime; used as segment filename suffix
	WallClock  time.Time // wall-clock time of first frame; drives EXT-X-PROGRAM-DATE-TIME and eviction
	DurationMs int64
	SizeBytes  int64
	URI        string // relative path from the media playlist file
}

// LiveMediaPlaylist is a live HLS media playlist with a sliding DVR window.
// It is safe for concurrent use.
type LiveMediaPlaylist struct {
	mu             sync.Mutex
	version        int
	targetDuration int
	mapURI         string
	segments       []LiveSegment
	mediaSequence  int // sequence number of the first segment currently in the window
}

// NewLiveMediaPlaylist creates a live media playlist.
// targetDurationSec should be segmentLengthSec + 1 (fixed for the lifetime of the stream).
func NewLiveMediaPlaylist(targetDurationSec int, mapURI string) *LiveMediaPlaylist {
	return &LiveMediaPlaylist{
		version:        6,
		targetDuration: targetDurationSec,
		mapURI:         mapURI,
	}
}

// Add appends a new segment to the window.
func (p *LiveMediaPlaylist) Add(seg LiveSegment) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.segments = append(p.segments, seg)
}

// Trim removes segments whose WallClock is older than dvr from the front of the window.
// Segments are removed from the manifest only; physical deletion is handled by S3 lifecycle rules.
// The EXT-X-MEDIA-SEQUENCE number advances accordingly.
func (p *LiveMediaPlaylist) Trim(dvr time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cutoff := time.Now().Add(-dvr)
	for len(p.segments) > 0 && p.segments[0].WallClock.Before(cutoff) {
		p.segments = p.segments[1:]
		p.mediaSequence++
	}
}

// Segments returns a copy of the current segment window (thread-safe).
func (p *LiveMediaPlaylist) Segments() []LiveSegment {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]LiveSegment, len(p.segments))
	copy(out, p.segments)
	return out
}

// MediaSequence returns the current EXT-X-MEDIA-SEQUENCE value.
func (p *LiveMediaPlaylist) MediaSequence() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mediaSequence
}

// Render produces the M3U8 text for the current window snapshot.
// There is no EXT-X-ENDLIST — the playlist is a live stream.
// EXT-X-PROGRAM-DATE-TIME is emitted before every segment.
func (p *LiveMediaPlaylist) Render() string {
	p.mu.Lock()
	segs := make([]LiveSegment, len(p.segments))
	copy(segs, p.segments)
	seq := p.mediaSequence
	p.mu.Unlock()

	var buf strings.Builder

	fmt.Fprintf(&buf, "#EXTM3U\n")
	fmt.Fprintf(&buf, "#EXT-X-VERSION:%d\n", p.version)
	fmt.Fprintf(&buf, "#EXT-X-TARGETDURATION:%d\n", p.targetDuration)
	fmt.Fprintf(&buf, "#EXT-X-MEDIA-SEQUENCE:%d\n", seq)

	if p.mapURI != "" {
		fmt.Fprintf(&buf, "#EXT-X-MAP:URI=\"%s\"\n", p.mapURI)
	}

	for _, seg := range segs {
		// RFC 8216 §4.4.4.6: emit EXT-X-PROGRAM-DATE-TIME before each segment
		// so clients can compute absolute position in the DVR window.
		fmt.Fprintf(&buf, "#EXT-X-PROGRAM-DATE-TIME:%s\n", seg.WallClock.UTC().Format("2006-01-02T15:04:05.000Z"))
		fmt.Fprintf(&buf, "#EXTINF:%.6f,\n", float64(seg.DurationMs)/1000.0)
		fmt.Fprintf(&buf, "%s\n", seg.URI)
	}

	return buf.String()
}
