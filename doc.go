// Package hls provides types and utilities for generating Apple HLS playlists.
//
// It covers four playlist variants:
//   - [MediaPlaylist] — VOD media playlist (EXT-X-ENDLIST, fMP4)
//   - [MasterPlaylist] — master playlist with video variants and optional audio groups
//   - [LiveMediaPlaylist] — sliding-window live playlist (standard HLS)
//   - [LLLiveMediaPlaylist] — sliding-window live playlist with partial segments (LL-HLS, RFC 8216bis)
//
// Bandwidth helpers ([ComputeBandwidth], [ComputeAverageBandwidth]) follow the
// sliding-window algorithm in RFC 8216 §4.3.4.2.
//
// [HLSValidator] checks structural invariants (tag ordering, required attributes,
// cross-rendition TARGETDURATION consistency).
//
// Thread safety: [LiveMediaPlaylist] and [LLLiveMediaPlaylist] are safe for
// concurrent use from multiple goroutines. Other types are not.
//
// The package name is "hls", not "hlslib", so no import alias is needed:
//
//	import "github.com/jonasevcik/hlslib"
//
//	pl := hls.NewLiveMediaPlaylist(6, "init.mp4")
package hls
