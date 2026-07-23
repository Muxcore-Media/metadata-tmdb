# Metadata TMDB

TMDB (The Movie Database) metadata provider for movies and TV shows.

## Key Features

- Search movies/TV (title query; optional year/language/page)
- Movie and TV details (title, overview, dates, runtime, genres, companies, etc.)
- Season details with episode list
- Movie collections
- Find by external ID (default source `imdb_id`)
- Alternative titles
- Trending and popular lists
- Image configuration (poster/backdrop sizes, base URLs)
- In-process response cache, request coalescing, and outbound rate limiting
- Runtime settings mesh (`api_key`, `base_url`)
- Configurable API base URL (HTTP client timeout defaults to 15s)

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `TMDB_API_KEY` | `""` | TMDB API key |
| `MUXCORE_CFG_TMDB_API_KEY` | `""` | Alternative TMDB API key |
| `METADATA_GRPC_ADDR` | `:9410` | gRPC listen address |
| `TMDB_BASE_URL` | `https://api.themoviedb.org` | TMDB API base URL |
| `TMDB_CACHE_MAX` | `1024` | Max cached responses (`0` disables) |
| `TMDB_CACHE_TTL_DETAILS` | `6h` | TTL for movie/TV/collection details |
| `TMDB_CACHE_TTL_SEARCH` | `15m` | TTL for search/find |
| `TMDB_CACHE_TTL_LIST` | `1h` | TTL for trending/popular |
| `TMDB_RATE` | `35` | Outbound requests per second (`0` = unlimited) |
| `TMDB_BURST` | `40` | Token-bucket burst size |
| `MUXCORE_INSECURE_DISABLE_TLS` | unset | Set `true` for insecure gRPC to core (dev) |

## Capability

`metadata`, `metadata.tmdb`, `settings` — TMDB metadata provider with runtime settings

## Dependencies

- `github.com/Muxcore-Media/core` — MuxCore SDK
