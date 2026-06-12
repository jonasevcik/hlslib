package hls

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiveMediaPlaylistRender(t *testing.T) {
	p := NewLiveMediaPlaylist(7, "init.mp4")
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)

	p.Add(LiveSegment{TfdtValue: 0, WallClock: now, DurationMs: 6000, URI: "chunk-1080p-0.m4s"})
	p.Add(LiveSegment{TfdtValue: 540000, WallClock: now.Add(6 * time.Second), DurationMs: 6000, URI: "chunk-1080p-540000.m4s"})

	output := p.Render()

	assert.True(t, strings.HasPrefix(output, "#EXTM3U\n"))
	assert.Contains(t, output, "#EXT-X-VERSION:6")
	assert.Contains(t, output, "#EXT-X-TARGETDURATION:7")
	assert.Contains(t, output, "#EXT-X-MEDIA-SEQUENCE:0")
	assert.Contains(t, output, `#EXT-X-MAP:URI="init.mp4"`)
	assert.Contains(t, output, "#EXT-X-PROGRAM-DATE-TIME:2026-06-06T12:00:00.000Z")
	assert.Contains(t, output, "#EXT-X-PROGRAM-DATE-TIME:2026-06-06T12:00:06.000Z")
	assert.Contains(t, output, "#EXTINF:6.000000,")
	assert.Contains(t, output, "chunk-1080p-0.m4s")
	assert.Contains(t, output, "chunk-1080p-540000.m4s")
	assert.NotContains(t, output, "#EXT-X-ENDLIST")
	assert.NotContains(t, output, "#EXT-X-PLAYLIST-TYPE")
}

func TestLiveMediaPlaylistMediaSequenceIncrements(t *testing.T) {
	p := NewLiveMediaPlaylist(7, "init.mp4")
	now := time.Now()

	// Add 4 segments spaced 6 seconds apart
	for i := range 4 {
		p.Add(LiveSegment{
			WallClock:  now.Add(time.Duration(i) * 6 * time.Second),
			DurationMs: 6000,
			URI:        "seg.m4s",
		})
	}

	assert.Equal(t, 0, p.MediaSequence())

	// Trim to 12 seconds — the first two segments (t=0 and t=6) should fall out
	// since they're older than now - 12s. We set the cutoff to now - 12s.
	// The segments are at t=now, t=now+6s, t=now+12s, t=now+18s.
	// None are older than now - 12s when they were just added "in the future".
	// Use a past time for segments instead:
	past := now.Add(-30 * time.Second)
	p2 := NewLiveMediaPlaylist(7, "init.mp4")
	for i := range 4 {
		p2.Add(LiveSegment{
			WallClock:  past.Add(time.Duration(i) * 6 * time.Second),
			DurationMs: 6000,
			URI:        "seg.m4s",
		})
	}
	// Segments: t=-30s, t=-24s, t=-18s, t=-12s relative to now
	// Trim to 15s: cutoff = now-15s; segments older than now-15s are evicted
	// t=-30s (-30 < -15) → evict; t=-24s (-24 < -15) → evict; t=-18s (-18 < -15) → evict
	// t=-12s (-12 >= -15) → keep
	p2.Trim(15 * time.Second)
	assert.Equal(t, 3, p2.MediaSequence())
	segs := p2.Segments()
	require.Len(t, segs, 1)
}

func TestLiveMediaPlaylistProgramDateTimeFormat(t *testing.T) {
	p := NewLiveMediaPlaylist(7, "init.mp4")
	ts := time.Date(2026, 6, 6, 15, 30, 45, 500_000_000, time.UTC)
	p.Add(LiveSegment{WallClock: ts, DurationMs: 6000, URI: "s.m4s"})
	output := p.Render()
	assert.Contains(t, output, "#EXT-X-PROGRAM-DATE-TIME:2026-06-06T15:30:45.500Z")
}

func TestLiveMediaPlaylistNoEndList(t *testing.T) {
	p := NewLiveMediaPlaylist(7, "init.mp4")
	p.Add(LiveSegment{WallClock: time.Now(), DurationMs: 6000, URI: "s.m4s"})
	assert.NotContains(t, p.Render(), "#EXT-X-ENDLIST")
}

func TestLiveMediaPlaylistNoPlaylistType(t *testing.T) {
	p := NewLiveMediaPlaylist(7, "init.mp4")
	p.Add(LiveSegment{WallClock: time.Now(), DurationMs: 6000, URI: "s.m4s"})
	assert.NotContains(t, p.Render(), "#EXT-X-PLAYLIST-TYPE")
}

func TestLiveMediaPlaylistSegmentsCopy(t *testing.T) {
	p := NewLiveMediaPlaylist(7, "init.mp4")
	p.Add(LiveSegment{WallClock: time.Now(), DurationMs: 6000, URI: "seg_a.m4s"})
	segs := p.Segments()
	segs[0].URI = "mutated"
	// Original must be unchanged
	assert.Equal(t, "seg_a.m4s", p.Segments()[0].URI)
}

func TestLiveMediaPlaylistEmptyRender(t *testing.T) {
	p := NewLiveMediaPlaylist(7, "init.mp4")
	output := p.Render()
	assert.Contains(t, output, "#EXTM3U")
	assert.Contains(t, output, "#EXT-X-MEDIA-SEQUENCE:0")
	assert.NotContains(t, output, "#EXTINF")
}

func TestLiveMediaPlaylistLLAudio_EmitsLLHeaders(t *testing.T) {
	p := NewLiveMediaPlaylist(7, "../media/audio/init.mp4")
	p.SetLLAudio(&LLAudioConfig{})
	p.Add(LiveSegment{WallClock: time.Now(), DurationMs: 6000, URI: "chunk.m4s"})

	out := p.Render()

	assert.Contains(t, out, "#EXT-X-VERSION:9")
	assert.NotContains(t, out, "#EXT-X-PART-INF")
	assert.Contains(t, out, "#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES,HOLD-BACK=21")
	assert.NotContains(t, out, "PART-HOLD-BACK")
}

func TestLiveMediaPlaylistLLAudio_NoHeadersWithoutConfig(t *testing.T) {
	p := NewLiveMediaPlaylist(7, "init.mp4")
	p.Add(LiveSegment{WallClock: time.Now(), DurationMs: 6000, URI: "chunk.m4s"})

	out := p.Render()

	assert.Contains(t, out, "#EXT-X-VERSION:6")
	assert.NotContains(t, out, "#EXT-X-PART-INF")
	assert.NotContains(t, out, "#EXT-X-SERVER-CONTROL")
}

func TestLiveMediaPlaylistLLAudio_TagOrderBeforeMap(t *testing.T) {
	p := NewLiveMediaPlaylist(7, "init.mp4")
	p.SetLLAudio(&LLAudioConfig{})
	p.Add(LiveSegment{WallClock: time.Now(), DurationMs: 6000, URI: "chunk.m4s"})

	out := p.Render()

	// EXT-X-SERVER-CONTROL must appear before EXT-X-MAP
	serverCtrlIdx := strings.Index(out, "#EXT-X-SERVER-CONTROL")
	mapIdx := strings.Index(out, "#EXT-X-MAP")
	assert.Greater(t, mapIdx, serverCtrlIdx, "EXT-X-SERVER-CONTROL must appear before EXT-X-MAP")
}

func TestLiveMediaPlaylistLLAudio_RenditionReports(t *testing.T) {
	p := NewLiveMediaPlaylist(7, "init.mp4")
	p.SetLLAudio(&LLAudioConfig{})
	p.Add(LiveSegment{WallClock: time.Now(), DurationMs: 6000, URI: "chunk.m4s"})

	reports := []RenditionReport{
		{URI: "video_1080p.m3u8", LastMSN: 5, LastPart: 3},
		{URI: "video_720p.m3u8", LastMSN: 5, LastPart: -1},
	}
	out := p.Render(reports...)

	assert.Contains(t, out, `#EXT-X-RENDITION-REPORT:URI="video_1080p.m3u8",LAST-MSN=5,LAST-PART=3`)
	assert.Contains(t, out, `#EXT-X-RENDITION-REPORT:URI="video_720p.m3u8",LAST-MSN=5`)
	assert.NotContains(t, out, "LAST-PART=-1")
}

func TestLiveMediaPlaylistLLAudio_NoRenditionReportsWhenNil(t *testing.T) {
	p := NewLiveMediaPlaylist(7, "init.mp4")
	p.SetLLAudio(&LLAudioConfig{})
	p.Add(LiveSegment{WallClock: time.Now(), DurationMs: 6000, URI: "chunk.m4s"})

	out := p.Render()
	assert.NotContains(t, out, "#EXT-X-RENDITION-REPORT")
}
