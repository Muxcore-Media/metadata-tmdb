package internal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	metadatav1 "github.com/Muxcore-Media/contracts-metadata/muxcore/metadata/v1"
)

// All certification tests are offline (ADR-0008): TMDB is either the built-in
// fixture corpus or a loopback httptest server serving testdata/certification.

func readCertFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "certification", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// certBackend is a loopback stand-in for TMDB. It counts upstream hits per path
// and records the append_to_response value of each request.
type certBackend struct {
	srv     *httptest.Server
	mu      sync.Mutex
	hits    map[string]int
	appends map[string][]string
}

func newCertBackend(t *testing.T) *certBackend {
	t.Helper()
	files := map[string][]byte{
		"/3/movie/603":     readCertFixture(t, "movie_603.json"),
		"/3/movie/604":     readCertFixture(t, "movie_604_no_release_dates.json"),
		"/3/movie/605":     readCertFixture(t, "movie_605_malformed_release_dates.json"),
		"/3/tv/1399":       readCertFixture(t, "tv_1399.json"),
		"/3/tv/1400":       readCertFixture(t, "tv_1400_no_content_ratings.json"),
		"/3/configuration": []byte(`{"images":{"base_url":"http://img.test/","secure_base_url":"https://img.test/"}}`),
	}
	b := &certBackend{hits: map[string]int{}, appends: map[string][]string{}}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		b.hits[r.URL.Path]++
		b.appends[r.URL.Path] = append(b.appends[r.URL.Path], r.URL.Query().Get("append_to_response"))
		b.mu.Unlock()
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(b.srv.Close)
	return b
}

func (b *certBackend) hitCount(path string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.hits[path]
}

func (b *certBackend) lastAppend(path string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	a := b.appends[path]
	if len(a) == 0 {
		return ""
	}
	return a[len(a)-1]
}

// noNetworkTransport fails every outbound request and counts attempts.
type noNetworkTransport struct{ calls atomic.Int64 }

func (n *noNetworkTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	n.calls.Add(1)
	return nil, errors.New("outbound HTTP attempted in fixture mode: " + r.URL.Host)
}

func TestParseCertificationCountry(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "US", false},
		{"   ", "US", false},
		{"US", "US", false},
		{"gb", "GB", false},
		{" de ", "DE", false},
		{"USA", "", true},
		{"U", "", true},
		{"U1", "", true},
		{"\u00dcS", "", true},
		{"U S", "", true},
	}
	for _, c := range cases {
		got, err := parseCertificationCountry(c.in)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("parseCertificationCountry(%q) = %q, %v; want %q, err=%v", c.in, got, err, c.want, c.wantErr)
		}
	}
}

func TestCertificationCountryConfig(t *testing.T) {
	t.Setenv("TMDB_CERTIFICATION_COUNTRY", "")
	if got := NewModule(Config{}).certificationCountry(); got != "US" {
		t.Errorf("default country = %q, want US", got)
	}
	if got := NewModule(Config{CertificationCountry: "de"}).certificationCountry(); got != "DE" {
		t.Errorf("Config country = %q, want DE", got)
	}
	t.Setenv("TMDB_CERTIFICATION_COUNTRY", "gb")
	if got := NewModule(Config{CertificationCountry: "DE"}).certificationCountry(); got != "GB" {
		t.Errorf("env must override Config: got %q, want GB", got)
	}
	// Invalid startup value: never guess another country; certification is disabled.
	t.Setenv("TMDB_CERTIFICATION_COUNTRY", "UK1")
	if got := NewModule(Config{}).certificationCountry(); got != "" {
		t.Errorf("invalid env country = %q, want disabled (\"\")", got)
	}
}

func TestSanitizeCertification(t *testing.T) {
	cases := []struct{ in, want string }{
		{"R", "R"},
		{"  PG-13 ", "PG-13"},
		{"TV-MA\n", "TV-MA"},
		{"PG\x00-13", "PG-13"},
		{"\u202eR", "R"},       // bidi override (format char) removed
		{"R\u200b", "R"},       // zero-width space removed
		{"\x1b[31mR", "[31mR"}, // ESC removed; the rest is printable text left for the media module to reject
		{"\xffR", "R"},         // invalid UTF-8 removed
		{"\t\r\n", ""},
		{"", ""},
		{"0123456789abcdef", "0123456789abcdef"}, // 16 bytes: kept
		{"0123456789abcdefg", ""},                // 17 bytes: dropped, not truncated
		{"Rated R for strong violence", ""},
	}
	for _, c := range cases {
		if got := sanitizeCertification(c.in); got != c.want {
			t.Errorf("sanitizeCertification(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func movieReleaseDates(t *testing.T, file string) json.RawMessage {
	t.Helper()
	var raw movieDetailRaw
	if err := json.Unmarshal(readCertFixture(t, file), &raw); err != nil {
		t.Fatal(err)
	}
	return raw.ReleaseDates
}

func TestSelectMovieCertification(t *testing.T) {
	full := movieReleaseDates(t, "movie_603.json")
	cases := []struct {
		name, country, want string
		raw                 json.RawMessage
	}{
		// US: premiere NR is listed first and theatrical is empty; digital R wins.
		{"US skips empty theatrical, prefers digital over premiere", "US", "R", full},
		{"GB theatrical", "GB", "15", full},
		{"DE theatrical", "DE", "16", full},
		{"country match is case-insensitive", "gb", "15", full},
		{"FR has only empty or blank entries", "FR", "", full},
		{"country absent", "JP", "", full},
		{"disabled country", "", "", full},
		{"block missing", "US", "", nil},
		{"block null", "US", "", json.RawMessage(`null`)},
		{"block malformed", "US", "", json.RawMessage(`{"results":"x"}`)},
		{"theatrical beats earlier physical", "US", "PG", json.RawMessage(`{"results":[{"iso_3166_1":"US","release_dates":[
			{"certification":"NR","type":5,"release_date":"1990-01-01T00:00:00.000Z"},
			{"certification":"PG","type":3,"release_date":"1995-01-01T00:00:00.000Z"}]}]}`)},
		{"limited theatrical beats digital", "US", "PG-13", json.RawMessage(`{"results":[{"iso_3166_1":"US","release_dates":[
			{"certification":"R","type":4},{"certification":"PG-13","type":2}]}]}`)},
		{"same type: earliest date wins", "US", "PG-13", json.RawMessage(`{"results":[{"iso_3166_1":"US","release_dates":[
			{"certification":"R","type":3,"release_date":"2001-02-01T00:00:00.000Z"},
			{"certification":"PG-13","type":3,"release_date":"2001-01-01T00:00:00.000Z"}]}]}`)},
		{"same type: empty date sorts last", "US", "R", json.RawMessage(`{"results":[{"iso_3166_1":"US","release_dates":[
			{"certification":"PG","type":3,"release_date":""},
			{"certification":"R","type":3,"release_date":"2001-01-01T00:00:00.000Z"}]}]}`)},
		{"same type and date: first in TMDB order", "US", "R", json.RawMessage(`{"results":[{"iso_3166_1":"US","release_dates":[
			{"certification":"R","type":3,"release_date":"2001-01-01T00:00:00.000Z"},
			{"certification":"PG","type":3,"release_date":"2001-01-01T00:00:00.000Z"}]}]}`)},
		{"unknown type used only as last resort", "US", "R", json.RawMessage(`{"results":[{"iso_3166_1":"US","release_dates":[
			{"certification":"X","type":9},{"certification":"R","type":6}]}]}`)},
		{"control chars stripped", "US", "R", json.RawMessage(`{"results":[{"iso_3166_1":"US","release_dates":[
			{"certification":"\u0000R\n","type":3}]}]}`)},
		{"over-long free text dropped, next entry used", "US", "PG", json.RawMessage(`{"results":[{"iso_3166_1":"US","release_dates":[
			{"certification":"Rated R for violence","type":3},{"certification":"PG","type":4}]}]}`)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cert, cc := selectMovieCertification(c.raw, c.country)
			if cert != c.want {
				t.Fatalf("certification = %q, want %q", cert, c.want)
			}
			wantCC := ""
			if c.want != "" {
				wantCC = c.country
			}
			if cc != wantCC {
				t.Fatalf("certification_country = %q, want %q", cc, wantCC)
			}
		})
	}
}

func TestSelectTVCertification(t *testing.T) {
	var raw tvDetailRaw
	if err := json.Unmarshal(readCertFixture(t, "tv_1399.json"), &raw); err != nil {
		t.Fatal(err)
	}
	full := raw.ContentRatings
	cases := []struct {
		name, country, want string
		raw                 json.RawMessage
	}{
		{"US skips empty entry", "US", "TV-MA", full},
		{"GB", "GB", "15", full},
		{"DE", "DE", "16", full},
		{"FR empty only", "FR", "", full},
		{"absent", "JP", "", full},
		{"disabled", "", "", full},
		{"missing", "US", "", nil},
		{"malformed", "US", "", json.RawMessage(`[1,2]`)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cert, cc := selectTVCertification(c.raw, c.country)
			if cert != c.want {
				t.Fatalf("certification = %q, want %q", cert, c.want)
			}
			if (cert == "") != (cc == "") {
				t.Fatalf("certification_country %q must be set iff certification is", cc)
			}
		})
	}
}

func TestCertificationDetailsByCountry(t *testing.T) {
	t.Setenv("TMDB_CERTIFICATION_COUNTRY", "")
	b := newCertBackend(t)
	ctx := context.Background()
	for _, c := range []struct{ country, movie, tv string }{
		{"US", "R", "TV-MA"},
		{"GB", "15", "15"},
		{"DE", "16", "16"},
		{"FR", "", ""},
		{"JP", "", ""},
	} {
		m := NewModule(Config{APIKey: "k", BaseURL: b.srv.URL, CertificationCountry: c.country})
		mv, err := m.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{Id: 603})
		if err != nil {
			t.Fatalf("%s movie: %v", c.country, err)
		}
		tv, err := m.GetTVDetails(ctx, &metadatav1.GetTVDetailsRequest{Id: 1399})
		if err != nil {
			t.Fatalf("%s tv: %v", c.country, err)
		}
		wantCC := func(cert string) string {
			if cert == "" {
				return ""
			}
			return c.country
		}
		if mv.GetCertification() != c.movie || mv.GetCertificationCountry() != wantCC(c.movie) {
			t.Errorf("%s movie = (%q,%q), want (%q,%q)", c.country, mv.GetCertification(), mv.GetCertificationCountry(), c.movie, wantCC(c.movie))
		}
		if tv.GetCertification() != c.tv || tv.GetCertificationCountry() != wantCC(c.tv) {
			t.Errorf("%s tv = (%q,%q), want (%q,%q)", c.country, tv.GetCertification(), tv.GetCertificationCountry(), c.tv, wantCC(c.tv))
		}
	}
	if got := b.lastAppend("/3/movie/603"); got != "release_dates" {
		t.Errorf("movie append_to_response = %q, want release_dates (single request)", got)
	}
	if got := b.lastAppend("/3/tv/1399"); got != "content_ratings" {
		t.Errorf("tv append_to_response = %q, want content_ratings (single request)", got)
	}
}

func TestCertificationKeepsCallerAppends(t *testing.T) {
	t.Setenv("TMDB_CERTIFICATION_COUNTRY", "")
	b := newCertBackend(t)
	m := NewModule(Config{APIKey: "k", BaseURL: b.srv.URL})
	ctx := context.Background()
	req := &metadatav1.GetMovieDetailsRequest{Id: 603, AppendToResponse: []string{"videos", "credits"}}
	if _, err := m.GetMovieDetails(ctx, req); err != nil {
		t.Fatal(err)
	}
	if got := b.lastAppend("/3/movie/603"); got != "videos,credits,release_dates" {
		t.Errorf("append_to_response = %q", got)
	}
	if len(req.AppendToResponse) != 2 {
		t.Errorf("caller request mutated: %v", req.AppendToResponse)
	}
	// Already requested by the caller: not duplicated.
	if _, err := m.GetTVDetails(ctx, &metadatav1.GetTVDetailsRequest{Id: 1399, AppendToResponse: []string{"Content_Ratings"}}); err != nil {
		t.Fatal(err)
	}
	if got := b.lastAppend("/3/tv/1399"); got != "Content_Ratings" {
		t.Errorf("tv append_to_response = %q", got)
	}
}

func TestCertificationMissingOrMalformedIsEmptyNotError(t *testing.T) {
	t.Setenv("TMDB_CERTIFICATION_COUNTRY", "")
	b := newCertBackend(t)
	m := NewModule(Config{APIKey: "k", BaseURL: b.srv.URL})
	ctx := context.Background()
	for _, id := range []int32{604, 605} {
		mv, err := m.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{Id: id})
		if err != nil {
			t.Fatalf("movie %d: %v", id, err)
		}
		if mv.GetTitle() == "" || mv.GetCertification() != "" || mv.GetCertificationCountry() != "" {
			t.Errorf("movie %d = title %q cert (%q,%q); want title and empty cert", id, mv.GetTitle(), mv.GetCertification(), mv.GetCertificationCountry())
		}
	}
	tv, err := m.GetTVDetails(ctx, &metadatav1.GetTVDetailsRequest{Id: 1400})
	if err != nil {
		t.Fatal(err)
	}
	if tv.GetName() == "" || tv.GetCertification() != "" || tv.GetCertificationCountry() != "" {
		t.Errorf("tv 1400 = %q cert (%q,%q)", tv.GetName(), tv.GetCertification(), tv.GetCertificationCountry())
	}
}

func TestCertificationDisabledByInvalidCountry(t *testing.T) {
	t.Setenv("TMDB_CERTIFICATION_COUNTRY", "XYZ")
	b := newCertBackend(t)
	m := NewModule(Config{APIKey: "k", BaseURL: b.srv.URL})
	ctx := context.Background()
	mv, err := m.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{Id: 603})
	if err != nil {
		t.Fatal(err)
	}
	tv, err := m.GetTVDetails(ctx, &metadatav1.GetTVDetailsRequest{Id: 1399})
	if err != nil {
		t.Fatal(err)
	}
	if mv.GetCertification() != "" || tv.GetCertification() != "" || mv.GetCertificationCountry() != "" || tv.GetCertificationCountry() != "" {
		t.Errorf("disabled country must yield empty certification: movie %q tv %q", mv.GetCertification(), tv.GetCertification())
	}
	if a := b.lastAppend("/3/movie/603") + b.lastAppend("/3/tv/1399"); a != "" {
		t.Errorf("disabled country must not request certification blocks, got %q", a)
	}
}

// The cache stores raw TMDB bodies (all countries); the configured country is
// applied per request after the cache. Changing the country must change the
// answer without a new upstream request and without serving another country's value.
func TestCertificationCacheSeparatedByCountry(t *testing.T) {
	t.Setenv("TMDB_CERTIFICATION_COUNTRY", "")
	b := newCertBackend(t)
	m := NewModule(Config{APIKey: "k", BaseURL: b.srv.URL})
	ctx := context.Background()
	steps := []struct{ country, movie, tv string }{
		{"US", "R", "TV-MA"},
		{"GB", "15", "15"},
		{"DE", "16", "16"},
		{"FR", "", ""},
		{"US", "R", "TV-MA"},
	}
	for _, s := range steps {
		if err := m.UpdateSetting("certification_country", s.country); err != nil {
			t.Fatal(err)
		}
		mv, err := m.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{Id: 603})
		if err != nil {
			t.Fatal(err)
		}
		tv, err := m.GetTVDetails(ctx, &metadatav1.GetTVDetailsRequest{Id: 1399})
		if err != nil {
			t.Fatal(err)
		}
		if mv.GetCertification() != s.movie || tv.GetCertification() != s.tv {
			t.Fatalf("country %s: movie %q tv %q, want %q %q (cache mixed countries?)", s.country, mv.GetCertification(), tv.GetCertification(), s.movie, s.tv)
		}
		if s.movie != "" && (mv.GetCertificationCountry() != s.country || tv.GetCertificationCountry() != s.country) {
			t.Fatalf("country %s: certification_country movie %q tv %q", s.country, mv.GetCertificationCountry(), tv.GetCertificationCountry())
		}
	}
	if b.hitCount("/3/movie/603") != 1 || b.hitCount("/3/tv/1399") != 1 {
		t.Errorf("upstream hits movie=%d tv=%d, want 1 each (raw body cached once)", b.hitCount("/3/movie/603"), b.hitCount("/3/tv/1399"))
	}
	// Invalid runtime update is rejected and keeps the current country.
	if err := m.UpdateSetting("certification_country", "Germany"); err == nil {
		t.Fatal("expected error for invalid certification_country")
	}
	if got := m.certificationCountry(); got != "US" {
		t.Errorf("country after rejected update = %q, want US", got)
	}
	if err := m.UpdateSetting("TMDB_CERTIFICATION_COUNTRY", ""); err != nil || m.certificationCountry() != "US" {
		t.Errorf("empty update must reset to US: %q %v", m.certificationCountry(), err)
	}
	var found bool
	for _, d := range m.Settings() {
		if d.Key == "certification_country" {
			found = d.Value == "US" && d.Default == "US"
		}
	}
	if !found {
		t.Error("certification_country setting missing or wrong value")
	}
}

func TestCertificationFixtureModeNoNetwork(t *testing.T) {
	t.Setenv("TMDB_CERTIFICATION_COUNTRY", "")
	m := NewModule(Config{Fixture: true})
	nt := &noNetworkTransport{}
	m.client.Transport = nt
	ctx := context.Background()
	for _, c := range []struct{ country, movie, tv string }{
		{"US", "R", "TV-MA"},
		{"GB", "18", "15"},
		{"DE", "18", "16"},
		{"JP", "", ""},
	} {
		if err := m.UpdateSetting("certification_country", c.country); err != nil {
			t.Fatal(err)
		}
		mv, err := m.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{Id: 550, AppendToResponse: []string{"videos", "credits"}})
		if err != nil {
			t.Fatal(err)
		}
		tv, err := m.GetTVDetails(ctx, &metadatav1.GetTVDetailsRequest{Id: 1396})
		if err != nil {
			t.Fatal(err)
		}
		if mv.GetCertification() != c.movie || tv.GetCertification() != c.tv {
			t.Errorf("fixture %s: movie %q tv %q, want %q %q", c.country, mv.GetCertification(), tv.GetCertification(), c.movie, c.tv)
		}
	}
	if n := nt.calls.Load(); n != 0 {
		t.Fatalf("fixture mode made %d outbound HTTP requests, want 0", n)
	}
}

// wireStrings decodes the top-level length-delimited fields of a marshaled
// message, so the test checks the tags actually on the wire.
func wireStrings(t *testing.T, b []byte) map[protowire.Number]string {
	t.Helper()
	out := map[protowire.Number]string{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			t.Fatal(protowire.ParseError(n))
		}
		b = b[n:]
		if typ == protowire.BytesType {
			v, m := protowire.ConsumeBytes(b)
			if m < 0 {
				t.Fatal(protowire.ParseError(m))
			}
			out[num] = string(v)
			b = b[m:]
			continue
		}
		m := protowire.ConsumeFieldValue(num, typ, b)
		if m < 0 {
			t.Fatal(protowire.ParseError(m))
		}
		b = b[m:]
	}
	return out
}

func startPlaintextModule(t *testing.T, cfg Config) (*Module, metadatav1.MetadataServiceClient) {
	t.Helper()
	t.Setenv("MUXCORE_DEV_TLS_SKIP", "true") // plaintext listener for this test (meshtls dev flag)
	cfg.GRPCAddr = "127.0.0.1:0"
	m := NewModule(cfg)
	ctx := context.Background()
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(ctx) })
	conn, err := grpc.NewClient(m.lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return m, metadatav1.NewMetadataServiceClient(conn)
}

// End to end through the registered gRPC service, fixture mode, no network.
// Also pins the wire tags (movie 28/29, TV 33/34) that metadata-tmdb's own
// proto/metadatav1 copy must decode identically.
func TestCertificationGRPCEndToEndFixture(t *testing.T) {
	t.Setenv("TMDB_CERTIFICATION_COUNTRY", "")
	m, client := startPlaintextModule(t, Config{Fixture: true})
	nt := &noNetworkTransport{}
	m.client.Transport = nt
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mv, err := client.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{Id: 550})
	if err != nil {
		t.Fatal(err)
	}
	if mv.GetCertification() != "R" || mv.GetCertificationCountry() != "US" {
		t.Errorf("movie over gRPC = (%q,%q), want (R,US)", mv.GetCertification(), mv.GetCertificationCountry())
	}
	tv, err := client.GetTVDetails(ctx, &metadatav1.GetTVDetailsRequest{Id: 1396})
	if err != nil {
		t.Fatal(err)
	}
	if tv.GetCertification() != "TV-MA" || tv.GetCertificationCountry() != "US" {
		t.Errorf("tv over gRPC = (%q,%q), want (TV-MA,US)", tv.GetCertification(), tv.GetCertificationCountry())
	}

	mb, err := proto.Marshal(mv)
	if err != nil {
		t.Fatal(err)
	}
	if w := wireStrings(t, mb); w[28] != "R" || w[29] != "US" {
		t.Errorf("movie wire tags 28/29 = %q/%q", w[28], w[29])
	}
	tb, err := proto.Marshal(tv)
	if err != nil {
		t.Fatal(err)
	}
	if w := wireStrings(t, tb); w[33] != "TV-MA" || w[34] != "US" {
		t.Errorf("tv wire tags 33/34 = %q/%q", w[33], w[34])
	}
	if n := nt.calls.Load(); n != 0 {
		t.Fatalf("fixture mode made %d outbound HTTP requests, want 0", n)
	}
}

// End to end through gRPC against the loopback backend, with the country taken
// from TMDB_CERTIFICATION_COUNTRY.
func TestCertificationGRPCEndToEndConfiguredCountry(t *testing.T) {
	b := newCertBackend(t)
	t.Setenv("TMDB_CERTIFICATION_COUNTRY", "gb")
	_, client := startPlaintextModule(t, Config{APIKey: "k", BaseURL: b.srv.URL})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mv, err := client.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{Id: 603})
	if err != nil {
		t.Fatal(err)
	}
	if mv.GetCertification() != "15" || mv.GetCertificationCountry() != "GB" {
		t.Errorf("movie = (%q,%q), want (15,GB)", mv.GetCertification(), mv.GetCertificationCountry())
	}
	tv, err := client.GetTVDetails(ctx, &metadatav1.GetTVDetailsRequest{Id: 1399})
	if err != nil {
		t.Fatal(err)
	}
	if tv.GetCertification() != "15" || tv.GetCertificationCountry() != "GB" {
		t.Errorf("tv = (%q,%q), want (15,GB)", tv.GetCertification(), tv.GetCertificationCountry())
	}
	none, err := client.GetMovieDetails(ctx, &metadatav1.GetMovieDetailsRequest{Id: 604})
	if err != nil {
		t.Fatalf("missing release_dates must not be an error: %v", err)
	}
	if none.GetCertification() != "" || none.GetCertificationCountry() != "" {
		t.Errorf("movie 604 = (%q,%q), want empty", none.GetCertification(), none.GetCertificationCountry())
	}
	if !strings.Contains(b.lastAppend("/3/movie/603"), "release_dates") {
		t.Error("movie request did not append release_dates")
	}
}
