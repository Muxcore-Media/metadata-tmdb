package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	metadatav1 "github.com/Muxcore-Media/contracts-metadata/muxcore/metadata/v1"
)

func newTestServer(t *testing.T) (*httptest.Server, *Module) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"images": map[string]any{
					"base_url":        "http://image.tmdb.org/t/p/",
					"secure_base_url": "https://image.tmdb.org/t/p/",
					"poster_sizes":    []string{"w92", "w154", "w185", "w342", "w500", "w780", "original"},
					"backdrop_sizes":  []string{"w300", "w780", "w1280", "original"},
					"logo_sizes":      []string{"w45", "w92", "w154", "w185", "w300", "w500", "original"},
					"profile_sizes":   []string{"w45", "w185", "h632", "original"},
					"still_sizes":     []string{"w92", "w185", "w300", "original"},
				},
			})
		case "/3/search/movie":
			if r.URL.Query().Get("primary_release_year") == "1999" || r.URL.Query().Get("primary_release_year") == "" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"page": 1, "total_results": 1, "total_pages": 1,
					"results": []map[string]any{
						{
							"id": 550, "title": "Fight Club",
							"overview":     "A ticking-clock thriller.",
							"poster_path":  "/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg",
							"release_date": "1999-10-15",
							"vote_average": 8.4, "vote_count": 25000,
							"media_type": "movie",
							"genre_ids":  []int32{18, 53},
						},
					},
				})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"page": 1, "total_results": 0, "total_pages": 0, "results": []map[string]any{},
				})
			}
		case "/3/search/tv":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"page": 1, "total_results": 1, "total_pages": 1,
				"results": []map[string]any{
					{
						"id": 1396, "name": "Breaking Bad",
						"overview":       "A high school chemistry teacher.",
						"poster_path":    "/ggFHVNu6YYI5L9W6QN5CvfWgmd.jpg",
						"first_air_date": "2008-01-20",
						"vote_average":   8.9, "vote_count": 10000,
						"media_type": "tv",
					},
				},
			})
		case "/3/movie/550/alternative_titles":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 550,
				"titles": []map[string]any{
					{"iso_3166_1": "US", "title": "Fight Club", "type": ""},
					{"iso_3166_1": "DE", "title": "Fight Club", "type": ""},
					{"iso_3166_1": "FR", "title": "Fight Club: Le Club de la Combat", "type": ""},
				},
			})
		case "/3/tv/1396/alternative_titles":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 1396,
				"results": []map[string]any{
					{"iso_3166_1": "US", "title": "Breaking Bad", "type": ""},
					{"iso_3166_1": "ES", "title": "Breaking Bad: Metástasis", "type": ""},
				},
			})
		case "/3/movie/550":
			payload := map[string]any{
				"id": 550, "title": "Fight Club",
				"original_title": "Fight Club",
				"overview":       "A ticking-clock thriller.",
				"tagline":        "Mischief. Mayhem. Soap.",
				"poster_path":    "/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg",
				"release_date":   "1999-10-15",
				"runtime":        139,
				"vote_average":   8.4, "vote_count": 25000,
				"status":               "Released",
				"budget":               63000000,
				"revenue":              100853753,
				"imdb_id":              "tt0137523",
				"genres":               []map[string]any{{"id": 18, "name": "Drama"}, {"id": 53, "name": "Thriller"}},
				"production_companies": []map[string]any{{"id": 508, "name": "20th Century Fox", "origin_country": "US"}},
				"spoken_languages":     []map[string]any{{"iso_639_1": "en", "name": "English"}},
			}
			if strings.Contains(r.URL.Query().Get("append_to_response"), "videos") {
				payload["videos"] = map[string]any{
					"results": []map[string]any{
						{"id": "v1", "key": "SUXWAEX2jlg", "name": "Trailer", "site": "YouTube", "type": "Trailer"},
					},
				}
			}
			if strings.Contains(r.URL.Query().Get("append_to_response"), "credits") {
				payload["credits"] = map[string]any{
					"cast": []map[string]any{
						{"id": 287, "name": "Brad Pitt", "character": "Tyler Durden", "profile_path": "/bp.jpg", "order": 1},
					},
					"crew": []map[string]any{
						{"id": 7467, "name": "David Fincher", "job": "Director", "department": "Directing", "profile_path": "/df.jpg"},
					},
				}
			}
			_ = json.NewEncoder(w).Encode(payload)
		case "/3/collection/10":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 10, "name": "Star Wars Collection", "overview": "A long time ago...",
				"poster_path": "/c.jpg", "backdrop_path": "/b.jpg",
				"parts": []map[string]any{
					{"id": 11, "title": "A New Hope", "release_date": "1977-05-25", "media_type": "movie", "vote_average": 8.2},
				},
			})
		case "/3/tv/1396":
			payload := map[string]any{
				"id": 1396, "name": "Breaking Bad",
				"original_name":      "Breaking Bad",
				"overview":           "A high school chemistry teacher.",
				"first_air_date":     "2008-01-20",
				"last_air_date":      "2013-09-29",
				"number_of_seasons":  5,
				"number_of_episodes": 62,
				"vote_average":       8.9, "vote_count": 10000,
				"status": "Ended",
				"genres": []map[string]any{{"id": 18, "name": "Drama"}, {"id": 80, "name": "Crime"}},
				"seasons": []map[string]any{
					{"id": 1, "name": "Season 1", "season_number": 1, "episode_count": 7, "air_date": "2008-01-20"},
				},
			}
			if strings.Contains(r.URL.Query().Get("append_to_response"), "videos") {
				payload["videos"] = map[string]any{
					"results": []map[string]any{
						{"id": "v2", "key": "HhesaQXLuRY", "name": "Trailer", "site": "YouTube", "type": "Trailer"},
					},
				}
			}
			_ = json.NewEncoder(w).Encode(payload)
		case "/3/tv/1396/season/1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 3572, "name": "Season 1", "overview": "Season one.",
				"poster_path": "/poster.jpg", "air_date": "2008-01-20",
				"season_number": 1, "vote_average": 8.2,
				"episodes": []map[string]any{
					{
						"id": 62085, "name": "Pilot", "overview": "Walter White.",
						"air_date": "2008-01-20", "episode_number": 1, "season_number": 1,
						"still_path": "/still.jpg", "runtime": 58,
						"vote_average": 8.0, "vote_count": 100, "production_code": "101",
					},
					{
						"id": 62086, "name": "Cat's in the Bag...", "overview": "Cleanup.",
						"air_date": "2008-01-27", "episode_number": 2, "season_number": 1,
						"still_path": "/still2.jpg", "runtime": 48,
						"vote_average": 7.9, "vote_count": 90, "production_code": "102",
					},
				},
			})
		case "/3/find/tt0137523":
			if r.URL.Query().Get("external_source") != "imdb_id" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"movie_results": []map[string]any{
					{
						"id": 550, "title": "Fight Club",
						"overview":     "A ticking-clock thriller.",
						"poster_path":  "/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg",
						"release_date": "1999-10-15",
						"vote_average": 8.4, "vote_count": 25000,
						"media_type": "movie",
					},
				},
				"tv_results": []map[string]any{},
			})
		case "/3/find/tt0903747":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"movie_results": []map[string]any{},
				"tv_results": []map[string]any{
					{
						"id": 1396, "name": "Breaking Bad",
						"overview":       "A high school chemistry teacher.",
						"poster_path":    "/ggFHVNu6YYI5L9W6QN5CvfWgmd.jpg",
						"first_air_date": "2008-01-20",
						"vote_average":   8.9, "vote_count": 10000,
						"media_type": "tv",
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"status_message": "Not found"})
		}
	}))

	m := NewModule(Config{
		GRPCAddr: ":0",
		APIKey:   "test-api-key",
		BaseURL:  srv.URL,
	})
	ctx := context.Background()
	if err := m.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = m.Stop(ctx) })

	return srv, m
}

func TestModuleInfo(t *testing.T) {
	m := NewModule(Config{})
	info := m.Info()
	if info.ID == "" {
		t.Error("module ID must not be empty")
	}
	if info.Version == "" {
		t.Error("module version must not be empty")
	}
	if len(info.Roles) == 0 || info.Roles[0] != "metadata" {
		t.Errorf("expected role metadata, got %v", info.Roles)
	}
	if len(info.Capabilities) == 0 || info.Capabilities[0] != "metadata" {
		t.Errorf("expected capability metadata, got %v", info.Capabilities)
	}
}

func TestSearchMovie(t *testing.T) {
	srv, m := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	resp, err := m.Search(ctx, &metadatav1.SearchRequest{
		Query: "fight club",
		Type:  metadatav1.MediaType_MEDIA_TYPE_MOVIE,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].Title != "Fight Club" {
		t.Errorf("expected 'Fight Club', got %s", resp.Results[0].Title)
	}
	if resp.Results[0].Id != 550 {
		t.Errorf("expected id 550, got %d", resp.Results[0].Id)
	}
}

func TestSearchTV(t *testing.T) {
	srv, m := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	resp, err := m.Search(ctx, &metadatav1.SearchRequest{
		Query: "breaking bad",
		Type:  metadatav1.MediaType_MEDIA_TYPE_TV,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].Name != "Breaking Bad" {
		t.Errorf("expected 'Breaking Bad', got %s", resp.Results[0].Name)
	}
}

func TestSearchMovieWithYear(t *testing.T) {
	srv, m := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	resp, err := m.Search(ctx, &metadatav1.SearchRequest{
		Query: "fight club",
		Type:  metadatav1.MediaType_MEDIA_TYPE_MOVIE,
		Year:  1999,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}

	empty, err := m.Search(ctx, &metadatav1.SearchRequest{
		Query: "fight club",
		Type:  metadatav1.MediaType_MEDIA_TYPE_MOVIE,
		Year:  2020,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Results) != 0 {
		t.Fatalf("expected 0 results for wrong year, got %d", len(empty.Results))
	}
}

func TestGetAlternativeTitlesMovie(t *testing.T) {
	srv, m := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	resp, err := m.GetAlternativeTitles(ctx, &metadatav1.GetAlternativeTitlesRequest{
		Id:   550,
		Type: metadatav1.MediaType_MEDIA_TYPE_MOVIE,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Id != 550 {
		t.Errorf("tmdb_id: %d", resp.Id)
	}
	if len(resp.Titles) < 2 {
		t.Fatalf("expected deduped alts, got %d", len(resp.Titles))
	}
	found := false
	for _, tit := range resp.Titles {
		if strings.Contains(tit.Title, "Combat") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected French alternate title")
	}
}

func TestGetAlternativeTitlesTV(t *testing.T) {
	srv, m := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	resp, err := m.GetAlternativeTitles(ctx, &metadatav1.GetAlternativeTitlesRequest{
		Id:   1396,
		Type: metadatav1.MediaType_MEDIA_TYPE_TV,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Titles) != 2 {
		t.Fatalf("expected 2 titles, got %d", len(resp.Titles))
	}
}

func TestGetMovieDetails(t *testing.T) {
	srv, m := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	if _, err := m.GetConfiguration(ctx, &metadatav1.GetConfigurationRequest{}); err != nil {
		t.Fatal(err)
	}

	resp, err := m.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{Id: 550})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Title != "Fight Club" {
		t.Errorf("expected 'Fight Club', got %s", resp.Title)
	}
	if resp.Runtime != 139 {
		t.Errorf("expected runtime 139, got %d", resp.Runtime)
	}
	if resp.ImdbId != "tt0137523" {
		t.Errorf("expected imdb tt0137523, got %s", resp.ImdbId)
	}
	if len(resp.Genres) != 2 {
		t.Fatalf("expected 2 genres, got %d", len(resp.Genres))
	}
	if resp.Genres[0].Name != "Drama" {
		t.Errorf("expected genre Drama, got %s", resp.Genres[0].Name)
	}
	if resp.PosterUrl == "" {
		t.Error("expected poster URL")
	}
}

func TestGetCollection(t *testing.T) {
	srv, m := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	resp, err := m.GetCollection(ctx, &metadatav1.GetCollectionRequest{Id: 10})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Name != "Star Wars Collection" || len(resp.Parts) != 1 {
		t.Fatalf("collection: %+v", resp)
	}
	if resp.Parts[0].Title != "A New Hope" {
		t.Errorf("part title=%q", resp.Parts[0].Title)
	}
}

func TestGetTVDetails(t *testing.T) {
	srv, m := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	if _, err := m.GetConfiguration(ctx, &metadatav1.GetConfigurationRequest{}); err != nil {
		t.Fatal(err)
	}

	resp, err := m.GetTVDetails(ctx, &metadatav1.GetTVDetailsRequest{Id: 1396})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Name != "Breaking Bad" {
		t.Errorf("expected 'Breaking Bad', got %s", resp.Name)
	}
	if resp.NumberOfSeasons != 5 {
		t.Errorf("expected 5 seasons, got %d", resp.NumberOfSeasons)
	}
	if len(resp.Seasons) != 1 {
		t.Fatalf("expected 1 season, got %d", len(resp.Seasons))
	}
	if resp.Seasons[0].Name != "Season 1" {
		t.Errorf("expected 'Season 1', got %s", resp.Seasons[0].Name)
	}
}

func TestGetSeasonDetails(t *testing.T) {
	srv, m := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	resp, err := m.GetSeasonDetails(ctx, &metadatav1.GetSeasonDetailsRequest{
		Id:           1396,
		SeasonNumber: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Name != "Season 1" {
		t.Errorf("expected 'Season 1', got %s", resp.Name)
	}
	if resp.SeasonNumber != 1 {
		t.Errorf("expected season 1, got %d", resp.SeasonNumber)
	}
	if len(resp.Episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(resp.Episodes))
	}
	ep := resp.Episodes[0]
	if ep.Name != "Pilot" {
		t.Errorf("expected 'Pilot', got %s", ep.Name)
	}
	if ep.EpisodeNumber != 1 {
		t.Errorf("expected episode 1, got %d", ep.EpisodeNumber)
	}
	if ep.Id != 62085 {
		t.Errorf("expected tmdb id 62085, got %d", ep.Id)
	}
	if ep.StillPath != "/still.jpg" {
		t.Errorf("expected still path, got %s", ep.StillPath)
	}
	if ep.Runtime != 58 {
		t.Errorf("expected runtime 58, got %d", ep.Runtime)
	}
	if ep.ProductionCode != "101" {
		t.Errorf("expected production code 101, got %s", ep.ProductionCode)
	}
}

func TestGetConfiguration(t *testing.T) {
	srv, m := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	resp, err := m.GetConfiguration(ctx, &metadatav1.GetConfigurationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.BaseUrl == "" {
		t.Error("expected base URL")
	}
	if len(resp.PosterSizes) == 0 {
		t.Error("expected poster sizes")
	}
}

func TestHealthNoAPIKey(t *testing.T) {
	m := NewModule(Config{})
	ctx := context.Background()
	if err := m.Health(ctx); err == nil {
		t.Error("expected health error without API key")
	}
}

func TestHealthWithAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/3/configuration" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"images": map[string]any{"secure_base_url": "https://image.tmdb.org/t/p/"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	m := NewModule(Config{APIKey: "test-key", BaseURL: srv.URL})
	ctx := context.Background()
	if err := m.Health(ctx); err != nil {
		t.Errorf("expected health ok with valid key, got %v", err)
	}
}

func TestLifecycle(t *testing.T) {
	m := NewModule(Config{GRPCAddr: ":0"})
	ctx := context.Background()
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

// ── Helper tests ───────────────────────────────────────────

func TestImageURL(t *testing.T) {
	m := NewModule(Config{})
	cfg := &tmdbConfig{
		Images: struct {
			BaseURL       string   `json:"base_url"`
			SecureBaseURL string   `json:"secure_base_url"`
			PosterSizes   []string `json:"poster_sizes"`
			BackdropSizes []string `json:"backdrop_sizes"`
			LogoSizes     []string `json:"logo_sizes"`
			ProfileSizes  []string `json:"profile_sizes"`
			StillSizes    []string `json:"still_sizes"`
		}{
			SecureBaseURL: "https://image.tmdb.org/t/p/",
		},
	}

	got := m.imageURL("/abc.jpg", cfg)
	want := "https://image.tmdb.org/t/p/original/abc.jpg"
	if got != want {
		t.Errorf("imageURL = %q, want %q", got, want)
	}
}

func TestImageURLEmptyPath(t *testing.T) {
	m := NewModule(Config{})
	cfg := &tmdbConfig{
		Images: struct {
			BaseURL       string   `json:"base_url"`
			SecureBaseURL string   `json:"secure_base_url"`
			PosterSizes   []string `json:"poster_sizes"`
			BackdropSizes []string `json:"backdrop_sizes"`
			LogoSizes     []string `json:"logo_sizes"`
			ProfileSizes  []string `json:"profile_sizes"`
			StillSizes    []string `json:"still_sizes"`
		}{
			SecureBaseURL: "https://image.tmdb.org/t/p/",
		},
	}

	if got := m.imageURL("", cfg); got != "" {
		t.Errorf("expected empty for empty path, got %q", got)
	}
	if got := m.imageURL("/abc.jpg", nil); got != "" {
		t.Errorf("expected empty for nil cfg, got %q", got)
	}
}

func TestGetCachedConfigNil(t *testing.T) {
	m := NewModule(Config{})
	if cfg := m.getCachedConfig(); cfg != nil {
		t.Error("expected nil cached config before any fetch")
	}
}

func TestMovieDetailRawToProto(t *testing.T) {
	raw := movieDetailRaw{
		ID: 550, Title: "Fight Club", OriginalTitle: "Fight Club",
		Overview: "A ticking-clock thriller.", Tagline: "First rule...",
		PosterPath: "/abc.jpg", BackdropPath: "/def.jpg",
		ReleaseDate: "1999-10-15", Runtime: 139,
		VoteAverage: 8.4, VoteCount: 25000, Popularity: 100,
		Status: "Released", OriginalLanguage: "en",
		Adult: false, Budget: 63000000, Revenue: 100000000,
		Homepage: "https://fightclub.com", IMDBID: "tt0137523",
		Genres: []genreRaw{
			{ID: 18, Name: "Drama"},
			{ID: 53, Name: "Thriller"},
		},
		ProductionCos: []companyRaw{
			{ID: 508, Name: "20th Century Fox", LogoPath: "/logo.png", OriginCountry: "US"},
		},
		SpokenLangs: []languageRaw{
			{ISO6391: "en", Name: "English"},
		},
		BelongsTo: &collectionRaw{
			ID: 1, Name: "Collection", PosterPath: "/p.jpg", BackdropPath: "/b.jpg",
		},
	}

	cfg := &tmdbConfig{
		Images: struct {
			BaseURL       string   `json:"base_url"`
			SecureBaseURL string   `json:"secure_base_url"`
			PosterSizes   []string `json:"poster_sizes"`
			BackdropSizes []string `json:"backdrop_sizes"`
			LogoSizes     []string `json:"logo_sizes"`
			ProfileSizes  []string `json:"profile_sizes"`
			StillSizes    []string `json:"still_sizes"`
		}{
			SecureBaseURL: "https://image.tmdb.org/t/p/",
		},
	}

	p := raw.toProto(cfg)
	if p.Id != 550 || p.Title != "Fight Club" {
		t.Errorf("Id/Title = %d/%q, want 550/'Fight Club'", p.Id, p.Title)
	}
	if p.Runtime != 139 {
		t.Errorf("Runtime = %d, want 139", p.Runtime)
	}
	if p.VoteAverage != 8.4 {
		t.Errorf("VoteAverage = %f, want 8.4", p.VoteAverage)
	}
	if p.ImdbId != "tt0137523" {
		t.Errorf("ImdbId = %q, want tt0137523", p.ImdbId)
	}
	if len(p.Genres) != 2 || p.Genres[0].Name != "Drama" {
		t.Errorf("Genres = %v, want [Drama Thriller]", p.Genres)
	}
	if len(p.ProductionCompanies) != 1 {
		t.Errorf("expected 1 production company, got %d", len(p.ProductionCompanies))
	}
	if len(p.SpokenLanguages) != 1 {
		t.Errorf("expected 1 spoken language, got %d", len(p.SpokenLanguages))
	}
	if p.BelongsToCollection == nil || p.BelongsToCollection.Name != "Collection" {
		t.Error("expected belongs_to_collection")
	}
	if p.PosterUrl == "" {
		t.Error("expected poster URL with cfg")
	}
}

func TestMovieDetailRawToProtoNoCfg(t *testing.T) {
	raw := movieDetailRaw{
		ID: 1, Title: "Test", PosterPath: "/abc.jpg",
	}
	p := raw.toProto(nil)
	if p.PosterUrl != "" {
		t.Error("expected empty poster URL without cfg")
	}
	if p.BackdropUrl != "" {
		t.Error("expected empty backdrop URL without cfg")
	}
	if p.BelongsToCollection != nil {
		t.Error("expected nil collection when not present")
	}
}

func TestMovieDetailRawToProtoEmptyPosterPath(t *testing.T) {
	cfg := &tmdbConfig{
		Images: struct {
			BaseURL       string   `json:"base_url"`
			SecureBaseURL string   `json:"secure_base_url"`
			PosterSizes   []string `json:"poster_sizes"`
			BackdropSizes []string `json:"backdrop_sizes"`
			LogoSizes     []string `json:"logo_sizes"`
			ProfileSizes  []string `json:"profile_sizes"`
			StillSizes    []string `json:"still_sizes"`
		}{
			SecureBaseURL: "https://image.tmdb.org/t/p/",
		},
	}
	raw := movieDetailRaw{ID: 1, Title: "Test"}
	p := raw.toProto(cfg)
	if p.PosterUrl != "" || p.BackdropUrl != "" {
		t.Errorf("expected empty image URLs for empty paths, got poster=%q backdrop=%q", p.PosterUrl, p.BackdropUrl)
	}
}

func TestTVDetailRawToProto(t *testing.T) {
	raw := tvDetailRaw{
		ID: 1668, Name: "Breaking Bad", OriginalName: "Breaking Bad",
		Overview: "A high school chemistry teacher.", Tagline: "I am the danger.",
		PosterPath: "/abc.jpg", BackdropPath: "/def.jpg",
		FirstAirDate: "2008-01-20", LastAirDate: "2013-09-29",
		NumSeasons: 5, NumEpisodes: 62,
		VoteAverage: 8.9, VoteCount: 10000, Popularity: 100,
		Status: "Ended", OriginalLanguage: "en",
		Homepage: "https://breakingbad.com", InProduction: false,
		OriginCountries: []string{"US"},
		Genres: []genreRaw{
			{ID: 18, Name: "Drama"},
			{ID: 80, Name: "Crime"},
		},
		Seasons: []seasonRaw{
			{ID: 1, Name: "Season 1", SeasonNumber: 1, EpisodeCount: 7, AirDate: "2008-01-20"},
		},
		ProductionCos: []companyRaw{
			{ID: 1, Name: "Sony", LogoPath: "/logo.png", OriginCountry: "US"},
		},
		SpokenLangs: []languageRaw{
			{ISO6391: "en", Name: "English"},
		},
		CreatedBy: []creatorRaw{
			{ID: 1, Name: "Vince Gilligan", ProfilePath: "/vince.jpg"},
		},
		Networks: []networkRaw{
			{ID: 1, Name: "AMC", LogoPath: "/amc.png", OriginCountry: "US"},
		},
	}

	cfg := &tmdbConfig{
		Images: struct {
			BaseURL       string   `json:"base_url"`
			SecureBaseURL string   `json:"secure_base_url"`
			PosterSizes   []string `json:"poster_sizes"`
			BackdropSizes []string `json:"backdrop_sizes"`
			LogoSizes     []string `json:"logo_sizes"`
			ProfileSizes  []string `json:"profile_sizes"`
			StillSizes    []string `json:"still_sizes"`
		}{
			SecureBaseURL: "https://image.tmdb.org/t/p/",
		},
	}

	p := raw.toProto(cfg)
	if p.Id != 1668 || p.Name != "Breaking Bad" {
		t.Errorf("Id/Name = %d/%q, want 1668/'Breaking Bad'", p.Id, p.Name)
	}
	if p.NumberOfSeasons != 5 || p.NumberOfEpisodes != 62 {
		t.Errorf("Seasons/Episodes = %d/%d, want 5/62", p.NumberOfSeasons, p.NumberOfEpisodes)
	}
	if p.VoteAverage != 8.9 {
		t.Errorf("VoteAverage = %f, want 8.9", p.VoteAverage)
	}
	if len(p.Genres) != 2 || p.Genres[0].Name != "Drama" {
		t.Errorf("Genres = %v, want [Drama Crime]", p.Genres)
	}
	if len(p.Seasons) != 1 || p.Seasons[0].SeasonNumber != 1 {
		t.Errorf("Seasons = %+v", p.Seasons)
	}
	if len(p.CreatedBy) != 1 {
		t.Errorf("expected 1 creator, got %d", len(p.CreatedBy))
	}
	if len(p.Networks) != 1 {
		t.Errorf("expected 1 network, got %d", len(p.Networks))
	}
	if len(p.OriginCountry) != 1 || p.OriginCountry[0] != "US" {
		t.Errorf("OriginCountry = %v, want [US]", p.OriginCountry)
	}
	if p.PosterUrl == "" {
		t.Error("expected poster URL with cfg")
	}
	if p.InProduction != false {
		t.Errorf("InProduction = %v, want false", p.InProduction)
	}
}

func TestTVDetailRawToProtoNoCfg(t *testing.T) {
	raw := tvDetailRaw{ID: 1, Name: "Test", PosterPath: "/abc.jpg"}
	p := raw.toProto(nil)
	if p.PosterUrl != "" {
		t.Error("expected empty poster URL without cfg")
	}
	if len(p.CreatedBy) != 0 {
		t.Error("expected 0 creators when empty")
	}
}

// ── parseSearchResult ──────────────────────────────────────

func TestParseSearchResultMovie(t *testing.T) {
	m := NewModule(Config{})
	raw := json.RawMessage(`{
		"id": 550, "title": "Fight Club", "original_title": "Fight Club",
		"overview": "A ticking-clock thriller.",
		"poster_path": "/abc.jpg", "backdrop_path": "/def.jpg",
		"release_date": "1999-10-15",
		"vote_average": 8.4, "vote_count": 25000, "popularity": 100,
		"original_language": "en", "genre_ids": [18, 53],
		"media_type": "movie"
	}`)

	r := m.parseSearchResult(raw)
	if r == nil {
		t.Fatal("expected non-nil result")
	}
	if r.Id != 550 || r.Title != "Fight Club" {
		t.Errorf("Id/Title = %d/%q, want 550/'Fight Club'", r.Id, r.Title)
	}
	if r.MediaType != metadatav1.MediaType_MEDIA_TYPE_MOVIE {
		t.Errorf("MediaType = %v, want MOVIE", r.MediaType)
	}
	if r.ReleaseDate != "1999-10-15" {
		t.Errorf("ReleaseDate = %q, want 1999-10-15", r.ReleaseDate)
	}
	if len(r.GenreIds) != 2 || r.GenreIds[0] != 18 {
		t.Errorf("GenreIds = %v, want [18, 53]", r.GenreIds)
	}
	if r.VoteAverage != 8.4 {
		t.Errorf("VoteAverage = %f, want 8.4", r.VoteAverage)
	}
}

func TestParseSearchResultTV(t *testing.T) {
	m := NewModule(Config{})
	raw := json.RawMessage(`{
		"id": 1396, "name": "Breaking Bad", "original_name": "Breaking Bad",
		"overview": "A high school chemistry teacher.",
		"poster_path": "/abc.jpg",
		"first_air_date": "2008-01-20",
		"vote_average": 8.9, "vote_count": 10000,
		"media_type": "tv"
	}`)

	r := m.parseSearchResult(raw)
	if r == nil {
		t.Fatal("expected non-nil result")
	}
	if r.Id != 1396 || r.Name != "Breaking Bad" {
		t.Errorf("Id/Name = %d/%q, want 1396/'Breaking Bad'", r.Id, r.Name)
	}
	if r.MediaType != metadatav1.MediaType_MEDIA_TYPE_TV {
		t.Errorf("MediaType = %v, want TV", r.MediaType)
	}
	if r.FirstAirDate != "2008-01-20" {
		t.Errorf("FirstAirDate = %q, want 2008-01-20", r.FirstAirDate)
	}
}

func TestParseSearchResultBadJSON(t *testing.T) {
	m := NewModule(Config{})
	raw := json.RawMessage(`{invalid}`)
	r := m.parseSearchResult(raw)
	if r != nil {
		t.Error("expected nil result for bad JSON")
	}
}

func TestParseSearchResultNoMediaType(t *testing.T) {
	m := NewModule(Config{})
	raw := json.RawMessage(`{
		"id": 1, "title": "Test",
		"vote_average": 5.0
	}`)
	r := m.parseSearchResult(raw)
	if r == nil {
		t.Fatal("expected non-nil result")
	}
	if r.MediaType != metadatav1.MediaType_MEDIA_TYPE_UNSPECIFIED {
		t.Errorf("MediaType = %v, want UNSPECIFIED", r.MediaType)
	}
}

func TestParseSearchResultFiltersPerson(t *testing.T) {
	m := NewModule(Config{})
	raw := json.RawMessage(`{
		"id": 31, "name": "Tom Hanks",
		"media_type": "person",
		"popularity": 10.0
	}`)
	if r := m.parseSearchResult(raw); r != nil {
		t.Fatalf("expected nil for person, got %+v", r)
	}
}

func TestParseSearchResultFiltersAdult(t *testing.T) {
	m := NewModule(Config{})
	raw := json.RawMessage(`{
		"id": 99, "title": "Adult Title",
		"media_type": "movie",
		"adult": true
	}`)
	if r := m.parseSearchResult(raw); r != nil {
		t.Fatalf("expected nil for adult title, got %+v", r)
	}
}

// ── Edge cases ─────────────────────────────────────────────

func TestTMDBGetNoAPIKey(t *testing.T) {
	m := NewModule(Config{BaseURL: "http://localhost:1"})
	ctx := context.Background()

	_, err := m.Search(ctx, &metadatav1.SearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("expected error without API key")
	}
}

func TestFixtureSearchMovie(t *testing.T) {
	m := NewModule(Config{Fixture: true})
	ctx := context.Background()
	if err := m.Health(ctx); err != nil {
		t.Fatalf("fixture Health: %v", err)
	}
	resp, err := m.Search(ctx, &metadatav1.SearchRequest{
		Query: "Fight Club",
		Type:  metadatav1.MediaType_MEDIA_TYPE_MOVIE,
		Year:  1999,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Id != 550 || resp.Results[0].Title != "Fight Club" {
		t.Fatalf("unexpected fixture search: %+v", resp.Results)
	}
}

func TestSearchDefaultMulti(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/3/search/multi" {
			if r.URL.Query().Get("include_adult") != "false" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"page": 1, "total_results": 2, "total_pages": 1,
				"results": []map[string]any{
					{"id": 1, "title": "Multi", "media_type": "movie"},
					{"id": 31, "name": "Tom Hanks", "media_type": "person"},
				},
			})
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	ctx := context.Background()

	resp, err := m.Search(ctx, &metadatav1.SearchRequest{Query: "multi"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result (person filtered), got %d", len(resp.Results))
	}
	if resp.Results[0].Title != "Multi" {
		t.Errorf("expected 'Multi', got %s", resp.Results[0].Title)
	}
}

func TestMovieDetailsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"status_message": "Not found"})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	if err := m.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	ctx := context.Background()

	_, err := m.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{Id: 99999})
	if err == nil {
		t.Fatal("expected error for not found movie")
	}
}

func TestSearchBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	ctx := context.Background()

	_, err := m.Search(ctx, &metadatav1.SearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("expected error for bad status")
	}
}

func TestGetMovieDetailsWithLanguage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("language") != "fr" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 550, "title": "Fight Club",
			"original_title": "Fight Club",
			"overview":       "Version française",
			"genres":         []map[string]any{},
		})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	ctx := context.Background()

	resp, err := m.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{
		Id:       550,
		Language: "fr",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Title != "Fight Club" {
		t.Errorf("Title = %q, want 'Fight Club'", resp.Title)
	}
}

func TestConfigurationCaching(t *testing.T) {
	fetchCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"images": map[string]any{
				"base_url":        "http://image.tmdb.org/t/p/",
				"secure_base_url": "https://image.tmdb.org/t/p/",
				"poster_sizes":    []string{"original"},
			},
		})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	ctx := context.Background()

	cfg, err := m.getConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	cfg2, err := m.getConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2 == nil {
		t.Fatal("expected non-nil cached config")
	}
	if fetchCount != 1 {
		t.Errorf("expected 1 fetch, got %d", fetchCount)
	}

	if cached := m.getCachedConfig(); cached == nil {
		t.Error("getCachedConfig should return last config")
	}
}

func TestSearchZeroPageDefaults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 1, "total_results": 0, "total_pages": 0,
			"results": []map[string]any{},
		})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	ctx := context.Background()

	// Zero page should default to 1 (no crash).
	resp, err := m.Search(ctx, &metadatav1.SearchRequest{Query: "test", Page: 0})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Page != 1 {
		t.Errorf("expected page=1, got %d", resp.Page)
	}
}

func TestEmptySearchResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 1, "total_results": 0, "total_pages": 0,
			"results": []map[string]any{},
		})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	ctx := context.Background()

	resp, err := m.Search(ctx, &metadatav1.SearchRequest{Query: "nonexistent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(resp.Results))
	}
	if resp.TotalResults != 0 {
		t.Errorf("expected TotalResults=0, got %d", resp.TotalResults)
	}
}

func TestMovieDetailRawToProtoNoGenresOrCompanies(t *testing.T) {
	raw := movieDetailRaw{ID: 1, Title: "Minimal"}
	p := raw.toProto(nil)
	if len(p.Genres) != 0 {
		t.Error("expected empty genres")
	}
	if len(p.ProductionCompanies) != 0 {
		t.Error("expected empty production companies")
	}
	if len(p.SpokenLanguages) != 0 {
		t.Error("expected empty spoken languages")
	}
}

func TestTVDetailRawToProtoEmptySeasonsCreatorsNetworks(t *testing.T) {
	raw := tvDetailRaw{ID: 1, Name: "Minimal"}
	p := raw.toProto(nil)
	if len(p.Seasons) != 0 {
		t.Error("expected empty seasons")
	}
	if len(p.CreatedBy) != 0 {
		t.Error("expected empty creators")
	}
	if len(p.Networks) != 0 {
		t.Error("expected empty networks")
	}
	if len(p.Genres) != 0 {
		t.Error("expected empty genres")
	}
}

func TestFindByExternalIDMovie(t *testing.T) {
	_, m := newTestServer(t)
	ctx := context.Background()
	resp, err := m.FindByExternalID(ctx, &metadatav1.FindByExternalIDRequest{
		ExternalId:     "tt0137523",
		ExternalSource: "imdb_id",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].Id != 550 {
		t.Errorf("expected id 550, got %d", resp.Results[0].Id)
	}
	if resp.Results[0].MediaType != metadatav1.MediaType_MEDIA_TYPE_MOVIE {
		t.Errorf("expected movie, got %v", resp.Results[0].MediaType)
	}
}

func TestFindByExternalIDTV(t *testing.T) {
	_, m := newTestServer(t)
	ctx := context.Background()
	resp, err := m.FindByExternalID(ctx, &metadatav1.FindByExternalIDRequest{
		ExternalId: "tt0903747",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].Id != 1396 {
		t.Errorf("expected id 1396, got %d", resp.Results[0].Id)
	}
	if resp.Results[0].MediaType != metadatav1.MediaType_MEDIA_TYPE_TV {
		t.Errorf("expected tv, got %v", resp.Results[0].MediaType)
	}
}

func TestFindByExternalIDRequiresID(t *testing.T) {
	_, m := newTestServer(t)
	_, err := m.FindByExternalID(context.Background(), &metadatav1.FindByExternalIDRequest{})
	if err == nil {
		t.Fatal("expected error for empty external_id")
	}
}

func TestResponseCacheHit(t *testing.T) {
	fetchCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 1, "total_results": 1, "total_pages": 1,
			"results": []map[string]any{
				{"id": 1, "title": "Cached", "media_type": "movie"},
			},
		})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	ctx := context.Background()
	req := &metadatav1.SearchRequest{Query: "cached", Type: metadatav1.MediaType_MEDIA_TYPE_MOVIE}

	if _, err := m.Search(ctx, req); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Search(ctx, req); err != nil {
		t.Fatal(err)
	}
	if fetchCount != 1 {
		t.Errorf("expected 1 fetch, got %d", fetchCount)
	}
}

func TestResponseCacheTTLExpiry(t *testing.T) {
	fetchCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 1, "total_results": 0, "total_pages": 0,
			"results": []map[string]any{},
		})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	m.cache.ttlSearch = time.Millisecond
	ctx := context.Background()
	req := &metadatav1.SearchRequest{Query: "expire", Type: metadatav1.MediaType_MEDIA_TYPE_MOVIE}

	if _, err := m.Search(ctx, req); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := m.Search(ctx, req); err != nil {
		t.Fatal(err)
	}
	if fetchCount != 2 {
		t.Errorf("expected 2 fetches after TTL expiry, got %d", fetchCount)
	}
}

func TestInflightCoalesce(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var fetchCount atomic.Int32
	var startOnce sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		startOnce.Do(func() { close(started) })
		<-release
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 1, "total_results": 0, "total_pages": 0,
			"results": []map[string]any{},
		})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	m.limiter.rate = 0
	ctx := context.Background()
	req := &metadatav1.SearchRequest{Query: "coalesce", Type: metadatav1.MediaType_MEDIA_TYPE_MOVIE}

	errCh := make(chan error, 2)
	go func() { _, err := m.Search(ctx, req); errCh <- err }()
	<-started
	go func() { _, err := m.Search(ctx, req); errCh <- err }()
	close(release)

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
	if got := fetchCount.Load(); got != 1 {
		t.Errorf("expected 1 coalesced fetch, got %d", got)
	}
}

func TestTMDB429RetryThenSuccess(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 1, "total_results": 0, "total_pages": 0,
			"results": []map[string]any{},
		})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	m.limiter.rate = 0
	m.cache.max = 0
	_, err := m.Search(context.Background(), &metadatav1.SearchRequest{
		Query: "retry", Type: metadatav1.MediaType_MEDIA_TYPE_MOVIE,
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", calls.Load())
	}
}

func TestTMDB429Exhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	m.limiter.rate = 0
	m.cache.max = 0
	_, err := m.Search(context.Background(), &metadatav1.SearchRequest{
		Query: "fail", Type: metadatav1.MediaType_MEDIA_TYPE_MOVIE,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", err)
	}
	if !strings.Contains(st.Message(), "rate_limit_exceeded") {
		t.Errorf("message = %q, want rate_limit_exceeded", st.Message())
	}
}

func TestRateLimiterWaitCancel(t *testing.T) {
	b := &tokenBucket{rate: 1, burst: 1, tokens: 0, last: time.Now()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.wait(ctx); err == nil {
		t.Fatal("expected context cancel error")
	}
}

func TestRateLimiterAllowsToken(t *testing.T) {
	b := &tokenBucket{rate: 1000, burst: 1, tokens: 1, last: time.Now()}
	ctx := context.Background()
	if err := b.wait(ctx); err != nil {
		t.Fatal(err)
	}
	ctx2, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	// tokens depleted; wait should either succeed after refill or cancel
	_ = b.wait(ctx2)
}

func TestCacheKeyStripsAPIKey(t *testing.T) {
	params := url.Values{}
	params.Set("api_key", "secret")
	params.Set("query", "x")
	got := cacheKey("/3/search/movie", params)
	if strings.Contains(got, "secret") || strings.Contains(got, "api_key") {
		t.Errorf("cache key leaked api_key: %q", got)
	}
	if !strings.Contains(got, "query=x") {
		t.Errorf("cache key missing query: %q", got)
	}
}

func TestTTLForEndpoints(t *testing.T) {
	c := newHTTPCache()
	cases := []struct {
		endpoint string
		want     time.Duration
	}{
		{"/3/configuration", c.ttlConfig},
		{"/3/search/movie", c.ttlSearch},
		{"/3/find/tt1", c.ttlSearch},
		{"/3/trending/movie/week", c.ttlList},
		{"/3/movie/popular", c.ttlList},
		{"/3/tv/popular", c.ttlList},
		{"/3/movie/550", c.ttlDetails},
		{"/3/tv/1396", c.ttlDetails},
		{"/3/collection/10", c.ttlDetails},
		{"/3/genre/movie/list", c.ttlDefault},
	}
	for _, tc := range cases {
		if got := c.ttlFor(tc.endpoint); got != tc.want {
			t.Errorf("ttlFor(%s) = %v, want %v", tc.endpoint, got, tc.want)
		}
	}
}

func TestListTrending(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/trending/movie/week" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 1, "total_results": 1, "total_pages": 1,
			"results": []map[string]any{
				{"id": 550, "title": "Fight Club", "media_type": "movie"},
			},
		})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	ctx := context.Background()
	resp, err := m.ListTrending(ctx, &metadatav1.ListTrendingRequest{
		MediaType:  metadatav1.TrendingMediaType_TRENDING_MEDIA_TYPE_MOVIE,
		TimeWindow: metadatav1.TrendingTimeWindow_TRENDING_TIME_WINDOW_WEEK,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Title != "Fight Club" {
		t.Fatalf("unexpected trending: %+v", resp.Results)
	}
}

func TestListPopular(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/movie/popular" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 1, "total_results": 1, "total_pages": 1,
			"results": []map[string]any{
				{"id": 550, "title": "Fight Club", "media_type": "movie"},
			},
		})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	resp, err := m.ListPopular(context.Background(), &metadatav1.ListPopularRequest{
		Type: metadatav1.MediaType_MEDIA_TYPE_MOVIE,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 popular result, got %d", len(resp.Results))
	}
}

func TestUpdateSetting(t *testing.T) {
	m := NewModule(Config{APIKey: "old", BaseURL: "https://api.example.test"})
	if err := m.updateSetting("api_key", "new-key"); err != nil {
		t.Fatal(err)
	}
	m.mu.RLock()
	got := m.apiKey
	m.mu.RUnlock()
	if got != "new-key" {
		t.Errorf("api_key = %q, want new-key", got)
	}
	if err := m.updateSetting("api_key", "********"); err != nil {
		t.Fatal(err)
	}
	m.mu.RLock()
	got = m.apiKey
	m.mu.RUnlock()
	if got != "new-key" {
		t.Errorf("masked update changed key to %q", got)
	}
	if err := m.updateSetting("base_url", "https://custom.example/"); err != nil {
		t.Fatal(err)
	}
	m.mu.RLock()
	base := m.baseURL
	m.mu.RUnlock()
	if base != "https://custom.example" {
		t.Errorf("base_url = %q", base)
	}
	if err := m.updateSetting("unknown", "x"); err == nil {
		t.Fatal("expected error for unknown setting")
	}
}

func TestGetMovieDetailsWithAppendVideosAndCredits(t *testing.T) {
	_, m := newTestServer(t)
	ctx := context.Background()
	resp, err := m.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{
		Id:               550,
		AppendToResponse: []string{"videos", "credits"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Videos) != 1 || resp.Videos[0].Key != "SUXWAEX2jlg" {
		t.Fatalf("videos: %+v", resp.Videos)
	}
	if len(resp.Credits) != 1 {
		t.Fatalf("expected credits block, got %d", len(resp.Credits))
	}
	if len(resp.Credits[0].Cast) != 1 || resp.Credits[0].Cast[0].Name != "Brad Pitt" {
		t.Fatalf("cast: %+v", resp.Credits[0].Cast)
	}
	if len(resp.Credits[0].Crew) != 1 || resp.Credits[0].Crew[0].Job != "Director" {
		t.Fatalf("crew: %+v", resp.Credits[0].Crew)
	}
}

func TestGetTVDetailsWithAppendVideos(t *testing.T) {
	_, m := newTestServer(t)
	resp, err := m.GetTVDetails(context.Background(), &metadatav1.GetTVDetailsRequest{
		Id:               1396,
		AppendToResponse: []string{"videos"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Videos) != 1 || resp.Videos[0].Site != "YouTube" {
		t.Fatalf("videos: %+v", resp.Videos)
	}
}

func TestSearchEmptyQueryInvalidArgument(t *testing.T) {
	m := NewModule(Config{APIKey: "key", BaseURL: "http://localhost:1"})
	_, err := m.Search(context.Background(), &metadatav1.SearchRequest{Query: "  "})
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestGetMovieDetailsZeroIDInvalidArgument(t *testing.T) {
	m := NewModule(Config{APIKey: "key"})
	_, err := m.GetMovieDetails(context.Background(), &metadatav1.GetMovieDetailsRequest{Id: 0})
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestTMDB401Unauthenticated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"status_message": "Invalid API key"})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "bad"})
	m.limiter.rate = 0
	m.cache.max = 0
	_, err := m.Search(context.Background(), &metadatav1.SearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestHealthInvalidAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "bad"})
	m.limiter.rate = 0
	m.cache.max = 0
	if err := m.Health(context.Background()); err == nil {
		t.Fatal("expected health error for invalid key")
	}
}

func TestSettingsChangeClearsCache(t *testing.T) {
	fetchCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 1, "total_results": 0, "total_pages": 0,
			"results": []map[string]any{},
		})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key1"})
	ctx := context.Background()
	req := &metadatav1.SearchRequest{Query: "cached", Type: metadatav1.MediaType_MEDIA_TYPE_MOVIE}
	if _, err := m.Search(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := m.updateSetting("api_key", "key2"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Search(ctx, req); err != nil {
		t.Fatal(err)
	}
	if fetchCount != 2 {
		t.Errorf("expected cache clear after api_key change, fetchCount=%d", fetchCount)
	}
}

func TestFixtureMovieVideosAndCredits(t *testing.T) {
	m := NewModule(Config{Fixture: true})
	ctx := context.Background()
	resp, err := m.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{
		Id:               550,
		AppendToResponse: []string{"videos", "credits"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Videos) != 1 || resp.Videos[0].Site != "YouTube" {
		t.Fatalf("fixture videos: %+v", resp.Videos)
	}
	if len(resp.Credits) != 1 || len(resp.Credits[0].Cast) < 2 {
		t.Fatalf("fixture credits: %+v", resp.Credits)
	}
}

func TestFixtureTVSeasonsAndCollection(t *testing.T) {
	m := NewModule(Config{Fixture: true})
	ctx := context.Background()
	for _, season := range []int32{0, 3, 4, 5} {
		if _, err := m.GetSeasonDetails(ctx, &metadatav1.GetSeasonDetailsRequest{
			Id: 1396, SeasonNumber: season,
		}); err != nil {
			t.Fatalf("season %d: %v", season, err)
		}
	}
	col, err := m.GetCollection(ctx, &metadatav1.GetCollectionRequest{Id: 10})
	if err != nil {
		t.Fatal(err)
	}
	if col.Name != "Star Wars Collection" {
		t.Errorf("collection name = %q", col.Name)
	}
}

func TestMapCredits(t *testing.T) {
	out := mapCredits(&creditsAppendRaw{
		Cast: []castRaw{{ID: 1, Name: "Actor", Character: "Role", Order: 0}},
		Crew: []crewRaw{{ID: 2, Name: "Director", Job: "Director", Department: "Directing"}},
	})
	if len(out) != 1 || len(out[0].Cast) != 1 || len(out[0].Crew) != 1 {
		t.Fatalf("mapCredits: %+v", out)
	}
	if mapCredits(nil) != nil {
		t.Error("expected nil for nil credits")
	}
}
