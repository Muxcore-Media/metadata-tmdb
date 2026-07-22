package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metadatav1 "github.com/Muxcore-Media/metadata-tmdb/proto/metadatav1"
)

func newTestServer(t *testing.T) (*httptest.Server, *Module) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/configuration":
			json.NewEncoder(w).Encode(map[string]any{
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
			json.NewEncoder(w).Encode(map[string]any{
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
		case "/3/search/tv":
			json.NewEncoder(w).Encode(map[string]any{
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
		case "/3/movie/550":
			json.NewEncoder(w).Encode(map[string]any{
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
			})
		case "/3/collection/10":
			json.NewEncoder(w).Encode(map[string]any{
				"id": 10, "name": "Star Wars Collection", "overview": "A long time ago...",
				"poster_path": "/c.jpg", "backdrop_path": "/b.jpg",
				"parts": []map[string]any{
					{"id": 11, "title": "A New Hope", "release_date": "1977-05-25", "media_type": "movie", "vote_average": 8.2},
				},
			})
		case "/3/tv/1396":
			json.NewEncoder(w).Encode(map[string]any{
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
			})
		case "/3/tv/1396/season/1":
			json.NewEncoder(w).Encode(map[string]any{
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
			json.NewEncoder(w).Encode(map[string]any{
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
			json.NewEncoder(w).Encode(map[string]any{
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
			json.NewEncoder(w).Encode(map[string]string{"status_message": "Not found"})
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
	t.Cleanup(func() { m.Stop(ctx) })

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

func TestGetMovieDetails(t *testing.T) {
	srv, m := newTestServer(t)
	defer srv.Close()
	ctx := context.Background()

	if _, err := m.GetConfiguration(ctx, &metadatav1.GetConfigurationRequest{}); err != nil {
		t.Fatal(err)
	}

	resp, err := m.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{TmdbId: 550})
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

	resp, err := m.GetCollection(ctx, &metadatav1.GetCollectionRequest{TmdbId: 10})
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

	resp, err := m.GetTVDetails(ctx, &metadatav1.GetTVDetailsRequest{TmdbId: 1396})
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
		TmdbId:       1396,
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
	m := NewModule(Config{APIKey: "test-key"})
	ctx := context.Background()
	if err := m.Health(ctx); err != nil {
		t.Errorf("expected health ok with API key, got %v", err)
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
	if p.InProduction != "false" {
		t.Errorf("InProduction = %q, want 'false'", p.InProduction)
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

// ── Edge cases ─────────────────────────────────────────────

func TestTMDBGetNoAPIKey(t *testing.T) {
	m := NewModule(Config{BaseURL: "http://localhost:1"})
	ctx := context.Background()

	_, err := m.Search(ctx, &metadatav1.SearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("expected error without API key")
	}
}

func TestSearchDefaultMulti(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/3/search/multi" {
			json.NewEncoder(w).Encode(map[string]any{
				"page": 1, "total_results": 1, "total_pages": 1,
				"results": []map[string]any{
					{"id": 1, "title": "Multi", "media_type": "movie"},
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
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].Title != "Multi" {
		t.Errorf("expected 'Multi', got %s", resp.Results[0].Title)
	}
}

func TestMovieDetailsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"status_message": "Not found"})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	m.Init(context.Background())
	ctx := context.Background()

	_, err := m.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{TmdbId: 99999})
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
		json.NewEncoder(w).Encode(map[string]any{
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
		TmdbId:   550,
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
		json.NewEncoder(w).Encode(map[string]any{
			"images": map[string]any{
				"base_url":        "http://image.tmdb.org/t/p/",
				"secure_base_url": "https://image.tmdb.org/t/p/",
				"poster_sizes":    []string{"original"},
			},
		})
	}))
	defer srv.Close()

	m := NewModule(Config{BaseURL: srv.URL, APIKey: "key"})
	m.configTTL = 24 * time.Hour
	ctx := context.Background()

	// First call fetches config.
	cfg, err := m.getConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	// Second call should use cache.
	cfg2, err := m.getConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2 != cfg {
		t.Error("expected cached config (same pointer)")
	}
	if fetchCount != 1 {
		t.Errorf("expected 1 fetch, got %d", fetchCount)
	}

	// getCachedConfig should also return the same config.
	if cached := m.getCachedConfig(); cached != cfg {
		t.Error("getCachedConfig should return cached config")
	}
}

func TestSearchZeroPageDefaults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "" || page == "0" {
			page = "1"
		}
		json.NewEncoder(w).Encode(map[string]any{
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
		json.NewEncoder(w).Encode(map[string]any{
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
