# hlslib

[![CI](https://github.com/jonasevcik/hlslib/actions/workflows/ci.yml/badge.svg)](https://github.com/jonasevcik/hlslib/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jonasevcik/hlslib.svg)](https://pkg.go.dev/github.com/jonasevcik/hlslib)
[![Go Report Card](https://goreportcard.com/badge/github.com/jonasevcik/hlslib)](https://goreportcard.com/report/github.com/jonasevcik/hlslib)

Go library for generating HLS playlists. Covers VOD, standard live, and Low-Latency HLS (LL-HLS), plus bandwidth calculation and structural validation.

## Install

Requires Go 1.24+.

```sh
go get github.com/jonasevcik/hlslib
```

The package name is `hls` (not `hlslib`), so no import alias is needed:

```go
import "github.com/jonasevcik/hlslib"
```

## Types

| Type | Description |
|------|-------------|
| `MediaPlaylist` | VOD media playlist (fMP4, EXT-X-ENDLIST) |
| `MasterPlaylist` | Master playlist with variants and optional audio groups |
| `LiveMediaPlaylist` | Live sliding-window playlist (standard HLS) |
| `LLLiveMediaPlaylist` | Live sliding-window playlist (LL-HLS, EXT-X-PART) |

`LiveMediaPlaylist` and `LLLiveMediaPlaylist` are safe for concurrent use from multiple goroutines. Other types are not.

## Usage

### VOD media playlist

```go
pl := &hls.MediaPlaylist{
    Version:        6,
    TargetDuration: 7,
    PlaylistType:   "VOD",
    MapURI:         "../media/init.mp4",
    EndList:        true,
    Segments: []hls.MediaPlaylistSegment{
        {Duration: 6.006, URI: "../media/seg_000000.m4s"},
        {Duration: 6.006, URI: "../media/seg_000001.m4s"},
        {Duration: 4.004, URI: "../media/seg_000002.m4s"},
    },
}
fmt.Print(pl.Render())
```

### Master playlist

```go
pl := hls.NewMasterPlaylistWithAudio(
    []hls.Variant{
        {Bandwidth: 800_000, AverageBandwidth: 700_000, Codecs: "avc1.640020", Resolution: "1280x720", FrameRate: 25, URI: "hls/720p.m3u8", AudioGroupID: "audio"},
        {Bandwidth: 3_000_000, AverageBandwidth: 2_700_000, Codecs: "avc1.640028", Resolution: "1920x1080", FrameRate: 25, URI: "hls/1080p.m3u8", AudioGroupID: "audio"},
    },
    []hls.AudioRendition{
        {GroupID: "audio", Name: "English", Language: "en", Default: true, AutoSelect: true, Channels: "2", URI: "hls/audio_en.m3u8"},
    },
    true, // EXT-X-INDEPENDENT-SEGMENTS
)
fmt.Print(pl.Render())
```

### Live playlist (standard HLS)

```go
pl := hls.NewLiveMediaPlaylist(7, "init.mp4")

// Add segments as they are produced
pl.Add(hls.LiveSegment{
    TfdtValue:  900000,
    WallClock:  time.Now(),
    DurationMs: 6000,
    SizeBytes:  512_000,
    URI:        "chunk-0-900000.m4s",
})

// Trim the DVR window (e.g. keep 30 seconds)
pl.Trim(30 * time.Second)

fmt.Print(pl.Render())
```

### Live playlist (LL-HLS)

```go
pl := hls.NewLLLiveMediaPlaylist(7, 500, "init.mp4") // 500 ms parts

// Parts arrive as they are written
pl.AddPart(hls.LivePartByteRange{
    URI: "chunk-0-900000.m4s", ByteOffset: 0, ByteLength: 62_000,
    DurationMs: 500, Independent: true,
}, time.Now())

pl.SetPreloadHint("chunk-0-900000.m4s", 62_000)

// Finalize the segment when all parts are written
pl.CommitSegment(900000, segWallClock, 6000, 512_000, "chunk-0-900000.m4s")

pl.Trim(30 * time.Second)

fmt.Print(pl.Render())
```

### Bandwidth calculation

```go
bw := hls.ComputeBandwidth(segments, targetDurationSec, encoderPeakBps)
avgBw := hls.ComputeAverageBandwidth(segments, totalDurationMs)
```

`ComputeBandwidth` uses a sliding window with a minimum length of `targetDuration/2` seconds, matching RFC 8216 §4.3.4.2.

### Validation

```go
v := hls.NewHLSValidator()

if err := v.ValidateMediaPlaylist(pl); err != nil {
    log.Fatal(err)
}
if err := v.ValidateMasterPlaylist(master); err != nil {
    log.Fatal(err)
}
// Verify all renditions share the same TARGETDURATION
if err := v.ValidateCrossRendition(playlists); err != nil {
    log.Fatal(err)
}
```

## License

PolyForm Noncommercial License 1.0.0 — free for personal, educational, and research use. See [LICENSE](LICENSE).
