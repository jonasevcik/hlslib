# hlslib

Go library for generating Apple HLS playlists (VOD, live, LL-HLS), computing bandwidth per RFC 8216, and validating playlist structure.

## Package layout

| File | Contents |
|------|----------|
| `playlist.go` | `MediaPlaylist`, `MasterPlaylist`, `ByteRange` |
| `live.go` | `LiveMediaPlaylist`, `LiveSegment` |
| `live_ll.go` | `LLLiveMediaPlaylist`, `LLLiveSegment`, `LivePartByteRange` |
| `bandwidth.go` | `ComputeBandwidth`, `ComputeAverageBandwidth`, `BandwidthSegment` |
| `validation.go` | `HLSValidator`, `ValidateSampleDurationsForFPS` |
| `doc.go` | Package-level documentation |

## Build & test

```sh
go test ./... -race    # unit tests — no Docker, no fixtures, no external services
go vet ./...           # must pass cleanly
```

There is no Makefile. Tests are self-contained and run in milliseconds.

## Definition of done

A change is complete when:
- `go test ./... -race` passes
- `go vet ./...` is clean
- Every exported symbol (type, function, field, method) has a godoc comment
- No `panic`, `fmt.Print`, `log.*`, or `init()` appears in library code

## Critical constraints

**Tag ordering is normative.** RFC 8216 specifies which tags must precede others. Tests assert exact rendered output — never reorder tags without updating tests and citing the RFC section that permits it.

**Mutex discipline in live playlists.** `LiveMediaPlaylist` and `LLLiveMediaPlaylist` guard all state with a `sync.Mutex`. Every public method must acquire the lock for its entire operation. Never expose a raw slice field from a locked method — copy it first.

**`ComputeBandwidth` follows RFC 8216 §4.3.4.2 exactly.** The sliding window must have a minimum length of `targetDuration/2` seconds. Do not simplify this logic.

**`EXT-X-PLAYLIST-TYPE` only accepts `VOD` and `EVENT`.** `SIMPLE` and any other value are not defined in RFC 8216 §4.3.3.5. The validator rejects them.

**`LLLiveMediaPlaylist.Render(skipSegments int, reports []RenditionReport)` signature.** `skipSegments > 0` produces a Playlist Delta Update (bis §9.5) — pass `0` for a full playlist. `reports` carries per-sibling `RenditionReport` entries for `EXT-X-RENDITION-REPORT` (bis §11.2) — pass `nil` when there are no siblings. Both parameters are always required; callers must not call the old zero-arg form.

**No new dependencies.** `github.com/stretchr/testify` is the only allowed test dependency. The library itself has zero runtime dependencies.

**No panics in library code.** Return errors. Callers should never see a panic from this package.

## RFC references

- HLS: [RFC 8216](https://www.rfc-editor.org/rfc/rfc8216) (standard HLS, VOD, master playlists, bandwidth)
- LL-HLS: [RFC 8216bis / Apple HLS spec](https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices) (EXT-X-PART, EXT-X-SERVER-CONTROL, blocking reload, EXT-X-RENDITION-REPORT §11.2, EXT-X-SKIP delta updates §9.5)
