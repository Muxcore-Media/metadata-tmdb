# Compatibility

## Core Version

| Module Version | Core Version | Status |
|----------------|-------------|--------|
| v0.1.5           | 0.5.8+     | Current |

MVP host stacks pin **core@v0.5.8**. This module declares `minCoreVersion` **0.4.0**.

## Capabilities

- `metadata`
- `metadata.tmdb`
- `settings`

## Contracts

Implements `MetadataProvider` from `github.com/Muxcore-Media/contracts-metadata` (v0.1.0).

### Certification fields

`certification` / `certification_country` are served on `GetMovieDetailsResponse` (tags 28/29) and `GetTVDetailsResponse` (tags 33/34). media-tvshows decodes them through contracts-metadata; media-movies decodes this repo's deprecated `proto/metadatav1` copy. Both copies must keep **identical** numbers, names and types for every message this server returns (`proto/metadatav1/certification_wire_test.go`, `internal/certification_test.go`). Note the copies already differ on `GetTVDetailsResponse` tag 18 (`string in_production` here, reserved in contracts-metadata) and lack tags 30–32 here; do not reuse 30–32 in this copy. Older consumers ignore the new fields.

## Breaking Changes

Pre-1.0 module: interfaces may change without a major version bump. Prefer published tags over `main`/`master` HEAD in production.
