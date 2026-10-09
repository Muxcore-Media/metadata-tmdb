# Changelog

## [0.1.10] - 2026-10-09

### Added
- `GetMovieDetails` / `GetTVDetails` return the raw TMDB certification for one configured country in `certification` + `certification_country` (contracts-metadata tags movie 28/29, TV 33/34), fetched with `append_to_response=release_dates` / `content_ratings` in the same request (ADR-0031 §2, roadmap T-M4-01 S4b). Deterministic movie release-type rule; empty when none, never an error; values trimmed, control characters removed, over 16 bytes dropped; no ladder mapping.
- `TMDB_CERTIFICATION_COUNTRY` (default `US`) and runtime setting `certification_country`. Invalid startup value disables certification with a warning; invalid runtime updates are rejected.
- Deprecated `proto/metadatav1` copy carries the same fields and tags; `make proto` regenerates it with the pinned generators.
- Fixture corpus includes US/GB/DE certifications for Fight Club and Breaking Bad.

### Changed
- Requires contracts-metadata with the certification fields (S4a).

## [0.1.9] - 2026-10-05


### Security
- gRPC server and peer dials use mesh TLS (meshtls, sdk/go/module v0.6.5) unless the dev insecure flag is set (ADR-0016/0017).


## [0.1.8] - 2026-10-05

### Changed
- Built on core v0.6.14 / sdk/go/module v0.6.4: unregisters on shutdown and re-registers after core restarts (ADR-0022).

## [0.1.7] - 2026-10-05


### Changed
- Reported version comes from muxcore.json (ADR-0021); built on core v0.6.12 / sdk/go/module v0.6.3 (mesh enrollment, ADR-0017).

## [0.1.6] - 2026-10-05

### Changed
- CI runs on GitHub-hosted runners from the umbrella template; retired-origin workflows removed.
- Dependencies resolve from published GitHub tags (no filesystem `replace`); requires core v0.6.0.

## [0.1.5] — 2026-08-10

### Fixed
- Sync Info()/muxcore.json version to **0.1.5**.


## [0.1.4] — 2026-08-10

### Fixed
- Module `Info().Version` aligned to **0.1.4** (was 0.1.1).

## v0.1.0 (2026-08-09)

- TMDB search/details/seasons/collections/trending
- In-process cache, coalescing, rate limiting
- Runtime settings mesh for API key / base URL
