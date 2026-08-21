package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Muxcore-Media/core/pkg/contracts"
	metadatav1 "github.com/Muxcore-Media/metadata-tmdb/proto/metadatav1"
)

type tmdbConfig struct {
	Images struct {
		BaseURL       string   `json:"base_url"`
		SecureBaseURL string   `json:"secure_base_url"`
		PosterSizes   []string `json:"poster_sizes"`
		BackdropSizes []string `json:"backdrop_sizes"`
		LogoSizes     []string `json:"logo_sizes"`
		ProfileSizes  []string `json:"profile_sizes"`
		StillSizes    []string `json:"still_sizes"`
	} `json:"images"`
}

type Module struct {
	metadatav1.UnimplementedMetadataServiceServer

	mu         sync.RWMutex
	client     *http.Client
	apiKey     string
	baseURL    string
	fixture    bool
	lastConfig *tmdbConfig
	cache      *httpCache
	inflight   *inflightGroup
	limiter    *tokenBucket

	id       string
	grpcAddr string
	grpcSrv  *grpc.Server
	lis      net.Listener
}

type Config struct {
	ID       string
	GRPCAddr string
	APIKey   string
	Timeout  time.Duration
	BaseURL  string
	Fixture  bool
}

func NewModule(cfg Config) *Module {
	if cfg.ID == "" {
		cfg.ID = "metadata-tmdb"
	}
	if cfg.GRPCAddr == "" {
		cfg.GRPCAddr = ":9411"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.themoviedb.org"
	}
	if v := os.Getenv("MUXCORE_CFG_TMDB_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("TMDB_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("METADATA_GRPC_ADDR"); v != "" {
		cfg.GRPCAddr = v
	}
	if v := os.Getenv("TMDB_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("TMDB_FIXTURE"); v == "1" || strings.EqualFold(v, "true") {
		cfg.Fixture = true
	}
	if strings.EqualFold(cfg.APIKey, "fixture") {
		cfg.Fixture = true
	}
	return &Module{
		id:       cfg.ID,
		grpcAddr: cfg.GRPCAddr,
		apiKey:   cfg.APIKey,
		baseURL:  cfg.BaseURL,
		fixture:  cfg.Fixture,
		cache:    newHTTPCache(),
		inflight: newInflightGroup(),
		limiter:  newTokenBucket(),
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (m *Module) fixtureMode() bool {
	return m.fixture || strings.EqualFold(m.apiKey, "fixture")
}

func (m *Module) Info() contracts.ModuleInfo {
	return contracts.ModuleInfo{
		ID:           m.id,
		Name:         "Metadata TMDB",
		Version:      "0.1.5",
		Roles:        []string{"metadata"},
		Description:  "TMDB (The Movie Database) metadata provider for movies and TV shows",
		Author:       "MuxCore",
		Capabilities: []string{"metadata", "metadata.tmdb", "settings"},
		Contracts: []contracts.ContractDeclaration{
			{
				Repo:      "github.com/Muxcore-Media/contracts-metadata",
				Interface: "MetadataProvider",
				Version:   "v0.1.0",
			},
		},
		MinCoreVersion: "0.4.0",
		HTTPAddr:       m.grpcAddr,
	}
}

func (m *Module) Init(ctx context.Context) error {
	lis, err := net.Listen("tcp", m.grpcAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", m.grpcAddr, err)
	}
	m.lis = lis
	slog.Info("metadata-tmdb initialized", "addr", m.grpcAddr, "fixture", m.fixtureMode())
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	m.grpcSrv = grpc.NewServer()
	metadatav1.RegisterMetadataServiceServer(m.grpcSrv, m)
	m.registerSettingsMesh(m.grpcSrv)
	go func() {
		slog.Info("metadata-tmdb gRPC service started", "addr", m.grpcAddr)
		if err := m.grpcSrv.Serve(m.lis); err != nil {
			slog.Error("metadata-tmdb gRPC serve error", "error", err)
		}
	}()
	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	if m.grpcSrv != nil {
		m.grpcSrv.GracefulStop()
	}
	m.client.CloseIdleConnections()
	slog.Info("metadata-tmdb stopped")
	return nil
}

func (m *Module) Health(ctx context.Context) error {
	if m.fixtureMode() {
		return nil
	}
	if m.apiKey == "" {
		return fmt.Errorf("TMDB API key not configured — set TMDB_API_KEY, MUXCORE_CFG_TMDB_API_KEY, or TMDB_FIXTURE=1")
	}
	return nil
}

func (m *Module) Search(ctx context.Context, req *metadatav1.SearchRequest) (*metadatav1.SearchResponse, error) {
	params := url.Values{}
	params.Set("query", req.GetQuery())
	params.Set("page", strconv.Itoa(int(req.GetPage())))
	if req.GetPage() <= 0 {
		params.Set("page", "1")
	}
	if req.GetLanguage() != "" {
		params.Set("language", req.GetLanguage())
	}

	var endpoint string
	switch req.GetType() {
	case metadatav1.MediaType_MEDIA_TYPE_MOVIE:
		endpoint = "/3/search/movie"
		if req.GetYear() > 0 {
			params.Set("primary_release_year", strconv.Itoa(int(req.GetYear())))
		}
	case metadatav1.MediaType_MEDIA_TYPE_TV:
		endpoint = "/3/search/tv"
		if req.GetYear() > 0 {
			params.Set("first_air_date_year", strconv.Itoa(int(req.GetYear())))
		}
	default:
		endpoint = "/3/search/multi"
		if req.GetYear() > 0 {
			params.Set("year", strconv.Itoa(int(req.GetYear())))
		}
	}

	var raw struct {
		Results      []json.RawMessage `json:"results"`
		TotalResults int               `json:"total_results"`
		TotalPages   int               `json:"total_pages"`
		Page         int               `json:"page"`
	}
	if err := m.tmdbGet(ctx, endpoint, params, &raw); err != nil {
		return nil, err
	}

	results := make([]*metadatav1.SearchResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		sr := m.parseSearchResult(r)
		if sr != nil {
			results = append(results, sr)
		}
	}

	return &metadatav1.SearchResponse{
		Results:      results,
		TotalResults: int32(raw.TotalResults),
		TotalPages:   int32(raw.TotalPages),
		Page:         int32(raw.Page),
	}, nil
}

func (m *Module) GetMovieDetails(ctx context.Context, req *metadatav1.GetMovieDetailsRequest) (*metadatav1.GetMovieDetailsResponse, error) {
	id := req.GetTmdbId()
	endpoint := fmt.Sprintf("/3/movie/%d", id)

	params := url.Values{}
	if req.GetLanguage() != "" {
		params.Set("language", req.GetLanguage())
	}
	if len(req.GetAppendToResponse()) > 0 {
		params.Set("append_to_response", strings.Join(req.GetAppendToResponse(), ","))
	}

	var raw movieDetailRaw
	if err := m.tmdbGet(ctx, endpoint, params, &raw); err != nil {
		return nil, err
	}

	cfg, _ := m.getConfig(ctx)
	return raw.toProto(cfg), nil
}

func (m *Module) GetTVDetails(ctx context.Context, req *metadatav1.GetTVDetailsRequest) (*metadatav1.GetTVDetailsResponse, error) {
	id := req.GetTmdbId()
	endpoint := fmt.Sprintf("/3/tv/%d", id)

	params := url.Values{}
	if req.GetLanguage() != "" {
		params.Set("language", req.GetLanguage())
	}
	if len(req.GetAppendToResponse()) > 0 {
		params.Set("append_to_response", strings.Join(req.GetAppendToResponse(), ","))
	}

	var raw tvDetailRaw
	if err := m.tmdbGet(ctx, endpoint, params, &raw); err != nil {
		return nil, err
	}

	cfg, _ := m.getConfig(ctx)
	return raw.toProto(cfg), nil
}

func (m *Module) GetSeasonDetails(ctx context.Context, req *metadatav1.GetSeasonDetailsRequest) (*metadatav1.GetSeasonDetailsResponse, error) {
	id := req.GetTmdbId()
	season := req.GetSeasonNumber()
	endpoint := fmt.Sprintf("/3/tv/%d/season/%d", id, season)

	params := url.Values{}
	if req.GetLanguage() != "" {
		params.Set("language", req.GetLanguage())
	}

	var raw seasonDetailRaw
	if err := m.tmdbGet(ctx, endpoint, params, &raw); err != nil {
		return nil, err
	}

	return raw.toProto(), nil
}

func (m *Module) GetCollection(ctx context.Context, req *metadatav1.GetCollectionRequest) (*metadatav1.GetCollectionResponse, error) {
	id := req.GetTmdbId()
	if id == 0 {
		return nil, fmt.Errorf("tmdb_id required")
	}
	endpoint := fmt.Sprintf("/3/collection/%d", id)
	params := url.Values{}
	if req.GetLanguage() != "" {
		params.Set("language", req.GetLanguage())
	}

	var raw collectionDetailRaw
	if err := m.tmdbGet(ctx, endpoint, params, &raw); err != nil {
		return nil, err
	}

	resp := &metadatav1.GetCollectionResponse{
		Id:           raw.ID,
		Name:         raw.Name,
		Overview:     raw.Overview,
		PosterPath:   raw.PosterPath,
		BackdropPath: raw.BackdropPath,
	}
	for _, p := range raw.Parts {
		mt := metadatav1.MediaType_MEDIA_TYPE_MOVIE
		if p.MediaType == "tv" {
			mt = metadatav1.MediaType_MEDIA_TYPE_TV
		}
		resp.Parts = append(resp.Parts, &metadatav1.CollectionPart{
			Id:            p.ID,
			Title:         p.Title,
			OriginalTitle: p.OriginalTitle,
			Overview:      p.Overview,
			PosterPath:    p.PosterPath,
			BackdropPath:  p.BackdropPath,
			ReleaseDate:   p.ReleaseDate,
			VoteAverage:   p.VoteAverage,
			MediaType:     mt,
		})
	}
	return resp, nil
}

type collectionDetailRaw struct {
	ID           int32               `json:"id"`
	Name         string              `json:"name"`
	Overview     string              `json:"overview"`
	PosterPath   string              `json:"poster_path"`
	BackdropPath string              `json:"backdrop_path"`
	Parts        []collectionPartRaw `json:"parts"`
}

type collectionPartRaw struct {
	ID            int32   `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	ReleaseDate   string  `json:"release_date"`
	VoteAverage   float64 `json:"vote_average"`
	MediaType     string  `json:"media_type"`
}

func (m *Module) GetConfiguration(ctx context.Context, req *metadatav1.GetConfigurationRequest) (*metadatav1.GetConfigurationResponse, error) {
	cfg, err := m.getConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &metadatav1.GetConfigurationResponse{
		BaseUrl:       cfg.Images.BaseURL,
		SecureBaseUrl: cfg.Images.SecureBaseURL,
		PosterSizes:   cfg.Images.PosterSizes,
		BackdropSizes: cfg.Images.BackdropSizes,
		LogoSizes:     cfg.Images.LogoSizes,
		ProfileSizes:  cfg.Images.ProfileSizes,
		StillSizes:    cfg.Images.StillSizes,
	}, nil
}

// ── Trending & Discover ─────────────────────────────────────────

func (m *Module) ListTrending(ctx context.Context, req *metadatav1.ListTrendingRequest) (*metadatav1.ListTrendingResponse, error) {
	mediaType := "all"
	switch req.GetMediaType() {
	case metadatav1.TrendingMediaType_TRENDING_MEDIA_TYPE_MOVIE:
		mediaType = "movie"
	case metadatav1.TrendingMediaType_TRENDING_MEDIA_TYPE_TV:
		mediaType = "tv"
	}
	timeWindow := "week"
	switch req.GetTimeWindow() {
	case metadatav1.TrendingTimeWindow_TRENDING_TIME_WINDOW_DAY:
		timeWindow = "day"
	}

	endpoint := fmt.Sprintf("/3/trending/%s/%s", mediaType, timeWindow)
	params := url.Values{}
	params.Set("page", strconv.Itoa(int(req.GetPage())))
	if req.GetPage() <= 0 {
		params.Set("page", "1")
	}
	if req.GetLanguage() != "" {
		params.Set("language", req.GetLanguage())
	}

	var raw struct {
		Results      []json.RawMessage `json:"results"`
		TotalResults int               `json:"total_results"`
		TotalPages   int               `json:"total_pages"`
		Page         int               `json:"page"`
	}
	if err := m.tmdbGet(ctx, endpoint, params, &raw); err != nil {
		return nil, err
	}

	results := make([]*metadatav1.SearchResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		sr := m.parseSearchResult(r)
		if sr != nil {
			results = append(results, sr)
		}
	}

	return &metadatav1.ListTrendingResponse{
		Results:      results,
		TotalResults: int32(raw.TotalResults),
		TotalPages:   int32(raw.TotalPages),
		Page:         int32(raw.Page),
	}, nil
}

func (m *Module) FindByExternalID(ctx context.Context, req *metadatav1.FindByExternalIDRequest) (*metadatav1.FindByExternalIDResponse, error) {
	extID := strings.TrimSpace(req.GetExternalId())
	if extID == "" {
		return nil, fmt.Errorf("external_id is required")
	}
	source := strings.TrimSpace(req.GetExternalSource())
	if source == "" {
		source = "imdb_id"
	}

	params := url.Values{}
	params.Set("external_source", source)
	if req.GetLanguage() != "" {
		params.Set("language", req.GetLanguage())
	}

	endpoint := fmt.Sprintf("/3/find/%s", url.PathEscape(extID))
	var raw struct {
		MovieResults []json.RawMessage `json:"movie_results"`
		TVResults    []json.RawMessage `json:"tv_results"`
	}
	if err := m.tmdbGet(ctx, endpoint, params, &raw); err != nil {
		return nil, err
	}

	results := make([]*metadatav1.SearchResult, 0, len(raw.MovieResults)+len(raw.TVResults))
	for _, r := range raw.MovieResults {
		sr := m.parseSearchResult(r)
		if sr == nil {
			continue
		}
		if sr.MediaType == metadatav1.MediaType_MEDIA_TYPE_UNSPECIFIED {
			sr.MediaType = metadatav1.MediaType_MEDIA_TYPE_MOVIE
		}
		results = append(results, sr)
	}
	for _, r := range raw.TVResults {
		sr := m.parseSearchResult(r)
		if sr == nil {
			continue
		}
		if sr.MediaType == metadatav1.MediaType_MEDIA_TYPE_UNSPECIFIED {
			sr.MediaType = metadatav1.MediaType_MEDIA_TYPE_TV
		}
		results = append(results, sr)
	}

	return &metadatav1.FindByExternalIDResponse{Results: results}, nil
}

func (m *Module) GetAlternativeTitles(ctx context.Context, req *metadatav1.GetAlternativeTitlesRequest) (*metadatav1.GetAlternativeTitlesResponse, error) {
	id := req.GetTmdbId()
	if id == 0 {
		return nil, fmt.Errorf("tmdb_id required")
	}

	var endpoint string
	switch req.GetType() {
	case metadatav1.MediaType_MEDIA_TYPE_TV:
		endpoint = fmt.Sprintf("/3/tv/%d/alternative_titles", id)
	default:
		endpoint = fmt.Sprintf("/3/movie/%d/alternative_titles", id)
	}

	var raw struct {
		ID     int32 `json:"id"`
		Titles []struct {
			ISO   string `json:"iso_3166_1"`
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"titles"`
		Results []struct {
			ISO   string `json:"iso_3166_1"`
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"results"`
	}
	if err := m.tmdbGet(ctx, endpoint, nil, &raw); err != nil {
		return nil, err
	}

	type altEntry struct {
		ISO   string
		Title string
		Type  string
	}
	entries := make([]altEntry, 0, len(raw.Titles)+len(raw.Results))
	for _, e := range raw.Titles {
		entries = append(entries, altEntry{ISO: e.ISO, Title: e.Title, Type: e.Type})
	}
	if len(entries) == 0 {
		for _, e := range raw.Results {
			entries = append(entries, altEntry{ISO: e.ISO, Title: e.Title, Type: e.Type})
		}
	}

	out := make([]*metadatav1.AlternativeTitle, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		title := strings.TrimSpace(e.Title)
		if title == "" {
			continue
		}
		key := strings.ToLower(title)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		typ := e.Type
		if typ == "" {
			typ = e.ISO
		} else if e.ISO != "" {
			typ = e.ISO + ":" + typ
		}
		out = append(out, &metadatav1.AlternativeTitle{
			Title: title,
			Type:  typ,
		})
	}

	return &metadatav1.GetAlternativeTitlesResponse{
		TmdbId: id,
		Titles: out,
	}, nil
}

func (m *Module) ListPopular(ctx context.Context, req *metadatav1.ListPopularRequest) (*metadatav1.ListPopularResponse, error) {
	endpoint := "/3/movie/popular"
	switch req.GetType() {
	case metadatav1.MediaType_MEDIA_TYPE_TV:
		endpoint = "/3/tv/popular"
	default:
		endpoint = "/3/movie/popular"
	}

	params := url.Values{}
	params.Set("page", strconv.Itoa(int(req.GetPage())))
	if req.GetPage() <= 0 {
		params.Set("page", "1")
	}
	if req.GetLanguage() != "" {
		params.Set("language", req.GetLanguage())
	}

	var raw struct {
		Results      []json.RawMessage `json:"results"`
		TotalResults int               `json:"total_results"`
		TotalPages   int               `json:"total_pages"`
		Page         int               `json:"page"`
	}
	if err := m.tmdbGet(ctx, endpoint, params, &raw); err != nil {
		return nil, err
	}

	results := make([]*metadatav1.SearchResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		sr := m.parseSearchResult(r)
		if sr != nil {
			results = append(results, sr)
		}
	}

	return &metadatav1.ListPopularResponse{
		Results:      results,
		TotalResults: int32(raw.TotalResults),
		TotalPages:   int32(raw.TotalPages),
		Page:         int32(raw.Page),
	}, nil
}

func (m *Module) tmdbGet(ctx context.Context, endpoint string, params url.Values, dest any) error {
	if params == nil {
		params = url.Values{}
	}
	if m.fixtureMode() {
		return m.fixtureGet(endpoint, params, dest)
	}
	if m.apiKey == "" {
		return fmt.Errorf("TMDB API key not configured")
	}
	key := cacheKey(endpoint, params)

	if body, ok := m.cache.get(key); ok {
		if err := json.Unmarshal(body, dest); err != nil {
			return fmt.Errorf("decode tmdb response: %w", err)
		}
		return nil
	}

	body, err := m.inflight.do(key, func() ([]byte, error) {
		if cached, ok := m.cache.get(key); ok {
			return cached, nil
		}
		return m.tmdbFetch(ctx, endpoint, params, key)
	})
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode tmdb response: %w", err)
	}
	return nil
}

func (m *Module) tmdbFetch(ctx context.Context, endpoint string, params url.Values, key string) ([]byte, error) {
	q := cloneValues(params)
	q.Set("api_key", m.apiKey)
	u := m.baseURL + endpoint + "?" + q.Encode()

	var retryAfterSec int
	for attempt := 0; ; attempt++ {
		if err := m.limiter.wait(ctx); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		resp, err := m.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("tmdb request: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfterSec = parseRetryAfter(resp.Header.Get("Retry-After"))
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if attempt >= 3 {
				msg := fmt.Sprintf("rate_limit_exceeded: TMDB API rate limit reached. Retry in %d seconds.", retryAfterSec)
				return nil, status.Error(codes.ResourceExhausted, msg)
			}
			wait := time.Duration(retryAfterSec) * time.Second
			if wait > 10*time.Second {
				wait = 10 * time.Second
			}
			if wait <= 0 {
				wait = time.Millisecond
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read tmdb response: %w", err)
		}

		if resp.StatusCode == http.StatusNotFound {
			return nil, status.Error(codes.NotFound, "tmdb returned 404 Not Found")
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("tmdb returned %s", resp.Status)
		}

		m.cache.set(key, body, m.cache.ttlFor(endpoint))
		return body, nil
	}
}


func cloneValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vals := range v {
		out[k] = append([]string(nil), vals...)
	}
	return out
}

func parseRetryAfter(h string) int {
	if h == "" {
		return 1
	}
	n, err := strconv.Atoi(strings.TrimSpace(h))
	if err != nil || n < 0 {
		return 1
	}
	return n
}

func (m *Module) getConfig(ctx context.Context) (*tmdbConfig, error) {
	var cfg tmdbConfig
	if err := m.tmdbGet(ctx, "/3/configuration", nil, &cfg); err != nil {
		return nil, fmt.Errorf("fetch tmdb config: %w", err)
	}
	out := &cfg
	m.mu.Lock()
	m.lastConfig = out
	m.mu.Unlock()
	return out, nil
}

func (m *Module) getCachedConfig() *tmdbConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastConfig
}

func (m *Module) imageURL(path string, cfg *tmdbConfig) string {
	if path == "" || cfg == nil {
		return ""
	}
	return cfg.Images.SecureBaseURL + "original" + path
}

func (m *Module) parseSearchResult(raw json.RawMessage) *metadatav1.SearchResult {
	var base struct {
		ID               int     `json:"id"`
		Overview         string  `json:"overview"`
		PosterPath       string  `json:"poster_path"`
		BackdropPath     string  `json:"backdrop_path"`
		VoteAverage      float64 `json:"vote_average"`
		VoteCount        int     `json:"vote_count"`
		Popularity       float64 `json:"popularity"`
		OriginalLanguage string  `json:"original_language"`
		GenreIDs         []int32 `json:"genre_ids"`
		MediaType        string  `json:"media_type"`
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		return nil
	}

	var movie struct {
		Title         string `json:"title"`
		OriginalTitle string `json:"original_title"`
		ReleaseDate   string `json:"release_date"`
	}
	var tv struct {
		Name         string `json:"name"`
		OriginalName string `json:"original_name"`
		FirstAirDate string `json:"first_air_date"`
	}
	json.Unmarshal(raw, &movie)
	json.Unmarshal(raw, &tv)

	result := &metadatav1.SearchResult{
		Id:               int32(base.ID),
		Overview:         base.Overview,
		PosterPath:       base.PosterPath,
		BackdropPath:     base.BackdropPath,
		VoteAverage:      base.VoteAverage,
		VoteCount:        int32(base.VoteCount),
		Popularity:       base.Popularity,
		OriginalLanguage: base.OriginalLanguage,
		GenreIds:         base.GenreIDs,
		Title:            movie.Title,
		OriginalTitle:    movie.OriginalTitle,
		ReleaseDate:      movie.ReleaseDate,
		Name:             tv.Name,
		FirstAirDate:     tv.FirstAirDate,
	}

	switch base.MediaType {
	case "movie":
		result.MediaType = metadatav1.MediaType_MEDIA_TYPE_MOVIE
	case "tv":
		result.MediaType = metadatav1.MediaType_MEDIA_TYPE_TV
	}

	return result
}

type movieDetailRaw struct {
	Adult            bool           `json:"adult"`
	Budget           int64          `json:"budget"`
	Homepage         string         `json:"homepage"`
	ID               int            `json:"id"`
	IMDBID           string         `json:"imdb_id"`
	OriginalLanguage string         `json:"original_language"`
	OriginalTitle    string         `json:"original_title"`
	Overview         string         `json:"overview"`
	Popularity       float64        `json:"popularity"`
	PosterPath       string         `json:"poster_path"`
	BackdropPath     string         `json:"backdrop_path"`
	ReleaseDate      string         `json:"release_date"`
	Revenue          int64          `json:"revenue"`
	Runtime          int            `json:"runtime"`
	Status           string         `json:"status"`
	Tagline          string         `json:"tagline"`
	Title            string         `json:"title"`
	VoteAverage      float64        `json:"vote_average"`
	VoteCount        int            `json:"vote_count"`
	Genres           []genreRaw     `json:"genres"`
	ProductionCos    []companyRaw   `json:"production_companies"`
	SpokenLangs      []languageRaw  `json:"spoken_languages"`
	BelongsTo        *collectionRaw `json:"belongs_to_collection"`
	Videos           *videosAppendRaw `json:"videos"`
}

type genreRaw struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type companyRaw struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

type languageRaw struct {
	ISO6391 string `json:"iso_639_1"`
	Name    string `json:"name"`
}

type videosAppendRaw struct {
	Results []videoRaw `json:"results"`
}

type videoRaw struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Site string `json:"site"`
	Type string `json:"type"`
}

type collectionRaw struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
}

func (r *movieDetailRaw) toProto(cfg *tmdbConfig) *metadatav1.GetMovieDetailsResponse {
	resp := &metadatav1.GetMovieDetailsResponse{
		Id:               int32(r.ID),
		Title:            r.Title,
		OriginalTitle:    r.OriginalTitle,
		Overview:         r.Overview,
		Tagline:          r.Tagline,
		PosterPath:       r.PosterPath,
		BackdropPath:     r.BackdropPath,
		ReleaseDate:      r.ReleaseDate,
		Runtime:          int32(r.Runtime),
		VoteAverage:      r.VoteAverage,
		VoteCount:        int32(r.VoteCount),
		Popularity:       r.Popularity,
		Status:           r.Status,
		OriginalLanguage: r.OriginalLanguage,
		Adult:            r.Adult,
		Budget:           r.Budget,
		Revenue:          r.Revenue,
		Homepage:         r.Homepage,
		ImdbId:           r.IMDBID,
	}
	if cfg != nil {
		resp.PosterUrl = cfg.Images.SecureBaseURL + "original" + r.PosterPath
		resp.BackdropUrl = cfg.Images.SecureBaseURL + "original" + r.BackdropPath
	}
	for _, g := range r.Genres {
		resp.Genres = append(resp.Genres, &metadatav1.Genre{Id: int32(g.ID), Name: g.Name})
	}
	for _, c := range r.ProductionCos {
		resp.ProductionCompanies = append(resp.ProductionCompanies, &metadatav1.ProductionCompany{
			Id: int32(c.ID), Name: c.Name, LogoPath: c.LogoPath, OriginCountry: c.OriginCountry,
		})
	}
	for _, l := range r.SpokenLangs {
		resp.SpokenLanguages = append(resp.SpokenLanguages, &metadatav1.SpokenLanguage{Iso_639_1: l.ISO6391, Name: l.Name})
	}
	if r.BelongsTo != nil {
		resp.BelongsToCollection = &metadatav1.Collection{
			Id: int32(r.BelongsTo.ID), Name: r.BelongsTo.Name,
			PosterPath: r.BelongsTo.PosterPath, BackdropPath: r.BelongsTo.BackdropPath,
		}
	}
	resp.Videos = mapVideos(r.Videos)
	return resp
}

type tvDetailRaw struct {
	ID               int           `json:"id"`
	Name             string        `json:"name"`
	OriginalName     string        `json:"original_name"`
	Overview         string        `json:"overview"`
	Tagline          string        `json:"tagline"`
	PosterPath       string        `json:"poster_path"`
	BackdropPath     string        `json:"backdrop_path"`
	FirstAirDate     string        `json:"first_air_date"`
	LastAirDate      string        `json:"last_air_date"`
	NumSeasons       int           `json:"number_of_seasons"`
	NumEpisodes      int           `json:"number_of_episodes"`
	VoteAverage      float64       `json:"vote_average"`
	VoteCount        int           `json:"vote_count"`
	Popularity       float64       `json:"popularity"`
	Status           string        `json:"status"`
	OriginalLanguage string        `json:"original_language"`
	Homepage         string        `json:"homepage"`
	InProduction     bool          `json:"in_production"`
	Genres           []genreRaw    `json:"genres"`
	Seasons          []seasonRaw   `json:"seasons"`
	ProductionCos    []companyRaw  `json:"production_companies"`
	SpokenLangs      []languageRaw `json:"spoken_languages"`
	CreatedBy        []creatorRaw  `json:"created_by"`
	Networks         []networkRaw  `json:"networks"`
	OriginCountries  []string      `json:"origin_country"`
	Videos           *videosAppendRaw `json:"videos"`
}

type seasonRaw struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	AirDate      string  `json:"air_date"`
	SeasonNumber int     `json:"season_number"`
	EpisodeCount int     `json:"episode_count"`
	VoteAverage  float64 `json:"vote_average"`
}

type seasonDetailRaw struct {
	ID           int          `json:"id"`
	Name         string       `json:"name"`
	Overview     string       `json:"overview"`
	PosterPath   string       `json:"poster_path"`
	AirDate      string       `json:"air_date"`
	SeasonNumber int          `json:"season_number"`
	VoteAverage  float64      `json:"vote_average"`
	Episodes     []episodeRaw `json:"episodes"`
}

type episodeRaw struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Overview       string  `json:"overview"`
	AirDate        string  `json:"air_date"`
	EpisodeNumber  int     `json:"episode_number"`
	SeasonNumber   int     `json:"season_number"`
	StillPath      string  `json:"still_path"`
	Runtime        int     `json:"runtime"`
	VoteAverage    float64 `json:"vote_average"`
	VoteCount      int     `json:"vote_count"`
	ProductionCode string  `json:"production_code"`
}

func (r *seasonDetailRaw) toProto() *metadatav1.GetSeasonDetailsResponse {
	resp := &metadatav1.GetSeasonDetailsResponse{
		Id:           int32(r.ID),
		Name:         r.Name,
		Overview:     r.Overview,
		PosterPath:   r.PosterPath,
		AirDate:      r.AirDate,
		SeasonNumber: int32(r.SeasonNumber),
		VoteAverage:  r.VoteAverage,
	}
	for _, e := range r.Episodes {
		resp.Episodes = append(resp.Episodes, &metadatav1.Episode{
			Id:             int32(e.ID),
			Name:           e.Name,
			Overview:       e.Overview,
			AirDate:        e.AirDate,
			EpisodeNumber:  int32(e.EpisodeNumber),
			SeasonNumber:   int32(e.SeasonNumber),
			StillPath:      e.StillPath,
			Runtime:        int32(e.Runtime),
			VoteAverage:    e.VoteAverage,
			VoteCount:      int32(e.VoteCount),
			ProductionCode: e.ProductionCode,
		})
	}
	return resp
}

type creatorRaw struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ProfilePath string `json:"profile_path"`
}

type networkRaw struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

func (r *tvDetailRaw) toProto(cfg *tmdbConfig) *metadatav1.GetTVDetailsResponse {
	resp := &metadatav1.GetTVDetailsResponse{
		Id:               int32(r.ID),
		Name:             r.Name,
		OriginalName:     r.OriginalName,
		Overview:         r.Overview,
		Tagline:          r.Tagline,
		PosterPath:       r.PosterPath,
		BackdropPath:     r.BackdropPath,
		FirstAirDate:     r.FirstAirDate,
		LastAirDate:      r.LastAirDate,
		NumberOfSeasons:  int32(r.NumSeasons),
		NumberOfEpisodes: int32(r.NumEpisodes),
		VoteAverage:      r.VoteAverage,
		VoteCount:        int32(r.VoteCount),
		Popularity:       r.Popularity,
		Status:           r.Status,
		OriginalLanguage: r.OriginalLanguage,
		Homepage:         r.Homepage,
		InProduction:     strconv.FormatBool(r.InProduction),
		OriginCountry:    r.OriginCountries,
	}
	if cfg != nil {
		resp.PosterUrl = cfg.Images.SecureBaseURL + "original" + r.PosterPath
		resp.BackdropUrl = cfg.Images.SecureBaseURL + "original" + r.BackdropPath
	}
	for _, g := range r.Genres {
		resp.Genres = append(resp.Genres, &metadatav1.Genre{Id: int32(g.ID), Name: g.Name})
	}
	for _, s := range r.Seasons {
		resp.Seasons = append(resp.Seasons, &metadatav1.Season{
			Id: int32(s.ID), Name: s.Name, Overview: s.Overview,
			PosterPath: s.PosterPath, AirDate: s.AirDate,
			SeasonNumber: int32(s.SeasonNumber), EpisodeCount: int32(s.EpisodeCount),
			VoteAverage: s.VoteAverage,
		})
	}
	for _, c := range r.ProductionCos {
		resp.ProductionCompanies = append(resp.ProductionCompanies, &metadatav1.ProductionCompany{
			Id: int32(c.ID), Name: c.Name, LogoPath: c.LogoPath, OriginCountry: c.OriginCountry,
		})
	}
	for _, l := range r.SpokenLangs {
		resp.SpokenLanguages = append(resp.SpokenLanguages, &metadatav1.SpokenLanguage{Iso_639_1: l.ISO6391, Name: l.Name})
	}
	for _, c := range r.CreatedBy {
		resp.CreatedBy = append(resp.CreatedBy, &metadatav1.CreatedBy{Id: int32(c.ID), Name: c.Name, ProfilePath: c.ProfilePath})
	}
	for _, n := range r.Networks {
		resp.Networks = append(resp.Networks, &metadatav1.Network{Id: int32(n.ID), Name: n.Name, LogoPath: n.LogoPath, OriginCountry: n.OriginCountry})
	}
	resp.Videos = mapVideos(r.Videos)
	return resp
}

func mapVideos(raw *videosAppendRaw) []*metadatav1.Video {
	if raw == nil {
		return nil
	}
	out := make([]*metadatav1.Video, 0, len(raw.Results))
	for _, v := range raw.Results {
		out = append(out, &metadatav1.Video{
			Id: v.ID, Key: v.Key, Name: v.Name, Site: v.Site, Type: v.Type,
		})
	}
	return out
}

var _ contracts.Module = (*Module)(nil)
