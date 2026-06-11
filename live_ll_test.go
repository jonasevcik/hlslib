package hls

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLLPlaylist() *LLLiveMediaPlaylist {
	return NewLLLiveMediaPlaylist(7, 333, "../media/1080p/init.mp4")
}

func TestLLLiveMediaPlaylistRender_Empty(t *testing.T) {
	p := newLLPlaylist()
	out := p.Render(nil)

	assert.Contains(t, out, "#EXT-X-VERSION:9")
	assert.Contains(t, out, "#EXT-X-TARGETDURATION:7")
	assert.Contains(t, out, "#EXT-X-MEDIA-SEQUENCE:0")
	assert.Contains(t, out, "#EXT-X-PART-INF:PART-TARGET=0.333000")
	assert.Contains(t, out, "CAN-BLOCK-RELOAD=YES")
	assert.Contains(t, out, "PART-HOLD-BACK=1.000000")
	assert.Contains(t, out, "HOLD-BACK=21")
	assert.Contains(t, out, `#EXT-X-MAP:URI="../media/1080p/init.mp4"`)
	assert.NotContains(t, out, "#EXTINF")
	assert.NotContains(t, out, "#EXT-X-PART:")
	assert.NotContains(t, out, "#EXT-X-ENDLIST")
}

func TestLLLiveMediaPlaylistRender_PartInProgressOnly(t *testing.T) {
	p := newLLPlaylist()
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)

	p.AddPart(LivePartByteRange{
		URI: "chunk-1080p-0.m4s", ByteOffset: 0, ByteLength: 200000,
		DurationMs: 333, Independent: true,
	}, now)
	p.SetPreloadHint("chunk-1080p-0.m4s", 200000)

	out := p.Render(nil)

	assert.Contains(t, out, "#EXT-X-PROGRAM-DATE-TIME:2026-06-07T10:00:00.000Z")
	assert.Contains(t, out, `#EXT-X-PART:DURATION=0.333000,URI="chunk-1080p-0.m4s",BYTERANGE="200000@0",INDEPENDENT=YES`)
	assert.Contains(t, out, `#EXT-X-PRELOAD-HINT:TYPE=PART,URI="chunk-1080p-0.m4s",BYTERANGE-START=200000`)
	assert.NotContains(t, out, "#EXTINF")
}

func TestLLLiveMediaPlaylistRender_CompletedSegmentWithParts(t *testing.T) {
	p := newLLPlaylist()
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)

	// Add 3 parts then commit the segment
	p.AddPart(LivePartByteRange{URI: "chunk-1080p-0.m4s", ByteOffset: 0, ByteLength: 200000, DurationMs: 333, Independent: true}, now)
	p.AddPart(LivePartByteRange{URI: "chunk-1080p-0.m4s", ByteOffset: 200000, ByteLength: 215000, DurationMs: 333, Independent: false}, now)
	p.AddPart(LivePartByteRange{URI: "chunk-1080p-0.m4s", ByteOffset: 415000, ByteLength: 205000, DurationMs: 334, Independent: false}, now)
	p.CommitSegment(0, now, 1000, 620000, "chunk-1080p-0.m4s")

	out := p.Render(nil)

	// Parts appear before EXTINF
	partIdx := strings.Index(out, "#EXT-X-PART:")
	extinfIdx := strings.Index(out, "#EXTINF:")
	require.True(t, partIdx < extinfIdx, "EXT-X-PART must appear before #EXTINF")

	assert.Contains(t, out, `#EXT-X-PART:DURATION=0.333000,URI="chunk-1080p-0.m4s",BYTERANGE="200000@0",INDEPENDENT=YES`)
	assert.Contains(t, out, `#EXT-X-PART:DURATION=0.333000,URI="chunk-1080p-0.m4s",BYTERANGE="215000@200000"`)
	assert.Contains(t, out, `#EXT-X-PART:DURATION=0.334000,URI="chunk-1080p-0.m4s",BYTERANGE="205000@415000"`)
	// Only the first part carries INDEPENDENT=YES; the other two must not.
	assert.NotContains(t, out, `BYTERANGE="215000@200000",INDEPENDENT=YES`)
	assert.NotContains(t, out, `BYTERANGE="205000@415000",INDEPENDENT=YES`)
	assert.Contains(t, out, "#EXTINF:1.000000,")
	assert.Contains(t, out, "chunk-1080p-0.m4s\n")
	// No pending parts → no preload hint
	assert.NotContains(t, out, "#EXT-X-PRELOAD-HINT")
}

func TestLLLiveMediaPlaylistRender_InProgressAfterCompleted(t *testing.T) {
	p := newLLPlaylist()
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)

	// Complete one segment
	p.AddPart(LivePartByteRange{URI: "chunk-1080p-0.m4s", ByteOffset: 0, ByteLength: 200000, DurationMs: 333, Independent: true}, now)
	p.CommitSegment(0, now, 6000, 200000, "chunk-1080p-0.m4s")

	// Begin next segment
	next := now.Add(6 * time.Second)
	p.AddPart(LivePartByteRange{URI: "chunk-1080p-540000.m4s", ByteOffset: 0, ByteLength: 202000, DurationMs: 333, Independent: true}, next)
	p.SetPreloadHint("chunk-1080p-540000.m4s", 202000)

	out := p.Render(nil)

	// First segment: has EXTINF
	assert.Contains(t, out, "#EXTINF:6.000000,")
	assert.Contains(t, out, "chunk-1080p-0.m4s")

	// In-progress: EXT-X-PART without EXTINF, with preload hint
	assert.Contains(t, out, "#EXT-X-PROGRAM-DATE-TIME:2026-06-07T10:00:06.000Z")
	assert.Contains(t, out, `#EXT-X-PART:DURATION=0.333000,URI="chunk-1080p-540000.m4s",BYTERANGE="202000@0",INDEPENDENT=YES`)
	assert.Contains(t, out, `#EXT-X-PRELOAD-HINT:TYPE=PART,URI="chunk-1080p-540000.m4s",BYTERANGE-START=202000`)

	// In-progress EXTINF must not appear
	assert.Equal(t, 1, strings.Count(out, "#EXTINF:"))
}

func TestLLLiveMediaPlaylistRender_INDEPENDENT_YES(t *testing.T) {
	p := newLLPlaylist()
	now := time.Now()
	p.AddPart(LivePartByteRange{URI: "seg.m4s", ByteOffset: 0, ByteLength: 100, DurationMs: 333, Independent: true}, now)
	p.AddPart(LivePartByteRange{URI: "seg.m4s", ByteOffset: 100, ByteLength: 100, DurationMs: 333, Independent: false}, now)
	out := p.Render(nil)

	assert.Contains(t, out, "INDEPENDENT=YES")
	// second part must NOT have INDEPENDENT=YES
	parts := strings.Split(out, "\n")
	var partLines []string
	for _, l := range parts {
		if strings.HasPrefix(l, "#EXT-X-PART:") {
			partLines = append(partLines, l)
		}
	}
	require.Len(t, partLines, 2)
	assert.Contains(t, partLines[0], "INDEPENDENT=YES")
	assert.NotContains(t, partLines[1], "INDEPENDENT")
}

func TestLLLiveMediaPlaylistMediaSequence(t *testing.T) {
	p := newLLPlaylist()
	past := time.Now().Add(-30 * time.Second)
	for i := range 4 {
		wc := past.Add(time.Duration(i) * 6 * time.Second)
		p.CommitSegment(int64(i*540000), wc, 6000, 100, "seg.m4s")
	}
	// Segments at -30s, -24s, -18s, -12s. Trim 15s: cutoff = now-15s.
	// -30 < -15 → evict; -24 < -15 → evict; -18 < -15 → evict; -12 >= -15 → keep
	p.Trim(15 * time.Second)
	assert.Equal(t, 3, p.MediaSequence())
	assert.Len(t, p.Segments(), 1)
}

func TestLLLiveMediaPlaylistRender_EarlierSegmentsHaveNoParts(t *testing.T) {
	p := newLLPlaylist()
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)

	p.AddPart(LivePartByteRange{URI: "seg0.m4s", ByteOffset: 0, ByteLength: 100, DurationMs: 333, Independent: true}, now)
	p.CommitSegment(0, now, 1000, 100, "seg0.m4s")

	next := now.Add(time.Second)
	p.AddPart(LivePartByteRange{URI: "seg1.m4s", ByteOffset: 0, ByteLength: 110, DurationMs: 333, Independent: true}, next)
	p.CommitSegment(1000, next, 1000, 110, "seg1.m4s")

	out := p.Render(nil)

	// Both segments must be present
	assert.Equal(t, 2, strings.Count(out, "#EXTINF:"))
	// Only the last segment (seg1) retains its EXT-X-PART tag
	assert.Equal(t, 1, strings.Count(out, "#EXT-X-PART:"))
	assert.Contains(t, out, `URI="seg1.m4s"`)
	assert.NotContains(t, out, `URI="seg0.m4s",`)
}

func TestLLLiveMediaPlaylistServerControlValues(t *testing.T) {
	// partTargetMs=500 → PART-HOLD-BACK = 3 * 0.5 + 0.001 = 1.501
	p := NewLLLiveMediaPlaylist(7, 500, "")
	out := p.Render(nil)
	assert.Contains(t, out, "PART-HOLD-BACK=1.501000")
	assert.Contains(t, out, "HOLD-BACK=21")
}

func TestLLLiveMediaPlaylistPartTargetFormatting(t *testing.T) {
	p := NewLLLiveMediaPlaylist(7, 200, "")
	out := p.Render(nil)
	// 200ms = 0.200000
	assert.Contains(t, out, "PART-TARGET=0.200000")
}

func TestLLLiveMediaPlaylistNoEndList(t *testing.T) {
	p := newLLPlaylist()
	p.CommitSegment(0, time.Now(), 6000, 100, "seg.m4s")
	assert.NotContains(t, p.Render(nil), "#EXT-X-ENDLIST")
}

func TestLLLiveMediaPlaylistCommitClearsPendingState(t *testing.T) {
	p := newLLPlaylist()
	now := time.Now()
	p.AddPart(LivePartByteRange{URI: "seg.m4s", ByteOffset: 0, ByteLength: 100, DurationMs: 333}, now)
	p.SetPreloadHint("seg.m4s", 100)
	p.CommitSegment(0, now, 6000, 100, "seg.m4s")

	out := p.Render(nil)
	// After commit, no pending parts and no preload hint
	assert.Equal(t, 1, strings.Count(out, "#EXT-X-PART:"))
	assert.NotContains(t, out, "#EXT-X-PRELOAD-HINT")
}

func TestLLLiveMediaPlaylistRender_RenditionReports_NoReports(t *testing.T) {
	p := newLLPlaylist()
	p.CommitSegment(0, time.Now(), 6000, 100, "seg.m4s")
	out := p.Render(nil)
	assert.NotContains(t, out, "#EXT-X-RENDITION-REPORT")
}

func TestLLLiveMediaPlaylistRender_RenditionReports_NonLL(t *testing.T) {
	// Sibling is a standard live playlist (audio) — no LAST-PART.
	p := newLLPlaylist()
	p.CommitSegment(0, time.Now(), 6000, 100, "seg.m4s")
	reports := []RenditionReport{
		{URI: "audio_en.m3u8", LastMSN: 5, LastPart: -1},
	}
	out := p.Render(reports)
	assert.Contains(t, out, `#EXT-X-RENDITION-REPORT:URI="audio_en.m3u8",LAST-MSN=5`)
	assert.NotContains(t, out, "LAST-PART")
}

func TestLLLiveMediaPlaylistRender_RenditionReports_LLSibling(t *testing.T) {
	// Sibling is an LL-HLS playlist — includes LAST-PART.
	p := newLLPlaylist()
	p.CommitSegment(0, time.Now(), 6000, 100, "seg.m4s")
	reports := []RenditionReport{
		{URI: "video_720p.m3u8", LastMSN: 7, LastPart: 2},
	}
	out := p.Render(reports)
	assert.Contains(t, out, `#EXT-X-RENDITION-REPORT:URI="video_720p.m3u8",LAST-MSN=7,LAST-PART=2`)
}

func TestLLLiveMediaPlaylistRender_RenditionReports_MultipleAndOrdering(t *testing.T) {
	// Multiple reports appear after EXT-X-PRELOAD-HINT.
	p := newLLPlaylist()
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	p.AddPart(LivePartByteRange{URI: "seg.m4s", ByteOffset: 0, ByteLength: 100, DurationMs: 333, Independent: true}, now)
	p.SetPreloadHint("seg.m4s", 100)
	reports := []RenditionReport{
		{URI: "video_720p.m3u8", LastMSN: 3, LastPart: 1},
		{URI: "audio_en.m3u8", LastMSN: 3, LastPart: -1},
	}
	out := p.Render(reports)

	hintIdx := strings.Index(out, "#EXT-X-PRELOAD-HINT")
	report1Idx := strings.Index(out, `URI="video_720p.m3u8"`)
	report2Idx := strings.Index(out, `URI="audio_en.m3u8"`)
	require.True(t, hintIdx >= 0)
	assert.True(t, report1Idx > hintIdx, "rendition reports must appear after EXT-X-PRELOAD-HINT")
	assert.True(t, report2Idx > hintIdx, "rendition reports must appear after EXT-X-PRELOAD-HINT")
	assert.True(t, report1Idx < report2Idx, "reports appear in provided order")
}
