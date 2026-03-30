package discover

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	gocache "github.com/robfig/go-cache"
	"github.com/sbondCo/Watcharr/config"
	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"github.com/sbondCo/Watcharr/media/jikan"
	"github.com/sbondCo/Watcharr/media/tmdb"
	"gorm.io/gorm"
)

// ─── Recommendation tuning constants ───────────────────────────────────────
// Adjust these to change how recommendations are computed and served.
const (
	// Max number of watched items used as recommendation sources (highest rated first).
	recMaxSources = 500
	// Default weight for unrated items (rating 0). Scale is 0–10.
	recDefaultWeight = 2.5
	// Number of TMDB recommendation pages fetched per source item (~20 results/page).
	recTMDBPages = 2
	// Default page size when paginating recommendation results.
	recDefaultPageSize = 20
	// How long computed recommendations are cached before recomputing.
	recCacheTTL = time.Hour
	// Interval at which expired cache entries are purged.
	recCacheCleanup = time.Minute * 5
)

// In-memory cache for computed recommendation lists (per user + content type).
var recCache = gocache.New(recCacheTTL, recCacheCleanup)

// recProgressMap tracks computation progress per user so the frontend can poll it.
var recProgressMap sync.Map

// RecProgress holds the current recommendation computation progress.
type RecProgress struct {
	Current   int  `json:"current"`
	Total     int  `json:"total"`
	Computing bool `json:"computing"`
}

func recProgressKey(userID uint, sourceUserID uint, contentType string) string {
	return fmt.Sprintf("%d_%d_%s", userID, sourceUserID, contentType)
}

type ContentProvider interface {
	Trending(t tmdb.TrendingType, pageNum int, region string) (tmdb.TMDBTrendingCombined, error)
	DiscoverMovies(o tmdb.DiscoverOptions, pageNum int, region string) (tmdb.TMDBDiscoverMovies, error)
	DiscoverTv(o tmdb.DiscoverOptions, pageNum int, region string) (tmdb.TMDBDiscoverShows, error)
	PopularPeople(pageNum int) (tmdb.TMDBPopularPeople, error)
	MovieRecommendations(tmdbId int, pageNum int) (tmdb.TMDBMovieSimilar, error)
	ShowRecommendations(tmdbId int, pageNum int) (tmdb.TMDBShowSimilar, error)
	FlushRecommendationCache()
}

type FollowProvider interface {
	IsFollowing(currentUserId uint, targetUserId uint) bool
}

type Service struct {
	db              *gorm.DB
	cfg             *config.ServerConfig
	contentProvider ContentProvider
	followProvider  FollowProvider
	jikan           *jikan.Jikan
}

func NewService(
	db *gorm.DB,
	cfg *config.ServerConfig,
	contentProvider ContentProvider,
	followProvider FollowProvider,
	jikan *jikan.Jikan,
) *Service {
	return &Service{
		db,
		cfg,
		contentProvider,
		followProvider,
		jikan,
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
	case domain.SearchTypeManga:
		return s.DiscoverManga(r, meta)
	case domain.SearchTypeAnime:
		return s.DiscoverAnime(r, meta)
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

// Discover anime (TV shows with anime keyword).
func (s *Service) DiscoverAnime(
	r domain.DiscoverRequest,
	meta domain.DiscoverRequestMeta,
) (domain.DiscoverResponse, error) {
	resp := domain.DiscoverResponse{}
	var err error
	switch r.Filter {
	case domain.DiscoverFilterTrending:
		err = s.discoverAnimeTrending(meta, &resp)
	case domain.DiscoverFilterUpcoming:
		err = s.discoverAnimeUpcoming(meta, &resp)
	case domain.DiscoverFilterPopular:
		err = s.discoverAnimePopular(meta, &resp)
	default:
		slog.Error("DiscoverAnime: Unsupported filter.")
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

// Discover manga.
func (s *Service) DiscoverManga(
	r domain.DiscoverRequest,
	meta domain.DiscoverRequestMeta,
) (domain.DiscoverResponse, error) {
	resp := domain.DiscoverResponse{}
	var err error
	switch r.Filter {
	case domain.DiscoverFilterTrending:
		err = s.discoverMangaTrending(meta, &resp)
	default:
		slog.Error("DiscoverManga: Unsupported filter.")
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

// animeKeyword is the TMDB keyword ID for "anime".
const animeKeyword = "210024"

func (s *Service) discoverAnimeTrending(
	meta domain.DiscoverRequestMeta,
	resp *domain.DiscoverResponse,
) error {
	tmdbRes, err := s.contentProvider.DiscoverTv(
		tmdb.DiscoverOptions{
			WithKeywords: animeKeyword,
		},
		meta.PageParams.Page,
		meta.Region,
	)
	if err != nil {
		slog.Error("discoverAnimeTrending: Failed to search tmdb!", "error", err)
		return errors.New("content request failed")
	}
	for _, v := range tmdbRes.Results {
		resp.Results = append(resp.Results, v.AsMedia())
	}
	resp.Page = tmdbRes.Page
	resp.TotalPages = tmdbRes.TotalPages
	resp.TotalResults = int64(tmdbRes.TotalResults)
	return nil
}

func (s *Service) discoverAnimeUpcoming(
	meta domain.DiscoverRequestMeta,
	resp *domain.DiscoverResponse,
) error {
	tmdbRes, err := s.contentProvider.DiscoverTv(
		tmdb.DiscoverOptions{
			ReleaseDateMin: time.Now(),
			ReleaseDateMax: time.Now().AddDate(0, 1, 0),
			WithKeywords:   animeKeyword,
		},
		meta.PageParams.Page,
		meta.Region,
	)
	if err != nil {
		slog.Error("discoverAnimeUpcoming: Failed to search tmdb!", "error", err)
		return errors.New("content request failed")
	}
	for _, v := range tmdbRes.Results {
		resp.Results = append(resp.Results, v.AsMedia())
	}
	resp.Page = tmdbRes.Page
	resp.TotalPages = tmdbRes.TotalPages
	resp.TotalResults = int64(tmdbRes.TotalResults)
	return nil
}

func (s *Service) discoverAnimePopular(
	meta domain.DiscoverRequestMeta,
	resp *domain.DiscoverResponse,
) error {
	tmdbRes, err := s.contentProvider.DiscoverTv(
		tmdb.DiscoverOptions{
			WithKeywords: animeKeyword,
		},
		meta.PageParams.Page,
		meta.Region,
	)
	if err != nil {
		slog.Error("discoverAnimePopular: Failed to search tmdb!", "error", err)
		return errors.New("content request failed")
	}
	for _, v := range tmdbRes.Results {
		resp.Results = append(resp.Results, v.AsMedia())
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

func (s *Service) discoverMangaTrending(
	meta domain.DiscoverRequestMeta,
	resp *domain.DiscoverResponse,
) error {
	if s.jikan == nil {
		return errors.New("manga provider not available")
	}
	jikanRes, err := s.jikan.TopManga(meta.PageParams.Page)
	if err != nil {
		slog.Error("discoverMangaTrending: Failed to get top manga from jikan!", "error", err)
		return errors.New("content request failed")
	}
	for _, v := range jikanRes.Data {
		resp.Results = append(
			resp.Results,
			v.AsMedia(),
		)
	}
	resp.Page = jikanRes.Pagination.CurrentPage
	resp.TotalPages = jikanRes.Pagination.LastVisiblePage
	resp.TotalResults = int64(jikanRes.Pagination.Items.Total)
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
	// Determine which user's watched list to use as recommendation source.
	sourceUserID := meta.UserID
	if meta.SourceUserID != 0 {
		// Validate that the current user follows the source user.
		if !s.followProvider.IsFollowing(meta.UserID, meta.SourceUserID) {
			return errors.New("you must follow this user to get recommendations from them")
		}
		sourceUserID = meta.SourceUserID
	}

	cacheKey := fmt.Sprintf("recommendations_%d_%d_%s", meta.UserID, sourceUserID, contentType)

	// Try to get cached results
	var allResults []domain.Media
	if cached, found := recCache.Get(cacheKey); found {
		allResults = cached.([]domain.Media)
		resp.FromCache = true
		slog.Info("discoverRecommended: Serving from cache", "cacheKey", cacheKey, "total", len(allResults))
	} else {
		// Compute recommendations from scratch
		progKey := recProgressKey(meta.UserID, sourceUserID, contentType)
		recProgressMap.Store(progKey, &RecProgress{Computing: true})
		var err error
		allResults, err = s.computeRecommendations(contentType, sourceUserID, meta.UserID, progKey)
		recProgressMap.Delete(progKey)
		if err != nil {
			return err
		}
		// Cache for 1 hour
		recCache.Set(cacheKey, allResults, recCacheTTL)
		slog.Debug("discoverRecommended: Computed and cached", "user", meta.UserID, "contentType", contentType, "total", len(allResults))
	}

	// Paginate
	total := len(allResults)
	limit := meta.PageParams.Limit
	if limit <= 0 {
		limit = recDefaultPageSize
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
// sourceUserID: the user whose watched list is used as source.
// viewerUserID: the current user viewing (to exclude their own watched items).
func (s *Service) computeRecommendations(contentType string, sourceUserID uint, viewerUserID uint, progKey string) ([]domain.Media, error) {
	// Get source user's watched items with content loaded
	var watched []entity.Watched
	res := s.db.Model(&entity.Watched{}).
		Preload("Content").
		Where("user_id = ? AND content_id IS NOT NULL", sourceUserID).
		Find(&watched)
	if res.Error != nil {
		slog.Error("computeRecommendations: Failed to get watched!", "error", res.Error)
		return nil, errors.New("failed to get watched list")
	}

	if len(watched) == 0 {
		return nil, nil
	}

	// Build watched set for viewer (to exclude items they already have)
	watchedSet := make(map[int]bool)
	if viewerUserID != sourceUserID {
		var viewerWatched []entity.Watched
		vRes := s.db.Model(&entity.Watched{}).
			Preload("Content").
			Where("user_id = ? AND content_id IS NOT NULL", viewerUserID).
			Find(&viewerWatched)
		if vRes.Error == nil {
			for _, w := range viewerWatched {
				if w.Content != nil {
					watchedSet[w.Content.TmdbID] = true
				}
			}
		}
	}

	// Build source items: tmdbId -> weight, filtered by content type, excluding DROPPED.
	type sourceItem struct {
		tmdbId      int
		contentType entity.ContentType
		weight      float64
		title       string
	}
	var sources []sourceItem

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
			weight = recDefaultWeight
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

	if len(sources) > recMaxSources {
		sources = sources[:recMaxSources]
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

	// Update progress tracker with total source count
	if p, ok := recProgressMap.Load(progKey); ok {
		prog := p.(*RecProgress)
		prog.Total = len(sources)
	}

	for i, src := range sources {
		// Update progress after each source
		if p, ok := recProgressMap.Load(progKey); ok {
			prog := p.(*RecProgress)
			prog.Current = i
		}

		for pg := 1; pg <= recTMDBPages; pg++ {
			switch src.contentType {
			case entity.MOVIE:
				tmdbRes, err := s.contentProvider.MovieRecommendations(src.tmdbId, pg)
				if err != nil {
					slog.Warn("computeRecommendations: Failed to get movie recs",
						"tmdbId", src.tmdbId, "page", pg, "error", err)
					break
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
				tmdbRes, err := s.contentProvider.ShowRecommendations(src.tmdbId, pg)
				if err != nil {
					slog.Warn("computeRecommendations: Failed to get show recs",
						"tmdbId", src.tmdbId, "page", pg, "error", err)
					break
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

// GetRecommendProgress returns the current computation progress for a user's recommendations.
func (s *Service) GetRecommendProgress(userID uint, sourceUserID uint, contentType string) RecProgress {
	if sourceUserID == 0 {
		sourceUserID = userID
	}
	key := recProgressKey(userID, sourceUserID, contentType)
	if p, ok := recProgressMap.Load(key); ok {
		return *p.(*RecProgress)
	}
	return RecProgress{}
}

// ClearRecommendCache removes the cached recommendations for a user.
func (s *Service) ClearRecommendCache(userID uint, sourceUserID uint, contentType string) {
	if sourceUserID == 0 {
		sourceUserID = userID
	}
	cacheKey := fmt.Sprintf("recommendations_%d_%d_%s", userID, sourceUserID, contentType)
	found := recCache.Delete(cacheKey)
	s.contentProvider.FlushRecommendationCache()
	slog.Info("ClearRecommendCache", "cacheKey", cacheKey, "found", found)
}

// GetRecommendSources returns followed users who have at least 1 non-dropped watched entry.
func (s *Service) GetRecommendSources(userID uint) ([]domain.RecommendSourceUser, error) {
	// Get users that userID follows.
	var follows []entity.Follow
	res := s.db.Where("user_id = ?", userID).
		Preload("FollowedUser", "private = ?", 0).
		Find(&follows)
	if res.Error != nil {
		slog.Error("GetRecommendSources: Failed to get follows", "error", res.Error)
		return nil, errors.New("failed to get follows")
	}

	var sources []domain.RecommendSourceUser
	for _, f := range follows {
		if f.FollowedUser.ID == 0 {
			continue
		}
		// Check if this followed user has at least 1 non-dropped watched entry.
		var count int64
		s.db.Model(&entity.Watched{}).
			Where("user_id = ? AND content_id IS NOT NULL AND status != ?", f.FollowedUserID, entity.DROPPED).
			Count(&count)
		if count > 0 {
			sources = append(sources, domain.RecommendSourceUser{
				ID:       f.FollowedUser.ID,
				Username: f.FollowedUser.Username,
			})
		}
	}
	return sources, nil
}
