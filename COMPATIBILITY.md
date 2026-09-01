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

## Breaking Changes

Pre-1.0 module: interfaces may change without a major version bump. Prefer published tags over `main`/`master` HEAD in production.
