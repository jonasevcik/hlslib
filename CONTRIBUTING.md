# Contributing

## Prerequisites

- Go 1.24+
- No Docker, no external services — tests are pure Go

## Running tests

```sh
go test ./... -race
```

`go vet ./...` must also pass cleanly.

## Guidelines

- Every exported symbol must have a godoc comment.
- Tag ordering in rendered playlists is normative per RFC 8216. Tests verify exact output — if you change rendering order, update the corresponding tests and cite the RFC section.
- `LiveMediaPlaylist` and `LLLiveMediaPlaylist` use an internal mutex. Every public method must hold the lock for the duration of the call; never return a reference to internal slice state without copying.
- No new runtime dependencies. Testify is the only allowed test dependency.
- No `panic`, `fmt.Print`, `log.*`, or `init()` in library code.

## Pull requests

Open a PR against `main`. All CI checks must pass. For non-trivial changes, include a short description of which RFC section the behaviour is based on.

## License

By contributing you agree that your contributions will be licensed under the [PolyForm Noncommercial License 1.0.0](LICENSE).
