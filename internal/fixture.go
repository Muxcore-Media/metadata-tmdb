package internal

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Offline corpus for TMDB_FIXTURE / TMDB_API_KEY=fixture (laptop demo + unit tests).
// Covers Fight Club (movie 550) and Breaking Bad (TV 1396) used by _mvp smoke.
// Details include release_dates / content_ratings (US R / TV-MA, GB, DE) so the
// certification path works offline; the payloads ignore append_to_response.
// No network is used while fixture mode is active.

const (
	fixtureConfigJSON = `{"images":{"base_url":"https://image.tmdb.org/t/p/","secure_base_url":"https://image.tmdb.org/t/p/","poster_sizes":["w92","w154","w185","w342","w500","w780","original"],"backdrop_sizes":["w300","w780","w1280","original"],"logo_sizes":["w45","w92","w154","w185","w300","w500","original"],"profile_sizes":["w45","w185","h632","original"],"still_sizes":["w92","w185","w300","original"]}}`

	fixtureFightClubSearch = `{"page":1,"total_pages":1,"total_results":1,"results":[{"id":550,"title":"Fight Club","original_title":"Fight Club","overview":"An insomniac office worker and a devil-may-care soap maker form an underground fight club that evolves into something much more.","poster_path":"/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg","backdrop_path":"/fCayJrkfRaCRCTh8GqLQLDJhzsF.jpg","release_date":"1999-10-15","vote_average":8.433,"vote_count":26280,"popularity":61.416,"original_language":"en","genre_ids":[18,53],"media_type":"movie","adult":false}]}`

	fixtureBreakingBadSearch = `{"page":1,"total_pages":1,"total_results":1,"results":[{"id":1396,"name":"Breaking Bad","original_name":"Breaking Bad","overview":"A high school chemistry teacher diagnosed with inoperable lung cancer turns to manufacturing and selling methamphetamine in order to secure his family's future.","poster_path":"/ggFHVNu6YYI5L9W6QN5CvfWgmd.jpg","backdrop_path":"/tsRy63Mu5cu8etL1X7Z4e6dP8aA.jpg","first_air_date":"2008-01-20","vote_average":8.918,"vote_count":12000,"popularity":280.0,"original_language":"en","genre_ids":[18,80],"media_type":"tv"}]}`

	fixtureMultiFightClub = `{"page":1,"total_pages":1,"total_results":1,"results":[{"id":550,"title":"Fight Club","original_title":"Fight Club","overview":"An insomniac office worker and a devil-may-care soap maker form an underground fight club that evolves into something much more.","poster_path":"/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg","backdrop_path":"/fCayJrkfRaCRCTh8GqLQLDJhzsF.jpg","release_date":"1999-10-15","vote_average":8.433,"vote_count":26280,"popularity":61.416,"original_language":"en","genre_ids":[18,53],"media_type":"movie","adult":false}]}`

	fixtureMultiBreakingBad = `{"page":1,"total_pages":1,"total_results":1,"results":[{"id":1396,"name":"Breaking Bad","original_name":"Breaking Bad","overview":"A high school chemistry teacher diagnosed with inoperable lung cancer turns to manufacturing and selling methamphetamine in order to secure his family's future.","poster_path":"/ggFHVNu6YYI5L9W6QN5CvfWgmd.jpg","first_air_date":"2008-01-20","vote_average":8.918,"vote_count":12000,"popularity":280.0,"original_language":"en","genre_ids":[18,80],"media_type":"tv"}]}`

	fixtureEmptyPage = `{"page":1,"total_pages":0,"total_results":0,"results":[]}`

	fixtureFightClubMovie = `{"id":550,"title":"Fight Club","original_title":"Fight Club","overview":"An insomniac office worker and a devil-may-care soap maker form an underground fight club that evolves into something much more.","tagline":"Mischief. Mayhem. Soap.","poster_path":"/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg","backdrop_path":"/fCayJrkfRaCRCTh8GqLQLDJhzsF.jpg","release_date":"1999-10-15","runtime":139,"vote_average":8.433,"vote_count":26280,"popularity":61.416,"status":"Released","original_language":"en","adult":false,"budget":63000000,"revenue":100853753,"homepage":"https://www.foxmovies.com/movies/fight-club","imdb_id":"tt0137523","genres":[{"id":18,"name":"Drama"},{"id":53,"name":"Thriller"}],"production_companies":[{"id":508,"name":"Regency Enterprises","logo_path":"/7PzJdsLGlR7oW4J0J5Xcd0pHGRg.png","origin_country":"US"},{"id":25,"name":"20th Century Fox","logo_path":"/qZCc1lxd3zbdrcXJSDwOAyfL5bO.png","origin_country":"US"}],"spoken_languages":[{"iso_639_1":"en","name":"English"}],"videos":{"results":[{"id":"5a5927a8c3a3685447022d9c","key":"SUXWAEX2jlg","name":"Fight Club - Trailer","site":"YouTube","type":"Trailer"}]},"credits":{"cast":[{"id":819,"name":"Edward Norton","character":"The Narrator","profile_path":"/5XBzL5p2N0R8q4G9yWBS0itMGRg.jpg","order":0},{"id":287,"name":"Brad Pitt","character":"Tyler Durden","profile_path":"/ccKC9DtZM8eB0Q2n7pcG7x2iL2.jpg","order":1},{"id":1283,"name":"Helena Bonham Carter","character":"Marla Singer","profile_path":"/DDeITcCpnGp0n0z5z2w2euKAXu.jpg","order":2}],"crew":[{"id":7467,"name":"David Fincher","job":"Director","department":"Directing","profile_path":"/8f6t6f5w4k3j2h1g0f9e8d7c6b5a4.jpg"}]},"release_dates":{"results":[{"iso_3166_1":"DE","release_dates":[{"certification":"18","descriptors":[],"iso_639_1":"","note":"","release_date":"1999-11-11T00:00:00.000Z","type":3}]},{"iso_3166_1":"GB","release_dates":[{"certification":"18","descriptors":[],"iso_639_1":"","note":"","release_date":"1999-11-12T00:00:00.000Z","type":3}]},{"iso_3166_1":"US","release_dates":[{"certification":"","descriptors":[],"iso_639_1":"","note":"Venice Film Festival","release_date":"1999-09-10T00:00:00.000Z","type":1},{"certification":"R","descriptors":[],"iso_639_1":"","note":"","release_date":"1999-10-15T00:00:00.000Z","type":3},{"certification":"R","descriptors":[],"iso_639_1":"","note":"DVD","release_date":"2000-06-06T00:00:00.000Z","type":5}]}]}}`

	fixtureFightClubAltTitles = `{"id":550,"titles":[{"iso_3166_1":"US","title":"Fight Club","type":""},{"iso_3166_1":"DE","title":"Fight Club","type":""},{"iso_3166_1":"FR","title":"Fight Club: Le Club de la Combat","type":""}]}`

	fixtureBreakingBadTV = `{"id":1396,"name":"Breaking Bad","original_name":"Breaking Bad","overview":"A high school chemistry teacher diagnosed with inoperable lung cancer turns to manufacturing and selling methamphetamine in order to secure his family's future.","tagline":"Change the equation.","poster_path":"/ggFHVNu6YYI5L9W6QN5CvfWgmd.jpg","backdrop_path":"/tsRy63Mu5cu8etL1X7Z4e6dP8aA.jpg","first_air_date":"2008-01-20","last_air_date":"2013-09-29","number_of_seasons":5,"number_of_episodes":62,"vote_average":8.918,"vote_count":12000,"popularity":280.0,"status":"Ended","original_language":"en","homepage":"https://www.amc.com/shows/breaking-bad","in_production":false,"genres":[{"id":18,"name":"Drama"},{"id":80,"name":"Crime"}],"origin_country":["US"],"created_by":[{"id":66633,"name":"Vince Gilligan","profile_path":"/rLSOn0xP7ZbdmDqf1J3LmMCWvdL.jpg"}],"networks":[{"id":174,"name":"AMC","logo_path":"/alqLicR1ZMHMaZGP3xRQh8VjcVn.png","origin_country":"US"}],"production_companies":[{"id":2605,"name":"Gran Via Productions","origin_country":"US"}],"spoken_languages":[{"iso_639_1":"en","name":"English"}],"seasons":[{"id":3571,"name":"Specials","overview":"","poster_path":null,"air_date":"2009-02-17","season_number":0,"episode_count":9,"vote_average":0},{"id":3572,"name":"Season 1","overview":"High school chemistry teacher Walter White's life is suddenly transformed by a dire medical diagnosis.","poster_path":"/1BP4HzxjZ7wSAiX8EVu5IXyMgkw.jpg","air_date":"2008-01-20","season_number":1,"episode_count":7,"vote_average":8.2},{"id":3573,"name":"Season 2","overview":"Walt and Jesse's partnership deepens as the business expands.","poster_path":"/e3o6NFlCcK9rEaPVqUw5YqZJ7qK.jpg","air_date":"2009-03-08","season_number":2,"episode_count":13,"vote_average":8.4},{"id":3575,"name":"Season 3","overview":"Walt faces new threats as Gus Fring enters the picture.","poster_path":"/ffP3CLvWnRkmC7Kd3N4mjdO3lYp.jpg","air_date":"2010-03-21","season_number":3,"episode_count":13,"vote_average":8.5},{"id":3576,"name":"Season 4","overview":"Walt and Gus escalate their war of wills.","poster_path":"/5eq2rH0Fgm8s0Qjh9eWXf6YKXQu.jpg","air_date":"2011-07-17","season_number":4,"episode_count":13,"vote_average":8.7},{"id":3578,"name":"Season 5","overview":"The final chapters of Walter White's transformation.","poster_path":"/r3z70v2iaOOcQQmcDtDuwQCfTPX.jpg","air_date":"2012-07-15","season_number":5,"episode_count":16,"vote_average":9.0}],"videos":{"results":[{"id":"5a5927a8c3a3685447022d9d","key":"HhesaQXLuRY","name":"Breaking Bad - Trailer","site":"YouTube","type":"Trailer"}]},"credits":{"cast":[{"id":17419,"name":"Bryan Cranston","character":"Walter White","profile_path":"/fngT2qNEhH9V8N2V7q8k3m4n5p6q.jpg","order":0},{"id":84497,"name":"Aaron Paul","character":"Jesse Pinkman","profile_path":"/7q8k9m0n1p2q3r4s5t6u7v8w9x0y.jpg","order":1},{"id":134,"name":"Anna Gunn","character":"Skyler White","profile_path":"/a1b2c3d4e5f6g7h8i9j0k1l2m3n4.jpg","order":2}],"crew":[{"id":66633,"name":"Vince Gilligan","job":"Creator","department":"Production","profile_path":"/rLSOn0xP7ZbdmDqf1J3LmMCWvdL.jpg"}]},"content_ratings":{"results":[{"descriptors":[],"iso_3166_1":"DE","rating":"16"},{"descriptors":[],"iso_3166_1":"GB","rating":"15"},{"descriptors":[],"iso_3166_1":"US","rating":"TV-MA"}]}}`

	fixtureBreakingBadAltTitles = `{"id":1396,"results":[{"iso_3166_1":"US","title":"Breaking Bad","type":""},{"iso_3166_1":"ES","title":"Breaking Bad: Metástasis","type":""}]}`

	fixtureBreakingBadSeason1 = `{"id":3572,"name":"Season 1","overview":"High school chemistry teacher Walter White's life is suddenly transformed by a dire medical diagnosis. Debt-ridden, dying from lung cancer, into a life of crime to provide for his family.","poster_path":"/1BP4HzxjZ7wSAiX8EVu5IXyMgkw.jpg","air_date":"2008-01-20","season_number":1,"vote_average":8.2,"episodes":[{"id":62085,"name":"Pilot","overview":"Diagnosed with terminal lung cancer, chemistry teacher Walter White teams up with former student Jesse Pinkman to cook and sell crystal meth.","air_date":"2008-01-20","episode_number":1,"season_number":1,"still_path":"/ydlY3eGaleoAMorlrK4qgW0EdDt.jpg","runtime":58,"vote_average":8.2,"vote_count":180,"production_code":"101"},{"id":62086,"name":"Cat's in the Bag...","overview":"Walt and Jesse attempt to dispose of the body of Emilio, who Walt killed in their first drug deal.","air_date":"2008-01-27","episode_number":2,"season_number":1,"still_path":"/sYn4qYqk5vd1hYTuB1L5ZQ0l0.jpg","runtime":48,"vote_average":7.9,"vote_count":150,"production_code":"102"},{"id":62087,"name":"...And the Bag's in the River","overview":"Walt is forced to make a tough decision as he and Jesse clean up the mess from their first cook.","air_date":"2008-02-10","episode_number":3,"season_number":1,"still_path":"/5x0q1.jpg","runtime":48,"vote_average":8.0,"vote_count":140,"production_code":"103"},{"id":62088,"name":"Cancer Man","overview":"Walt tells the rest of his family about his cancer. Jesse tries to retrieve his clothes from his parents' house.","air_date":"2008-02-17","episode_number":4,"season_number":1,"still_path":"/5x0q2.jpg","runtime":48,"vote_average":7.8,"vote_count":130,"production_code":"104"},{"id":62089,"name":"Gray Matter","overview":"Walt rejects charity from former partners Gretchen and Elliott. Jesse tries going solo.","air_date":"2008-02-24","episode_number":5,"season_number":1,"still_path":"/5x0q3.jpg","runtime":48,"vote_average":8.1,"vote_count":135,"production_code":"105"},{"id":62090,"name":"Crazy Handful of Nothin'","overview":"Walt and Jesse try a new approach to distribution that brings them face to face with Tuco.","air_date":"2008-03-02","episode_number":6,"season_number":1,"still_path":"/5x0q4.jpg","runtime":48,"vote_average":8.5,"vote_count":160,"production_code":"106"},{"id":62091,"name":"A No-Rough-Stuff-Type Deal","overview":"Walt and Jesse finalize a deal with Tuco. Skyler grows suspicious of Walt's secrecy.","air_date":"2008-03-09","episode_number":7,"season_number":1,"still_path":"/5x0q5.jpg","runtime":48,"vote_average":8.3,"vote_count":145,"production_code":"107"}]}`

	fixtureBreakingBadSeason2 = `{"id":3573,"name":"Season 2","overview":"Walt and Jesse's partnership deepens as the business expands and consequences mount.","poster_path":"/e3o6NFlCcK9rEaPVqUw5YqZJ7qK.jpg","air_date":"2009-03-08","season_number":2,"vote_average":8.4,"episodes":[{"id":62092,"name":"Seven Thirty-Seven","overview":"Walt and Jesse realize how dangerous Tuco can be.","air_date":"2009-03-08","episode_number":1,"season_number":2,"still_path":"/s2e1.jpg","runtime":47,"vote_average":8.3,"vote_count":120,"production_code":"201"},{"id":62093,"name":"Grilled","overview":"Walt and Jesse are held captive by Tuco.","air_date":"2009-03-15","episode_number":2,"season_number":2,"still_path":"/s2e2.jpg","runtime":47,"vote_average":8.6,"vote_count":130,"production_code":"202"}]}`

	fixtureBreakingBadSeason0 = `{"id":3571,"name":"Specials","overview":"","poster_path":null,"air_date":"2009-02-17","season_number":0,"vote_average":0,"episodes":[]}`

	fixtureBreakingBadSeason3 = `{"id":3575,"name":"Season 3","overview":"Walt faces new threats as Gus Fring enters the picture.","poster_path":"/ffP3CLvWnRkmC7Kd3N4mjdO3lYp.jpg","air_date":"2010-03-21","season_number":3,"vote_average":8.5,"episodes":[]}`

	fixtureBreakingBadSeason4 = `{"id":3576,"name":"Season 4","overview":"Walt and Gus escalate their war of wills.","poster_path":"/5eq2rH0Fgm8s0Qjh9eWXf6YKXQu.jpg","air_date":"2011-07-17","season_number":4,"vote_average":8.7,"episodes":[]}`

	fixtureBreakingBadSeason5 = `{"id":3578,"name":"Season 5","overview":"The final chapters of Walter White's transformation.","poster_path":"/r3z70v2iaOOcQQmcDtDuwQCfTPX.jpg","air_date":"2012-07-15","season_number":5,"vote_average":9.0,"episodes":[]}`

	fixtureStarWarsCollection = `{"id":10,"name":"Star Wars Collection","overview":"A long time ago in a galaxy far, far away...","poster_path":"/c.jpg","backdrop_path":"/b.jpg","parts":[{"id":11,"title":"Star Wars","original_title":"Star Wars","overview":"Luke Skywalker joins forces with a Jedi Knight.","poster_path":"/p1.jpg","backdrop_path":"/b1.jpg","release_date":"1977-05-25","vote_average":8.2,"media_type":"movie"}]}`

	fixtureFindFightClub = `{"movie_results":[{"id":550,"title":"Fight Club","original_title":"Fight Club","overview":"An insomniac office worker and a devil-may-care soap maker form an underground fight club that evolves into something much more.","poster_path":"/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg","backdrop_path":"/fCayJrkfRaCRCTh8GqLQLDJhzsF.jpg","release_date":"1999-10-15","vote_average":8.433,"vote_count":26280,"popularity":61.416,"original_language":"en","genre_ids":[18,53],"media_type":"movie","adult":false}],"tv_results":[],"person_results":[],"tv_episode_results":[],"tv_season_results":[]}`

	fixtureFindBreakingBad = `{"movie_results":[],"tv_results":[{"id":1396,"name":"Breaking Bad","original_name":"Breaking Bad","overview":"A high school chemistry teacher diagnosed with inoperable lung cancer turns to manufacturing and selling methamphetamine in order to secure his family's future.","poster_path":"/ggFHVNu6YYI5L9W6QN5CvfWgmd.jpg","first_air_date":"2008-01-20","vote_average":8.918,"vote_count":12000,"popularity":280.0,"original_language":"en","genre_ids":[18,80],"media_type":"tv"}],"person_results":[],"tv_episode_results":[],"tv_season_results":[]}`

	fixturePopularMovies = `{"page":1,"total_pages":1,"total_results":1,"results":[{"id":550,"title":"Fight Club","original_title":"Fight Club","overview":"An insomniac office worker and a devil-may-care soap maker form an underground fight club that evolves into something much more.","poster_path":"/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg","backdrop_path":"/fCayJrkfRaCRCTh8GqLQLDJhzsF.jpg","release_date":"1999-10-15","vote_average":8.433,"vote_count":26280,"popularity":61.416,"original_language":"en","genre_ids":[18,53],"media_type":"movie","adult":false}]}`

	fixturePopularTV = `{"page":1,"total_pages":1,"total_results":1,"results":[{"id":1396,"name":"Breaking Bad","original_name":"Breaking Bad","overview":"A high school chemistry teacher diagnosed with inoperable lung cancer turns to manufacturing and selling methamphetamine in order to secure his family's future.","poster_path":"/ggFHVNu6YYI5L9W6QN5CvfWgmd.jpg","first_air_date":"2008-01-20","vote_average":8.918,"vote_count":12000,"popularity":280.0,"original_language":"en","genre_ids":[18,80],"media_type":"tv"}]}`

	fixtureTrendingMovie = fixturePopularMovies
	fixtureTrendingTV    = fixturePopularTV
)

// fixtureGet serves offline TMDB payloads when TMDB_FIXTURE=1 (MVP smoke / no API key).
func (m *Module) fixtureGet(endpoint string, params url.Values, dest any) error {
	q := strings.ToLower(strings.TrimSpace(params.Get("query")))
	var body []byte

	switch endpoint {
	case "/3/configuration":
		body = []byte(fixtureConfigJSON)

	case "/3/search/movie":
		yearOK := fixtureYearOK(params.Get("primary_release_year"), "1999")
		if yearOK && fixtureQueryMatch(q, "fight club", "fight") {
			body = []byte(fixtureFightClubSearch)
		} else {
			body = []byte(fixtureEmptyPage)
		}

	case "/3/search/tv":
		yearOK := fixtureYearOK(params.Get("first_air_date_year"), "2008")
		if yearOK && fixtureQueryMatch(q, "breaking bad", "breaking") {
			body = []byte(fixtureBreakingBadSearch)
		} else {
			body = []byte(fixtureEmptyPage)
		}

	case "/3/search/multi":
		year := params.Get("year")
		switch {
		case fixtureQueryMatch(q, "fight club", "fight") && fixtureYearOK(year, "1999"):
			body = []byte(fixtureMultiFightClub)
		case fixtureQueryMatch(q, "breaking bad", "breaking") && fixtureYearOK(year, "2008"):
			body = []byte(fixtureMultiBreakingBad)
		default:
			body = []byte(fixtureEmptyPage)
		}

	case "/3/movie/550/alternative_titles":
		body = []byte(fixtureFightClubAltTitles)
	case "/3/movie/550":
		body = []byte(fixtureFightClubMovie)

	case "/3/tv/1396/alternative_titles":
		body = []byte(fixtureBreakingBadAltTitles)
	case "/3/tv/1396/season/1":
		body = []byte(fixtureBreakingBadSeason1)
	case "/3/tv/1396/season/2":
		body = []byte(fixtureBreakingBadSeason2)
	case "/3/tv/1396/season/0":
		body = []byte(fixtureBreakingBadSeason0)
	case "/3/tv/1396/season/3":
		body = []byte(fixtureBreakingBadSeason3)
	case "/3/tv/1396/season/4":
		body = []byte(fixtureBreakingBadSeason4)
	case "/3/tv/1396/season/5":
		body = []byte(fixtureBreakingBadSeason5)
	case "/3/tv/1396":
		body = []byte(fixtureBreakingBadTV)

	case "/3/collection/10":
		body = []byte(fixtureStarWarsCollection)

	case "/3/find/tt0137523":
		body = []byte(fixtureFindFightClub)
	case "/3/find/tt0903747":
		body = []byte(fixtureFindBreakingBad)

	case "/3/movie/popular", "/3/trending/movie/day", "/3/trending/movie/week":
		body = []byte(fixtureTrendingMovie)
	case "/3/tv/popular", "/3/trending/tv/day", "/3/trending/tv/week":
		body = []byte(fixtureTrendingTV)
	case "/3/trending/all/day", "/3/trending/all/week":
		body = []byte(fixtureMultiFightClub)

	default:
		if strings.HasPrefix(endpoint, "/3/collection/") {
			body = []byte(fixtureStarWarsCollection)
		} else if strings.Contains(endpoint, "/search/") || strings.Contains(endpoint, "/popular") || strings.Contains(endpoint, "/trending/") {
			body = []byte(fixtureEmptyPage)
		} else if strings.HasPrefix(endpoint, "/3/find/") {
			body = []byte(`{"movie_results":[],"tv_results":[],"person_results":[],"tv_episode_results":[],"tv_season_results":[]}`)
		} else {
			return fmt.Errorf("TMDB fixture: unsupported endpoint %s (set a real TMDB_API_KEY for live TMDB)", endpoint)
		}
	}

	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode tmdb fixture: %w", err)
	}
	return nil
}

func fixtureQueryMatch(q string, phrases ...string) bool {
	if q == "" {
		return false
	}
	for _, p := range phrases {
		if strings.Contains(q, p) {
			return true
		}
	}
	return false
}

// fixtureYearOK accepts empty year (no filter) or an exact match to want.
func fixtureYearOK(got, want string) bool {
	got = strings.TrimSpace(got)
	if got == "" {
		return true
	}
	if _, err := strconv.Atoi(got); err != nil {
		return false
	}
	return got == want
}
