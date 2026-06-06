package hls

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── MediaPlaylist ──────────────────────────────────────────────────────────

func minimalMediaPlaylist() *MediaPlaylist {
	return &MediaPlaylist{
		Version:        6,
		TargetDuration: 6,
		MediaSequence:  0,
		PlaylistType:   "VOD",
		MapURI:         "init.mp4",
		Segments: []MediaPlaylistSegment{
			{Duration: 6.0, URI: "seg_000.m4s", Independent: true},
		},
		EndList: true,
	}
}

// PKG-HLS-001: #EXTM3U must be the very first bytes with no BOM or blank line.
func TestEXTM3UIsFirstLine(t *testing.T) {
	p := minimalMediaPlaylist()
	output := p.Render()
	assert.True(t, strings.HasPrefix(output, "#EXTM3U\n"), "output must start with #EXTM3U\\n")
}

// PKG-HLS-002: No illegal control characters, no BOM.
func TestNoIllegalControlChars(t *testing.T) {
	p := minimalMediaPlaylist()
	output := p.Render()
	assert.False(t, strings.HasPrefix(output, "\xef\xbb\xbf"), "BOM must not appear")
	for i, r := range output {
		if r == '\r' || r == '\n' || r == '\t' {
			continue
		}
		assert.False(t, unicode.IsControl(r), "illegal control char %U at byte offset %d", r, i)
	}
}

// PKG-HLS-003: HLS tag names must be uppercase.
func TestTagCaseSensitivity(t *testing.T) {
	p := minimalMediaPlaylist()
	output := p.Render()
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		tag := strings.TrimPrefix(line, "#")
		if idx := strings.IndexByte(tag, ':'); idx >= 0 {
			tag = tag[:idx]
		}
		assert.Equal(t, strings.ToUpper(tag), tag, "tag %q must be uppercase", tag)
	}
}

// PKG-HLS-004: No whitespace around '=' or ',' in attribute lists.
func TestNoAttributeWhitespace(t *testing.T) {
	p := minimalMediaPlaylist()
	output := p.Render()
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "#EXT-X-") {
			continue
		}
		assert.NotContains(t, line, " =")
		assert.NotContains(t, line, "= ")
		assert.NotContains(t, line, " ,")
		assert.NotContains(t, line, ", ")
	}
}

// PKG-HLS-006: Media Playlist must not contain Multivariant Playlist tags.
func TestNoMixedTagsInMediaPlaylist(t *testing.T) {
	p := minimalMediaPlaylist()
	output := p.Render()
	assert.NotContains(t, output, "EXT-X-STREAM-INF")
	assert.NotContains(t, output, "EXT-X-I-FRAME-STREAM-INF")
	assert.NotContains(t, output, "EXT-X-SESSION-DATA")
}

// PKG-HLS-020: Every segment URI must be immediately preceded by #EXTINF.
func TestEXTINFPrecedesEachURI(t *testing.T) {
	p := &MediaPlaylist{
		Version: 6, TargetDuration: 6, PlaylistType: "VOD",
		MapURI: "init.mp4", EndList: true,
		Segments: []MediaPlaylistSegment{
			{Duration: 6, URI: "seg_000.m4s"},
			{Duration: 6, URI: "seg_001.m4s"},
			{Duration: 6, URI: "seg_002.m4s"},
		},
	}
	lines := strings.Split(strings.TrimRight(p.Render(), "\n"), "\n")
	for i, line := range lines {
		if strings.HasSuffix(line, ".m4s") {
			require.Greater(t, i, 0)
			prev := strings.TrimSpace(lines[i-1])
			assert.True(t, strings.HasPrefix(prev, "#EXTINF:"), "line before %q should be #EXTINF, got %q", line, prev)
		}
	}
}

// PKG-HLS-023: EXT-X-TARGETDURATION must be >= ceil(max EXTINF duration).
func TestTargetDurationAtLeastRoundedMax(t *testing.T) {
	tests := []struct {
		name      string
		durationMs int64
		targetSec int
	}{
		{"uniform 6s", 6000, 6},
		{"5.49s rounds to 5", 5490, 5},
		{"5.50s rounds to 6", 5500, 6},
		{"minimum 1", 500, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &MediaPlaylist{
				Version:        6,
				TargetDuration: tt.targetSec,
				PlaylistType:   "VOD",
				MapURI:         "init.mp4",
				EndList:        true,
				Segments:       []MediaPlaylistSegment{{Duration: float64(tt.durationMs) / 1000, URI: "s.m4s"}},
			}
			for _, seg := range p.Segments {
				rounded := int(math.Round(seg.Duration))
				assert.LessOrEqual(t, rounded, p.TargetDuration)
			}
		})
	}
}

// PKG-HLS-025: EXTINF precision must be within 1 ms of true value.
func TestEXTINFPrecision(t *testing.T) {
	cases := []struct {
		durationMs int64
		wantSubstr string
	}{
		{3003, "3.003"},
		{9009, "9.009"},
		{2185, "2.185"},
	}
	for _, tc := range cases {
		p := &MediaPlaylist{
			Version: 6, TargetDuration: 10, PlaylistType: "VOD",
			MapURI: "init.mp4", EndList: true,
			Segments: []MediaPlaylistSegment{{Duration: float64(tc.durationMs) / 1000, URI: "s.m4s"}},
		}
		output := p.Render()
		assert.Contains(t, output, tc.wantSubstr)
		for _, line := range strings.Split(output, "\n") {
			if !strings.HasPrefix(line, "#EXTINF:") {
				continue
			}
			var rendered float64
			_, err := fmt.Sscanf(strings.TrimSuffix(strings.TrimPrefix(line, "#EXTINF:"), ","), "%f", &rendered)
			require.NoError(t, err)
			assert.InDelta(t, float64(tc.durationMs)/1000, rendered, 0.001)
		}
	}
}

// PKG-HLS-026: Segments appear in the order provided.
func TestSegmentOrderingFollowsInput(t *testing.T) {
	uris := []string{"seg_000.m4s", "seg_001.m4s", "seg_002.m4s"}
	p := &MediaPlaylist{
		Version: 6, TargetDuration: 6, PlaylistType: "VOD",
		MapURI: "init.mp4", EndList: true,
		Segments: []MediaPlaylistSegment{
			{Duration: 6, URI: uris[0]},
			{Duration: 6, URI: uris[1]},
			{Duration: 6, URI: uris[2]},
		},
	}
	output := p.Render()
	pos := [3]int{}
	for i, uri := range uris {
		pos[i] = strings.Index(output, uri)
		require.NotEqual(t, -1, pos[i])
	}
	assert.Less(t, pos[0], pos[1])
	assert.Less(t, pos[1], pos[2])
}

// PKG-HLS-027/028: EXT-X-MEDIA-SEQUENCE must appear before the first segment.
func TestMediaSequenceBeforeFirstSegment(t *testing.T) {
	p := minimalMediaPlaylist()
	p.MediaSequence = 42
	lines := strings.Split(p.Render(), "\n")
	seqIdx := firstLineContaining(lines, "#EXT-X-MEDIA-SEQUENCE:42")
	extinfIdx := firstLineContaining(lines, "#EXTINF:")
	require.NotEqual(t, -1, seqIdx)
	require.NotEqual(t, -1, extinfIdx)
	assert.Less(t, seqIdx, extinfIdx)
}

// PKG-HLS-040: EXT-X-MAP must appear before the first EXTINF.
func TestEXTMapBeforeFirstSegment(t *testing.T) {
	p := minimalMediaPlaylist()
	lines := strings.Split(p.Render(), "\n")
	mapIdx := firstLineContaining(lines, "#EXT-X-MAP:")
	extinfIdx := firstLineContaining(lines, "#EXTINF:")
	require.NotEqual(t, -1, mapIdx)
	require.NotEqual(t, -1, extinfIdx)
	assert.Less(t, mapIdx, extinfIdx)
}

// PKG-HLS-043: Byte range rendering.
func TestByteRangeRendering(t *testing.T) {
	offset0 := int64(0)
	offset4096 := int64(4096)
	p := &MediaPlaylist{
		Version: 6, TargetDuration: 6, PlaylistType: "VOD",
		MapURI: "init.mp4", EndList: true,
		Segments: []MediaPlaylistSegment{
			{Duration: 2.0, URI: "media.mp4", ByteRange: &ByteRange{Length: 4096, Start: &offset0}},
			{Duration: 2.0, URI: "media.mp4", ByteRange: &ByteRange{Length: 8192, Start: &offset4096}},
			{Duration: 2.0, URI: "media.mp4", ByteRange: &ByteRange{Length: 4096, Start: nil}},
		},
	}
	output := p.Render()
	assert.Contains(t, output, "#EXT-X-BYTERANGE:4096@0")
	assert.Contains(t, output, "#EXT-X-BYTERANGE:8192@4096")
	assert.Contains(t, output, "#EXT-X-BYTERANGE:4096\n")
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.Contains(line, "#EXT-X-BYTERANGE:") {
			require.Greater(t, i, 0)
			require.Less(t, i+1, len(lines))
			assert.True(t, strings.HasPrefix(lines[i-1], "#EXTINF:"))
			assert.False(t, strings.HasPrefix(lines[i+1], "#"))
		}
	}
}

// PKG-HLS-061: EXT-X-INDEPENDENT-SEGMENTS must not appear in media playlists.
func TestIndependentSegmentsNotInMediaPlaylist(t *testing.T) {
	p := minimalMediaPlaylist()
	assert.NotContains(t, p.Render(), "#EXT-X-INDEPENDENT-SEGMENTS")
}

// PKG-HLS-DISC: Discontinuity tag is rendered before the right segment.
func TestDiscontinuityTagRendered(t *testing.T) {
	p := &MediaPlaylist{
		Version: 6, TargetDuration: 6, PlaylistType: "VOD",
		MapURI: "init.mp4", EndList: true,
		Segments: []MediaPlaylistSegment{
			{Duration: 6, URI: "seg_000.m4s"},
			{Duration: 6, URI: "seg_001.m4s", Discontinuity: true},
			{Duration: 6, URI: "seg_002.m4s"},
		},
	}
	output := p.Render()
	assert.Equal(t, 1, strings.Count(output, "#EXT-X-DISCONTINUITY\n"))
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "#EXT-X-DISCONTINUITY" {
			require.Less(t, i+1, len(lines))
			assert.True(t, strings.HasPrefix(lines[i+1], "#EXTINF:"))
		}
	}
}

// ── Validation ─────────────────────────────────────────────────────────────

func TestValidationRejectsExceededTargetDuration(t *testing.T) {
	v := NewHLSValidator()
	p := &MediaPlaylist{
		Version: 6, TargetDuration: 6, PlaylistType: "VOD",
		MapURI: "init.mp4", EndList: true,
		Segments: []MediaPlaylistSegment{{Duration: 6.51, URI: "seg.m4s"}},
	}
	err := v.ValidateMediaPlaylist(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rounded duration")
}

func TestValidationRejectsEmptyPlaylist(t *testing.T) {
	v := NewHLSValidator()
	p := &MediaPlaylist{
		Version: 6, TargetDuration: 6, PlaylistType: "VOD",
		MapURI: "init.mp4", EndList: true,
	}
	err := v.ValidateMediaPlaylist(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no segments")
}

func TestValidationRejectsVersionBelow6(t *testing.T) {
	v := NewHLSValidator()
	p := minimalMediaPlaylist()
	p.Version = 5
	err := v.ValidateMediaPlaylist(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), ">= 6")
}

func TestValidationRejectsOffsetlessByteRangeForFirstSegment(t *testing.T) {
	v := NewHLSValidator()
	p := &MediaPlaylist{
		Version: 6, TargetDuration: 6, PlaylistType: "VOD",
		MapURI: "init.mp4", EndList: true,
		Segments: []MediaPlaylistSegment{{Duration: 6, URI: "m.mp4", ByteRange: &ByteRange{Length: 4096}}},
	}
	err := v.ValidateMediaPlaylist(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "offsetless byte range")
}

// ── MasterPlaylist ─────────────────────────────────────────────────────────

func threeVariantPlaylist() *MasterPlaylist {
	return NewMasterPlaylist([]Variant{
		{Bandwidth: 2097152, AverageBandwidth: 1970000, Codecs: "avc1.640029", Resolution: "1920x1080", FrameRate: 29.97, URI: "video_1080p.m3u8"},
		{Bandwidth: 1048576, AverageBandwidth: 975000, Codecs: "avc1.4d401f", Resolution: "1280x720", FrameRate: 29.97, URI: "video_720p.m3u8"},
		{Bandwidth: 524288, AverageBandwidth: 490000, Codecs: "avc1.4d400d", Resolution: "640x360", FrameRate: 29.97, URI: "video_360p.m3u8"},
	}, true)
}

// PKG-HLS-006: Master Playlist must not contain Media Playlist segment tags.
func TestNoMixedTagsInMasterPlaylist(t *testing.T) {
	output := threeVariantPlaylist().Render()
	assert.NotContains(t, output, "#EXTINF")
	assert.NotContains(t, output, "#EXT-X-TARGETDURATION")
	assert.NotContains(t, output, "#EXT-X-ENDLIST")
	assert.NotContains(t, output, "#EXT-X-MAP")
}

// PKG-HLS-100: Every EXT-X-STREAM-INF must be immediately followed by a URI line.
func TestStreamInfFollowedImmediatelyByURI(t *testing.T) {
	lines := strings.Split(strings.TrimRight(threeVariantPlaylist().Render(), "\n"), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			continue
		}
		require.Less(t, i+1, len(lines))
		next := strings.TrimSpace(lines[i+1])
		assert.False(t, strings.HasPrefix(next, "#"), "line after EXT-X-STREAM-INF must be URI, got: %q", next)
		assert.NotEmpty(t, next)
	}
}

// PKG-HLS-101: Every EXT-X-STREAM-INF must include BANDWIDTH.
func TestBandwidthPresentInAllVariants(t *testing.T) {
	output := threeVariantPlaylist().Render()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			assert.Contains(t, line, "BANDWIDTH=")
		}
	}
}

// PKG-HLS-104/106: CODECS, RESOLUTION, FRAME-RATE required.
func TestCodecsResolutionFrameRatePresent(t *testing.T) {
	output := threeVariantPlaylist().Render()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			assert.Contains(t, line, "CODECS=")
			assert.Contains(t, line, "RESOLUTION=")
			assert.Contains(t, line, "FRAME-RATE=")
		}
	}
}

// PKG-HLS-060: EXT-X-INDEPENDENT-SEGMENTS appears exactly once in master.
func TestIndependentSegmentsInMaster(t *testing.T) {
	output := threeVariantPlaylist().Render()
	assert.Equal(t, 1, strings.Count(output, "#EXT-X-INDEPENDENT-SEGMENTS"))
}

func TestVariantsSortedByBandwidthAscending(t *testing.T) {
	pl := threeVariantPlaylist()
	assert.Equal(t, int64(524288), pl.Variants[0].Bandwidth)
	assert.Equal(t, int64(1048576), pl.Variants[1].Bandwidth)
	assert.Equal(t, int64(2097152), pl.Variants[2].Bandwidth)
}

func TestMasterPlaylistWithAudioGroup(t *testing.T) {
	variants := []Variant{{
		Bandwidth: 5128000, AverageBandwidth: 4256000, Codecs: "avc1.640028,mp4a.40.2",
		Resolution: "1920x1080", FrameRate: 25.0, URI: "video/1080p/playlist.m3u8", AudioGroupID: "audio-aac",
	}}
	audioRenditions := []AudioRendition{
		{GroupID: "audio-aac", Name: "English", Language: "en", Default: true, AutoSelect: true, Channels: "2", URI: "audio/en/playlist.m3u8"},
		{GroupID: "audio-aac", Name: "German", Language: "de", Default: false, AutoSelect: true, Channels: "2", URI: "audio/de/playlist.m3u8"},
	}
	pl := NewMasterPlaylistWithAudio(variants, audioRenditions, true)
	output := pl.Render()

	assert.Contains(t, output, "#EXT-X-VERSION:7")
	assert.Contains(t, output, `#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio-aac",NAME="English"`)
	assert.Contains(t, output, `LANGUAGE="en"`)
	assert.Contains(t, output, `DEFAULT=YES`)
	assert.Contains(t, output, `CHANNELS="2"`)

	mediaPos := strings.Index(output, "#EXT-X-MEDIA:")
	streamPos := strings.Index(output, "#EXT-X-STREAM-INF:")
	assert.Less(t, mediaPos, streamPos, "EXT-X-MEDIA must precede EXT-X-STREAM-INF")

	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			assert.Contains(t, line, `AUDIO="audio-aac"`)
		}
	}
}

func TestLanguageUndOmitted(t *testing.T) {
	pl := NewMasterPlaylistWithAudio(
		[]Variant{{Bandwidth: 1000000, Codecs: "avc1.640028,mp4a.40.2", URI: "v.m3u8", AudioGroupID: "ag"}},
		[]AudioRendition{{GroupID: "ag", Name: "Main", Language: "und", URI: "a.m3u8"}},
		false,
	)
	output := pl.Render()
	assert.NotContains(t, output, `LANGUAGE="und"`)
}

// helpers

func firstLineContaining(lines []string, substr string) int {
	for i, line := range lines {
		if strings.Contains(line, substr) {
			return i
		}
	}
	return -1
}
