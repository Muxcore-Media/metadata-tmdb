# Metadata TMDB

TMDB (The Movie Database) metadata provider for movies and TV shows.

## Key Features

- Search movies/TV (title query; optional year/language/page)
- Movie and TV details (title, overview, dates, runtime, genres, companies, etc.)
- Raw TMDB content certification for one configured country on movie/TV details (see below)
- Season details with episode list
- Movie collections
- Find by external ID (default source `imdb_id`)
- Alternative titles
- Trending and popular lists
- Image configuration (poster/backdrop sizes, base URLs)
- In-process response cache, request coalescing, and outbound rate limiting
- Runtime settings mesh (`api_key`, `base_url`, `certification_country`)
- Configurable API base URL (HTTP client timeout defaults to 15s)

## Configuration

Offline / laptop demo: set `TMDB_FIXTURE=1` or `TMDB_API_KEY=fixture` to serve the built-in Fight Club + Breaking Bad corpus without calling api.themoviedb.org. The MVP stack defaults to fixture mode when no real key is configured.

| Env Var | Default | Description |
|---------|---------|-------------|
| `TMDB_API_KEY` | `""` | TMDB API key (use `fixture` for offline mode) |
| `MUXCORE_CFG_TMDB_API_KEY` | `""` | Alternative TMDB API key |
| `TMDB_FIXTURE` | unset | Set `1` or `true` for offline fixture corpus (no network) |
| `METADATA_GRPC_ADDR` | `:9411` | gRPC listen address |
| `TMDB_BASE_URL` | `https://api.themoviedb.org` | TMDB API base URL |
| `TMDB_CERTIFICATION_COUNTRY` | `US` | ISO 3166-1 alpha-2 country (case-insensitive) whose TMDB certification is returned on movie/TV details. An invalid value logs a warning and **disables** certification (empty fields); another country is never guessed |
| `TMDB_CACHE_MAX` | `1024` | Max cached responses (`0` disables) |
| `TMDB_CACHE_TTL_DETAILS` | `6h` | TTL for movie/TV/collection details |
| `TMDB_CACHE_TTL_SEARCH` | `15m` | TTL for search/find |
| `TMDB_CACHE_TTL_LIST` | `1h` | TTL for trending/popular |
| `TMDB_RATE` | `35` | Outbound requests per second (`0` = unlimited) |
| `TMDB_BURST` | `40` | Token-bucket burst size |
| `MUXCORE_INSECURE_DISABLE_TLS` | unset | Set `true` for insecure gRPC to core (dev) |

## Content certification (ADR-0031 §2)

`GetMovieDetails` and `GetTVDetails` fill `certification` and `certification_country` (contracts-metadata tags: movie 28/29, TV 33/34; the deprecated `proto/metadatav1` copy uses the same tags):

- **Movies** — `append_to_response=release_dates` (no extra request). Among the configured country's entries with a non-empty certification, the best release type wins: theatrical (3), limited theatrical (2), digital (4), physical (5), premiere (1), TV (6), then any other type. Within one type, the earliest `release_date` wins, then TMDB order.
- **TV** — `append_to_response=content_ratings`; the first non-empty rating for the configured country.
- Values are trimmed, control/format characters are removed and anything longer than 16 bytes is dropped. Tokens are **not** mapped or validated here: the media modules own the rating ladder, an operator rating always wins, and an empty or unknown token is *unavailable*, never unrestricted.
- No certification for the country (or a missing/malformed block) returns empty strings, never an error.
- The response cache stores raw TMDB bodies, which carry every country; the country is applied per request after the cache, so changing `certification_country` at runtime never serves another country's value.
- Only one country is supported. Fixture mode (`TMDB_FIXTURE=1`) includes US/GB/DE certifications for Fight Club and Breaking Bad.

## Capability

`metadata`, `metadata.tmdb`, `settings` — TMDB metadata provider with runtime settings

## Dependencies

- `github.com/Muxcore-Media/core` — MuxCore SDK
