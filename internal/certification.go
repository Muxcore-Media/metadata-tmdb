package internal

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TMDB certifications (ADR-0031 §2, roadmap T-M4-01 S4b).
//
// metadata-tmdb returns the raw TMDB certification for ONE configured country.
// It does not map or validate tokens against the rating ladder: the owning media
// modules do that, an operator rating always wins over TMDB, and an empty or
// unknown token is "unavailable" (never unrestricted). This file only selects
// and sanitizes.

const (
	defaultCertificationCountry = "US"
	// maxCertificationLen bounds TMDB free text; longer values are dropped (empty),
	// not truncated, so a cut-off string can never look like a valid token.
	maxCertificationLen = 16
)

// parseCertificationCountry normalizes an ISO 3166-1 alpha-2 code. Empty means
// the default ("US"). Case-insensitive; the result is uppercase. Anything other
// than exactly two ASCII letters is an error.
func parseCertificationCountry(v string) (string, error) {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v == "" {
		return defaultCertificationCountry, nil
	}
	if len(v) != 2 || v[0] < 'A' || v[0] > 'Z' || v[1] < 'A' || v[1] > 'Z' {
		return "", fmt.Errorf("certification country %q is not an ISO 3166-1 alpha-2 code (e.g. US, GB, DE)", v)
	}
	return v, nil
}

func (m *Module) certificationCountry() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.certCountry
}

// withAppend returns list plus name unless list already contains name
// (case-insensitive). The caller's slice is never modified.
func withAppend(list []string, name string) []string {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), name) {
			return list
		}
	}
	out := make([]string, 0, len(list)+1)
	out = append(out, list...)
	return append(out, name)
}

// sanitizeCertification trims the value and removes control, format and other
// non-printable runes (and invalid UTF-8). Values longer than
// maxCertificationLen bytes after cleaning are dropped.
func sanitizeCertification(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == utf8.RuneError || !unicode.IsPrint(r) {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if len(out) > maxCertificationLen {
		return ""
	}
	return out
}

type releaseDatesAppendRaw struct {
	Results []struct {
		Country      string `json:"iso_3166_1"`
		ReleaseDates []struct {
			Certification string `json:"certification"`
			ReleaseDate   string `json:"release_date"`
			Type          int    `json:"type"`
		} `json:"release_dates"`
	} `json:"results"`
}

type contentRatingsAppendRaw struct {
	Results []struct {
		Country string `json:"iso_3166_1"`
		Rating  string `json:"rating"`
	} `json:"results"`
}

// releaseTypeRank orders TMDB release types for certification selection:
// 3 Theatrical, 2 Theatrical (limited), 4 Digital, 5 Physical, 1 Premiere,
// 6 TV, then anything else. Lower is preferred.
func releaseTypeRank(t int) int {
	switch t {
	case 3:
		return 0
	case 2:
		return 1
	case 4:
		return 2
	case 5:
		return 3
	case 1:
		return 4
	case 6:
		return 5
	default:
		return 6
	}
}

// earlierDate reports whether TMDB ISO-8601 date a sorts before b; an empty
// date sorts last.
func earlierDate(a, b string) bool {
	if a == "" {
		return false
	}
	if b == "" {
		return true
	}
	return a < b
}

// selectMovieCertification picks the certification for country from a TMDB
// movie `release_dates` block. Deterministic rule:
//  1. only entries whose iso_3166_1 equals country (case-insensitive);
//  2. only entries whose sanitized certification is non-empty;
//  3. best release type by releaseTypeRank (theatrical, limited, digital,
//     physical, premiere, TV, other);
//  4. within one type, the earliest release_date; then the first in TMDB order.
//
// It returns ("", "") when country is empty, the block is missing or malformed,
// or no entry qualifies; never an error.
func selectMovieCertification(raw json.RawMessage, country string) (cert, certCountry string) {
	if country == "" || len(raw) == 0 {
		return "", ""
	}
	var block releaseDatesAppendRaw
	if err := json.Unmarshal(raw, &block); err != nil {
		return "", ""
	}
	found := false
	bestRank := 0
	bestDate := ""
	for _, c := range block.Results {
		if !strings.EqualFold(strings.TrimSpace(c.Country), country) {
			continue
		}
		for _, rd := range c.ReleaseDates {
			v := sanitizeCertification(rd.Certification)
			if v == "" {
				continue
			}
			rank := releaseTypeRank(rd.Type)
			if !found || rank < bestRank || (rank == bestRank && earlierDate(rd.ReleaseDate, bestDate)) {
				found, cert, bestRank, bestDate = true, v, rank, rd.ReleaseDate
			}
		}
	}
	if !found {
		return "", ""
	}
	return cert, country
}

// selectTVCertification picks the first non-empty sanitized rating whose
// iso_3166_1 equals country (case-insensitive) from a TMDB TV `content_ratings`
// block. Same empty/never-error contract as selectMovieCertification.
func selectTVCertification(raw json.RawMessage, country string) (cert, certCountry string) {
	if country == "" || len(raw) == 0 {
		return "", ""
	}
	var block contentRatingsAppendRaw
	if err := json.Unmarshal(raw, &block); err != nil {
		return "", ""
	}
	for _, c := range block.Results {
		if !strings.EqualFold(strings.TrimSpace(c.Country), country) {
			continue
		}
		if v := sanitizeCertification(c.Rating); v != "" {
			return v, country
		}
	}
	return "", ""
}
