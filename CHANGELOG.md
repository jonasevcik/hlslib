# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] — 2024-12-01

### Added

- `MediaPlaylist` — VOD media playlist renderer (EXT-X-ENDLIST, fMP4, EXT-X-MAP, EXT-X-BYTERANGE)
- `MasterPlaylist` — master playlist renderer with `EXT-X-STREAM-INF` variants and optional `EXT-X-MEDIA` audio groups; variants sorted by bandwidth ascending per RFC 8216
- `LiveMediaPlaylist` — thread-safe sliding-window live playlist (standard HLS); supports `Add`, `Trim`, `Render`, `MediaSequence`, `Segments`
- `LLLiveMediaPlaylist` — thread-safe sliding-window live playlist for Low-Latency HLS (RFC 8216bis); supports partial segments (`EXT-X-PART`), preload hints (`EXT-X-PRELOAD-HINT`), `EXT-X-SERVER-CONTROL` with `CAN-BLOCK-RELOAD=YES`, blocking reload via `CurrentMSN`
- `ComputeBandwidth` — peak bandwidth using a sliding window of minimum `targetDuration/2` seconds, matching RFC 8216 §4.3.4.2
- `ComputeAverageBandwidth` — average bandwidth over total content duration
- `HLSValidator` — structural validation for `MediaPlaylist`, `MasterPlaylist`, and cross-rendition `TARGETDURATION` consistency
- `ValidateSampleDurationsForFPS` — validates that fMP4 sample durations are consistent with a declared frame rate
