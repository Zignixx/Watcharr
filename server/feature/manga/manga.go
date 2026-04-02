package manga

import (
	"errors"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/sbondCo/Watcharr/config"
	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"github.com/sbondCo/Watcharr/image"
	"github.com/sbondCo/Watcharr/media/jikan"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db               *gorm.DB
	jikan            *jikan.Jikan
	activityProvider domain.ActivityAddProvider
}

func NewService(db *gorm.DB, jikan *jikan.Jikan, activityProvider domain.ActivityAddProvider) *Service {
	s := &Service{
		db,
		jikan,
		activityProvider,
	}
	s.refreshIncompleteManga()
	return s
}

// refreshIncompleteManga is a one-time migration that re-fetches manga entries
// missing poster data (poster_id IS NULL). This ensures manga added before poster
// caching was fixed are backfilled so they display correctly in tierlists etc.
func (s *Service) refreshIncompleteManga() {
	markerPath := path.Join(config.DataPath, ".manga_poster_refresh_done")
	if _, err := os.Stat(markerPath); err == nil {
		return // Already done
	}
	var mangas []entity.Manga
	s.db.Where("poster_id IS NULL AND poster_url != ''").Find(&mangas)
	if len(mangas) == 0 {
		os.WriteFile(markerPath, []byte("done"), 0644)
		return
	}
	slog.Info("refreshIncompleteManga: Found manga entries missing posters, refreshing...", "count", len(mangas))
	for _, m := range mangas {
		resp, err := s.jikan.MangaDetails(m.MalID)
		if err != nil {
			slog.Error("refreshIncompleteManga: Failed to fetch manga details", "malId", m.MalID, "error", err)
			continue
		}
		if _, err := s.cacheManga(resp); err != nil {
			slog.Error("refreshIncompleteManga: Failed to re-cache manga", "malId", m.MalID, "error", err)
		} else {
			slog.Info("refreshIncompleteManga: Refreshed manga", "malId", m.MalID, "title", m.Title)
		}
		// Jikan has rate limits, small delay between requests
		time.Sleep(500 * time.Millisecond)
	}
	os.WriteFile(markerPath, []byte("done"), 0644)
	slog.Info("refreshIncompleteManga: Migration complete.")
}

// Cache(save) manga to our table
func (s *Service) saveManga(c *entity.Manga) error {
	slog.Info("Saving manga to db", "id", c.MalID, "title", c.Title)
	if c.MalID == 0 || c.Title == "" {
		slog.Error("saveManga: manga missing id or title!", "id", c.MalID, "title", c.Title)
		return errors.New("manga missing id or title")
	}
	if c.PosterURL != "" {
		p, err := image.DownloadAndInsertImage(s.db, c.PosterURL, "manga")
		if err != nil {
			slog.Error("saveManga: Failed to cache manga cover.", "error", err)
		} else {
			slog.Debug("saveManga: Cached manga cover", "p", p)
			c.PosterID = &p.ID
		}
	}
	// On conflict, update existing row with details incase any were updated/missing.
	res := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "mal_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"title",
			"poster_url",
			"poster_id",
			"synopsis",
			"release_date",
			"score",
			"scored_by",
			"status",
			"chapters",
			"volumes",
			"authors",
			"genres",
		}),
	}).Create(&c)
	if res.Error != nil {
		if res.Error != gorm.ErrDuplicatedKey {
			slog.Error("saveManga: Error creating manga in database", "error", res.Error.Error())
			return errors.New("failed to cache manga in database")
		}
	}
	return nil
}

func (s *Service) cacheManga(m jikan.JikanManga) (entity.Manga, error) {
	slog.Debug("cacheManga", "mal_id", m.MalID, "title", m.GetDisplayTitle())
	var (
		authors string
		genres  string
	)
	for _, a := range m.Authors {
		authors += a.Name + "|"
	}
	for _, g := range m.Genres {
		genres += g.Name + "|"
	}
	for _, g := range m.Themes {
		genres += g.Name + "|"
	}

	var posterURL string
	if m.Images.JPG.LargeImageURL != "" {
		posterURL = m.Images.JPG.LargeImageURL
	} else if m.Images.JPG.ImageURL != "" {
		posterURL = m.Images.JPG.ImageURL
	}

	var releaseDate *time.Time
	if m.Published.From != nil {
		releaseDate = m.Published.From
	}

	c := entity.Manga{
		MalID:       m.MalID,
		Title:       m.GetDisplayTitle(),
		PosterURL:   posterURL,
		Synopsis:    m.Synopsis,
		ReleaseDate: releaseDate,
		Score:       m.Score,
		ScoredBy:    m.ScoredBy,
		Status:      m.Status,
		Chapters:    m.Chapters,
		Volumes:     m.Volumes,
		Authors:     strings.TrimRight(authors, "|"),
		Genres:      strings.TrimRight(genres, "|"),
	}
	err := s.saveManga(&c)
	if err != nil {
		slog.Error("cacheManga: Failed to save manga!", "error", err)
		return entity.Manga{}, errors.New("failed to save manga")
	}
	return c, nil
}

func (s *Service) GetOrCache(malID int) (entity.Manga, error) {
	var manga entity.Manga
	s.db.Preload("Poster").Where("mal_id = ?", malID).Find(&manga)

	if manga == (entity.Manga{}) {
		slog.Debug("GetOrCache: Manga not in db, fetching...")
		return s.fetchAndCache(malID)
	}

	// Re-fetch if manga is missing poster data
	if manga.PosterID == nil && manga.PosterURL != "" {
		slog.Info("GetOrCache: Manga missing cached poster, re-fetching...", "mal_id", malID)
		refreshed, err := s.fetchAndCache(malID)
		if err == nil {
			return refreshed, nil
		}
		slog.Error("GetOrCache: Failed to refresh manga, returning existing data", "error", err)
	}

	return manga, nil
}

func (s *Service) fetchAndCache(malID int) (entity.Manga, error) {
	resp, err := s.jikan.MangaDetails(malID)
	if err != nil {
		slog.Error("fetchAndCache: jikan api request failed", "error", err)
		return entity.Manga{}, errors.New("failed to find requested manga")
	}

	manga, err := s.cacheManga(resp)
	if err != nil {
		slog.Error("fetchAndCache: failed to cache manga",
			"mal_id", malID,
			"err", err)
		return manga, errors.New("failed to cache manga")
	}
	return manga, nil
}
