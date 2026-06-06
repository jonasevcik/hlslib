package hls

import (
	"fmt"
	"sort"
	"strings"
)

// MediaPlaylist represents an HLS media playlist (VOD variant).
type MediaPlaylist struct {
	Version        int
	TargetDuration int
	MediaSequence  int
	PlaylistType   string
	MapURI         string
	Segments       []MediaPlaylistSegment
	EndList        bool
}

// MediaPlaylistSegment is one #EXTINF entry in a media playlist.
type MediaPlaylistSegment struct {
	Duration      float64
	URI           string
	Comment       string
	Independent   bool
	DateTimeUTC   string
	ByteRange     *ByteRange
	Discontinuity bool
}

// ByteRange represents an HLS EXT-X-BYTERANGE value. Start nil means offsetless
// (only valid when the previous segment ends in the same resource); Start non-nil
// emits an explicit "@<offset>" which is always safe and required for the first
// segment in a byte-range series.
type ByteRange struct {
	Start  *int64
	Length int64
}

// Render outputs the VOD media playlist in M3U8 format.
func (p *MediaPlaylist) Render() string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "#EXTM3U\n")
	fmt.Fprintf(&buf, "#EXT-X-VERSION:%d\n", p.Version)
	fmt.Fprintf(&buf, "#EXT-X-TARGETDURATION:%d\n", p.TargetDuration)
	fmt.Fprintf(&buf, "#EXT-X-MEDIA-SEQUENCE:%d\n", p.MediaSequence)

	if p.PlaylistType != "" {
		fmt.Fprintf(&buf, "#EXT-X-PLAYLIST-TYPE:%s\n", p.PlaylistType)
	}

	if p.MapURI != "" {
		fmt.Fprintf(&buf, "#EXT-X-MAP:URI=\"%s\"\n", p.MapURI)
	}

	for _, seg := range p.Segments {
		if seg.Discontinuity {
			fmt.Fprintf(&buf, "#EXT-X-DISCONTINUITY\n")
		}
		if seg.DateTimeUTC != "" {
			fmt.Fprintf(&buf, "#EXT-X-PROGRAM-DATE-TIME:%s\n", seg.DateTimeUTC)
		}
		fmt.Fprintf(&buf, "#EXTINF:%.6f,\n", seg.Duration)
		if seg.ByteRange != nil {
			if seg.ByteRange.Start != nil {
				fmt.Fprintf(&buf, "#EXT-X-BYTERANGE:%d@%d\n", seg.ByteRange.Length, *seg.ByteRange.Start)
			} else {
				fmt.Fprintf(&buf, "#EXT-X-BYTERANGE:%d\n", seg.ByteRange.Length)
			}
		}
		fmt.Fprintf(&buf, "%s\n", seg.URI)
	}

	if p.EndList {
		fmt.Fprintf(&buf, "#EXT-X-ENDLIST\n")
	}

	return buf.String()
}

// MasterPlaylist represents an HLS master playlist.
type MasterPlaylist struct {
	Version             int
	Variants            []Variant
	AudioRenditions     []AudioRendition
	IndependentSegments bool
}

// AudioRendition represents one EXT-X-MEDIA:TYPE=AUDIO entry.
type AudioRendition struct {
	GroupID    string
	Name       string
	Language   string
	Default    bool
	AutoSelect bool
	Channels   string
	URI        string
}

// Variant represents one EXT-X-STREAM-INF entry.
type Variant struct {
	Bandwidth        int64
	AverageBandwidth int64
	Codecs           string
	Resolution       string
	FrameRate        float64
	URI              string
	AudioGroupID     string
}

// NewMasterPlaylist creates a master playlist with variants sorted by bandwidth ascending.
func NewMasterPlaylist(variants []Variant, independentSegments bool) *MasterPlaylist {
	sorted := make([]Variant, len(variants))
	copy(sorted, variants)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Bandwidth < sorted[j].Bandwidth
	})
	return &MasterPlaylist{
		Version:             6,
		Variants:            sorted,
		IndependentSegments: independentSegments,
	}
}

// NewMasterPlaylistWithAudio creates a master playlist including HLS audio groups.
func NewMasterPlaylistWithAudio(variants []Variant, audioRenditions []AudioRendition, independentSegments bool) *MasterPlaylist {
	pl := NewMasterPlaylist(variants, independentSegments)
	pl.AudioRenditions = audioRenditions
	if len(audioRenditions) > 0 {
		pl.Version = 7
	}
	return pl
}

// Render outputs the master playlist in M3U8 format.
func (p *MasterPlaylist) Render() string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "#EXTM3U\n")
	fmt.Fprintf(&buf, "#EXT-X-VERSION:%d\n", p.Version)

	if p.IndependentSegments {
		fmt.Fprintf(&buf, "#EXT-X-INDEPENDENT-SEGMENTS\n")
	}

	for _, ar := range p.AudioRenditions {
		defaultVal := "NO"
		if ar.Default {
			defaultVal = "YES"
		}
		autoselect := "NO"
		if ar.AutoSelect {
			autoselect = "YES"
		}
		line := fmt.Sprintf(`#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="%s",NAME="%s"`, ar.GroupID, ar.Name)
		if ar.Language != "" && ar.Language != "und" {
			line += fmt.Sprintf(`,LANGUAGE="%s"`, ar.Language)
		}
		line += fmt.Sprintf(`,DEFAULT=%s,AUTOSELECT=%s`, defaultVal, autoselect)
		if ar.Channels != "" {
			line += fmt.Sprintf(`,CHANNELS="%s"`, ar.Channels)
		}
		line += fmt.Sprintf(`,URI="%s"`, ar.URI)
		fmt.Fprintln(&buf, line)
	}

	for _, v := range p.Variants {
		attrs := fmt.Sprintf("BANDWIDTH=%d,AVERAGE-BANDWIDTH=%d,CODECS=\"%s\"",
			v.Bandwidth, v.AverageBandwidth, v.Codecs)
		if v.Resolution != "" {
			attrs += fmt.Sprintf(",RESOLUTION=%s", v.Resolution)
		}
		if v.FrameRate > 0 {
			attrs += fmt.Sprintf(",FRAME-RATE=%.3f", v.FrameRate)
		}
		if v.AudioGroupID != "" {
			attrs += fmt.Sprintf(",AUDIO=\"%s\"", v.AudioGroupID)
		}
		fmt.Fprintf(&buf, "#EXT-X-STREAM-INF:%s\n%s\n", attrs, v.URI)
	}

	return buf.String()
}
