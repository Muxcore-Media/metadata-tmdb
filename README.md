# Metadata TMDB

TMDB (The Movie Database) metadata provider for movies and TV shows.

## Key Features

- Fetch movie details (title, overview, release date, runtime, genres, cast)
- Fetch TV show details with season and episode info
- Search by title, IMDb ID, or TMDB ID
- Image configuration (poster/backdrop sizes, base URLs)
- Configurable API base URL and timeout

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `TMDB_API_KEY` | `""` | TMDB API key |
| `MUXCORE_CFG_TMDB_API_KEY` | `""` | Alternative TMDB API key |
| `METADATA_GRPC_ADDR` | `:9410` | gRPC listen address |
| `TMDB_BASE_URL` | `https://api.themoviedb.org` | TMDB API base URL |

## Capability

`metadata.tmdb` — TMDB metadata provider

## Dependencies

- `github.com/Muxcore-Media/core` — MuxCore SDK
