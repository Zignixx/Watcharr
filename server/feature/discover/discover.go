package discover

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	gocache "github.com/robfig/go-cache"
	"github.com/sbondCo/Watcharr/config"
	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"github.com/sbondCo/Watcharr/media/tmdb"
	"gorm.io/gorm"
)

// In-memory cache for computed recommendation lists (per user + content type).
var recCache = gocache.New(time.Hour, time.Minute*5)

type ContentProvider interface {
	Trending(t tmdb.TrendingType, pageNum int, region string) (tmdb.TMDBTrendingCombined, error)
	DiscoverMovies(o tmdb.DiscoverOptions, pageNum int, region string) (tmdb.TMDBDiscoverMovies, error)
	DiscoverTv(o tmdb.DiscoverOptions, pageNum int, region string) (tmdb.TMDBDiscoverShows, error)
	PopularPeople(pageNum int) (tmdb.TMDBPopularPeople, error)
	MovieRecommendations(tmdbId int, pageNum int) (tmdb.TMDBMovieSimilar, error)
	ShowRecommendations(tmdbId int, pageNum int) (tmdb.TMDBShowSimilar, error)
}

type Service struct {
	db              *gorm.DB
	cfg             *config.ServerConfig
	contentProvider ContentProvider
}

func NewService(
	db *gorm.DB,
	cfg *config.ServerConfig,
	contentProvider ContentProvider,
) *Service {
	return &Service{
		db,
		cfg,
		contentProvider,
	}
}

// `Limit` is not supported.
func (s *Service) Discover(
	// User request
	r domain.DiscoverRequest,
	// Extra data
	meta domain.DiscoverRequestMeta,
) (domain.DiscoverResponse, error) {
	resp := domain.DiscoverResponse{}

	switch r.Type {
	case domain.SearchTypeMulti:
		return s.DiscoverMulti(r, meta)
	case domain.SearchTypeShow:
		return s.DiscoverTv(r, meta)
	case domain.SearchTypePerson:
		return s.DiscoverPeople(r, meta)
	case domain.SearchTypeMovie:
		return s.DiscoverMovie(r, meta)
	case domain.SearchTypeGame:
		return s.DiscoverGame(r, meta)
	}
	return resp, nil
}

// Discover Multi. Just for tmdb.
func (s *Service) DiscoverMulti(
	r domain.DiscoverRequest,
	meta domain.DiscoverRequestMeta,
) (domain.DiscoverResponse, error) {
	resp := domain.DiscoverResponse{}
	var err error
	switch r.Filter {
	case domain.DiscoverFilterTrending:
		err = s.discoverMultiTrending(tmdb.TrendingTypeAll, meta, &resp)
	case domain.DiscoverFilterInTheatres:
		err = s.discoverMovieInTheatres(meta, &resp)
	case domain.DiscoverFilterRecommended:
		err = s.discoverRecommended("", meta, &resp)
	default:
		slog.Error("DiscoverMulti: Unsupported filter.")
		return resp, errors.New("unsupported filter")
	}
	return resp, err
}

// Discover movies.
func (s *Service) DiscoverMovie(
	r domain.DiscoverRequest,
	meta domain.DiscoverRequestMeta,
) (domain.DiscoverResponse, error) {
	resp := domain.DiscoverResponse{}
	var err error
	switch r.Filter {
	case domain.DiscoverFilterTrending:
		err = s.discoverMultiTrending(tmdb.TrendingTypeMovie, meta, &resp)
	case domain.DiscoverFilterInTheatres:
		err = s.discoverMovieInTheatres(meta, &resp)
	case domain.DiscoverFilterUpcoming:
		err = s.discoverMovieUpcoming(meta, &resp)
	case domain.DiscoverFilterPopular:
		err = s.discoverMoviePopular(meta, &resp)
	case domain.DiscoverFilterRecommended:
		err = s.discoverRecommended("movie", meta, &resp)
	default:
		slog.Error("DiscoverMovie: Unsupported filter.")
		return resp, errors.New("unsupported filter")
	}
	return resp, err
}

// Discover shows.
func (s *Service) DiscoverTv(
	r domain.DiscoverRequest,
	meta domain.DiscoverRequestMeta,
) (domain.DiscoverResponse, error) {
	resp := domain.DiscoverResponse{}
	var err error
	switch r.Filter {
	case domain.DiscoverFilterTrending:
		err = s.discoverMultiTrending(tmdb.TrendingTypeShow, meta, &resp)
	case domain.DiscoverFilterUpcoming:
		err = s.discoverTvUpcoming(meta, &resp)
	case domain.DiscoverFilterPopular:
		err = s.discoverTvPopular(meta, &resp)
	case domain.DiscoverFilterRecommended:
		err = s.discoverRecommended("tv", meta, &resp)
	default:
		slog.Error("DiscoverTv: Unsupported filter.")
		return resp, errors.New("unsupported filter")
	}
	return resp, err
}

// Discover people.
func (s *Service) DiscoverPeople(
	r domain.DiscoverRequest,
	meta domain.DiscoverRequestMeta,
) (domain.DiscoverResponse, error) {
	resp := domain.DiscoverResponse{}
	var err error
	switch r.Filter {
	case domain.DiscoverFilterTrending:
		err = s.discoverMultiTrending(tmdb.TrendingTypePerson, meta, &resp)
	case domain.DiscoverFilterPopular:
		err = s.discoverPeoplePopular(meta, &resp)
	default:
		slog.Error("DiscoverMulti: Unsupported filter.")
		return resp, errors.New("unsupported filter")
	}
	return resp, err
}

// Discover games.
func (s *Service) DiscoverGame(
	r domain.DiscoverRequest,
	meta domain.DiscoverRequestMeta,
) (domain.DiscoverResponse, error) {
	resp := domain.DiscoverResponse{}
	var err error
	switch r.Filter {
	case domain.DiscoverFilterTrending:
		err = s.discoverGameTrending(&resp)
	case domain.DiscoverFilterUpcoming:
		err = s.discoverGameUpcoming(&resp)
	default:
		slog.Error("DiscoverGame: Unsupported filter.")
		return resp, errors.New("unsupported filter")
	}
	return resp, err
}

// Discover anything that is trending on TMDB (including combined).
func (s *Service) discoverMultiTrending(
	t tmdb.TrendingType,
	meta domain.DiscoverRequestMeta,
	resp *domain.DiscoverResponse,
) error {
	tmdbRes, err := s.contentProvider.Trending(t, meta.PageParams.Page, meta.Region)
	if err != nil {
		slog.Error("discoverMulti: Failed to search tmdb!", "error", err)
		return errors.New("content request failed")
	}
	for _, v := range tmdbRes.Results {
		resp.Results = append(
			resp.Results,
			v.AsMedia(),
		)
	}
	resp.Page = tmdbRes.Page
	resp.TotalPages = tmdbRes.TotalPages
	resp.TotalResults = int64(tmdbRes.TotalResults)
	return nil
}

func (s *Service) discoverMovieInTheatres(
	meta domain.DiscoverRequestMeta,
	resp *domain.DiscoverResponse,
) error {
	tmdbRes, err := s.contentProvider.DiscoverMovies(
		tmdb.DiscoverOptions{
			ReleaseDateMin:  time.Now().AddDate(0, 0, -40),
			ReleaseDateMax:  time.Now().AddDate(0, 0, 2),
			WithReleaseType: "2|3",
		},
		meta.PageParams.Page,
		meta.Region,
	)
	if err != nil {
		slog.Error("discoverMovieInTheatres: Failed to search tmdb!",
			"error", err)
		return errors.New("content request failed")
	}
	for _, v := range tmdbRes.Results {
		resp.Results = append(
			resp.Results,
			v.AsMedia(),
		)
	}
	resp.Page = tmdbRes.Page
	resp.TotalPages = tmdbRes.TotalPages
	resp.TotalResults = int64(tmdbRes.TotalResults)
	return nil
}

func (s *Service) discoverMovieUpcoming(
	meta domain.DiscoverRequestMeta,
	resp *domain.DiscoverResponse,
) error {
	tmdbRes, err := s.contentProvider.DiscoverMovies(
		tmdb.DiscoverOptions{
			ReleaseDateMin:  time.Now(),
			ReleaseDateMax:  time.Now().AddDate(0, 1, 0),
			WithReleaseType: "2|3",
		},
		meta.PageParams.Page,
		meta.Region,
	)
	if err != nil {
		slog.Error("discoverMovieUpcoming: Failed to search tmdb!",
			"error", err)
		return errors.New("content request failed")
	}
	for _, v := range tmdbRes.Results {
		resp.Results = append(
			resp.Results,
			v.AsMedia(),
		)
	}
	resp.Page = tmdbRes.Page
	resp.TotalPages = tmdbRes.TotalPages
	resp.TotalResults = int64(tmdbRes.TotalResults)
	return nil
}

func (s *Service) discoverMoviePopular(
	meta domain.DiscoverRequestMeta,
	resp *domain.DiscoverResponse,
) error {
	tmdbRes, err := s.contentProvider.DiscoverMovies(
		tmdb.DiscoverOptions{},
		meta.PageParams.Page,
		meta.Region,
	)
	if err != nil {
		slog.Error("discoverMoviePopular: Failed to search tmdb!",
			"error", err)
		return errors.New("content request failed")
	}
	for _, v := range tmdbRes.Results {
		resp.Results = append(
			resp.Results,
			v.AsMedia(),
		)
	}
	resp.Page = tmdbRes.Page
	resp.TotalPages = tmdbRes.TotalPages
	resp.TotalResults = int64(tmdbRes.TotalResults)
	return nil
}

func (s *Service) discoverTvUpcoming(
	meta domain.DiscoverRequestMeta,
	resp *domain.DiscoverResponse,
) error {
	tmdbRes, err := s.contentProvider.DiscoverTv(
		tmdb.DiscoverOptions{
			ReleaseDateMin:  time.Now(),
			ReleaseDateMax:  time.Now().AddDate(0, 1, 0),
			WithReleaseType: "2|3",
		},
		meta.PageParams.Page,
		meta.Region,
	)
	if err != nil {
		slog.Error("discoverTvUpcoming: Failed to search tmdb!",
			"error", err)
		return errors.New("content request failed")
	}
	for _, v := range tmdbRes.Results {
		resp.Results = append(
			resp.Results,
			v.AsMedia(),
		)
	}
	resp.Page = tmdbRes.Page
	resp.TotalPages = tmdbRes.TotalPages
	resp.TotalResults = int64(tmdbRes.TotalResults)
	return nil
}

func (s *Service) discoverTvPopular(
	meta domain.DiscoverRequestMeta,
	resp *domain.DiscoverResponse,
) error {
	tmdbRes, err := s.contentProvider.DiscoverTv(
		tmdb.DiscoverOptions{},
		meta.PageParams.Page,
		meta.Region,
	)
	if err != nil {
		slog.Error("discoverTvPopular: Failed to search tmdb!",
			"error", err)
		return errors.New("content request failed")
	}
	for _, v := range tmdbRes.Results {
		resp.Results = append(
			resp.Results,
			v.AsMedia(),
		)
	}
	resp.Page = tmdbRes.Page
	resp.TotalPages = tmdbRes.TotalPages
	resp.TotalResults = int64(tmdbRes.TotalResults)
	return nil
}

func (s *Service) discoverPeoplePopular(
	meta domain.DiscoverRequestMeta,
	resp *domain.DiscoverResponse,
) error {
	tmdbRes, err := s.contentProvider.PopularPeople(
		meta.PageParams.Page,
	)
	if err != nil {
		slog.Error("discoverPeoplePopular: Failed to search tmdb!",
			"error", err)
		return errors.New("content request failed")
	}
	for _, v := range tmdbRes.Results {
		resp.Results = append(
			resp.Results,
			v.AsMedia(),
		)
	}
	resp.Page = tmdbRes.Page
	resp.TotalPages = tmdbRes.TotalPages
	resp.TotalResults = int64(tmdbRes.TotalResults)
	return nil
}

func (s *Service) discoverGameTrending(
	resp *domain.DiscoverResponse,
) error {
	igdbRes, err := s.cfg.TWITCH.Trending()
	if err != nil {
		slog.Error("discoverGameTrending: Failed to search igdb!", "error", err)
		return errors.New("content request failed")
	}
	for _, v := range igdbRes {
		resp.Results = append(
			resp.Results,
			v.AsMedia(),
		)
	}
	resp.Page = 1
	resp.TotalPages = 1
	resp.TotalResults = int64(len(igdbRes))
	return nil
}

func (s *Service) discoverGameUpcoming(
	resp *domain.DiscoverResponse,
) error {
	igdbRes, err := s.cfg.TWITCH.Upcoming()
	if err != nil {
		slog.Error("discoverGameUpcoming: Failed to search igdb!", "error", err)
		return errors.New("content request failed")
	}
	for _, v := range igdbRes {
		resp.Results = append(
			resp.Results,
			v.AsMedia(),
		)
	}
	resp.Page = 1
	resp.TotalPages = 1
	resp.TotalResults = int64(len(igdbRes))
	return nil
}

// Discover recommended content based on user's watched list.
// Uses all watched items (excluding DROPPED), weighted by user rating.
// Recommendations that appear from multiple sources score higher.
// Results are cached per user+contentType for 1 hour and paginated.
// contentType: "movie", "tv", or "" for both.
func (s *Service) discoverRecommended(
	contentType string,
	meta domain.DiscoverRequestMeta,
	resp *domain.DiscoverResponse,
) error {
	cacheKey := fmt.Sprintf("recommendations_%d_%s", meta.UserID, contentType)

	// Try to get cached results
	var allResults []domain.Media
	if cached, found := recCache.Get(cacheKey); found {
		allResults = cached.([]domain.Media)
		slog.Debug("discoverRecommended: Serving from cache", "user", meta.UserID, "contentType", contentType, "total", len(allResults))
	} else {
		// Compute recommendations from scratch
		var err error
		allResults, err = s.computeRecommendations(contentType, meta.UserID)
		if err != nil {
			return err
		}
		// Cache for 1 hour
		recCache.Set(cacheKey, allResults, time.Hour)
		slog.Debug("discoverRecommended: Computed and cached", "user", meta.UserID, "contentType", contentType, "total", len(allResults))
	}

	// Paginate
	total := len(allResults)
	limit := meta.PageParams.Limit
	if limit <= 0 {
		limit = 20
	}
	page := meta.PageParams.Page
	if page <= 0 {
		page = 1
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	start := (page - 1) * limit
	end := start + limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	resp.Results = allResults[start:end]
	resp.Page = page
	resp.Limit = limit
	resp.TotalPages = totalPages
	resp.TotalResults = int64(total)
	return nil
}

// computeRecommendations does the heavy lifting: fetches watched list,
// calls TMDB for recommendations, scores and sorts them.
func (s *Service) computeRecommendations(contentType string, userID uint) ([]domain.Media, error) {
	// Get user's watched items with content loaded
	var watched []entity.Watched
	res := s.db.Model(&entity.Watched{}).
		Preload("Content").
		Where("user_id = ? AND content_id IS NOT NULL", userID).
		Find(&watched)
	if res.Error != nil {
		slog.Error("computeRecommendations: Failed to get watched!", "error", res.Error)
		return nil, errors.New("failed to get watched list")
	}

	if len(watched) == 0 {
		return nil, nil
	}

	// Build source items: tmdbId -> weight, filtered by content type, excluding DROPPED.
	type sourceItem struct {
		tmdbId      int
		contentType entity.ContentType
		weight      float64
		title       string
	}
	var sources []sourceItem
	watchedSet := make(map[int]bool)

	for _, w := range watched {
		if w.Content == nil {
			continue
		}
		watchedSet[w.Content.TmdbID] = true

		if w.Status == entity.DROPPED {
			continue
		}

		if contentType == "movie" && w.Content.Type != entity.MOVIE {
			continue
		}
		if contentType == "tv" && w.Content.Type != entity.SHOW {
			continue
		}
		if w.Content.Type != entity.MOVIE && w.Content.Type != entity.SHOW {
			continue
		}

		weight := w.Rating
		if weight <= 0 {
			weight = 5.0
		}

		sources = append(sources, sourceItem{
			tmdbId:      w.Content.TmdbID,
			contentType: w.Content.Type,
			weight:      weight,
			title:       w.Content.Title,
		})
	}

	if len(sources) == 0 {
		return nil, nil
	}

	sort.Slice(sources, func(i, j int) bool {
		return sources[i].weight > sources[j].weight
	})

	maxSources := 50
	if len(sources) > maxSources {
		sources = sources[:maxSources]
	}

	type recSource struct {
		name   string
		weight float64
	}
	type scoredRec struct {
		media      domain.Media
		score      float64
		sources    int
		recSources []recSource
	}
	recScores := make(map[int]*scoredRec)

	for _, src := range sources {
		switch src.contentType {
		case entity.MOVIE:
			tmdbRes, err := s.contentProvider.MovieRecommendations(src.tmdbId, 1)
			if err != nil {
				slog.Warn("computeRecommendations: Failed to get movie recs",
					"tmdbId", src.tmdbId, "error", err)
				continue
			}
			for _, v := range tmdbRes.Results {
				m := v.AsMedia()
				if watchedSet[m.IDs.TMDB] {
					continue
				}
				if existing, ok := recScores[m.IDs.TMDB]; ok {
					existing.score += src.weight
					existing.sources++
					existing.recSources = append(existing.recSources, recSource{name: src.title, weight: src.weight})
				} else {
					recScores[m.IDs.TMDB] = &scoredRec{
						media:      m,
						score:      src.weight,
						sources:    1,
						recSources: []recSource{{name: src.title, weight: src.weight}},
					}
				}
			}
		case entity.SHOW:
			tmdbRes, err := s.contentProvider.ShowRecommendations(src.tmdbId, 1)
			if err != nil {
				slog.Warn("computeRecommendations: Failed to get show recs",
					"tmdbId", src.tmdbId, "error", err)
				continue
			}
			for _, v := range tmdbRes.Results {
				m := v.AsMedia()
				if watchedSet[m.IDs.TMDB] {
					continue
				}
				if existing, ok := recScores[m.IDs.TMDB]; ok {
					existing.score += src.weight
					existing.sources++
					existing.recSources = append(existing.recSources, recSource{name: src.title, weight: src.weight})
				} else {
					recScores[m.IDs.TMDB] = &scoredRec{
						media:      m,
						score:      src.weight,
						sources:    1,
						recSources: []recSource{{name: src.title, weight: src.weight}},
					}
				}
			}
		}
	}

	// Convert to slice and sort by score descending
	results := make([]scoredRec, 0, len(recScores))
	for _, rec := range recScores {
		results = append(results, *rec)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return results[i].sources > results[j].sources
	})

	// Build final media list with RecommendedBy populated
	allMedia := make([]domain.Media, 0, len(results))
	for _, rec := range results {
		m := rec.media
		for _, rs := range rec.recSources {
			pct := 0.0
			if rec.score > 0 {
				pct = rs.weight / rec.score * 100
			}
			m.RecommendedBy = append(m.RecommendedBy, domain.RecommendationSource{Name: rs.name, Weight: pct})
		}
		allMedia = append(allMedia, m)
	}

	return allMedia, nil
}
