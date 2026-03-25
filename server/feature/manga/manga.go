package manga

import (
	"errors"
	"log/slog"
	"strings"
	"time"

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
	return &Service{
		db,
		jikan,
		activityProvider,
	}
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

		resp, err := s.jikan.MangaDetails(malID)
		if err != nil {
			slog.Error("GetOrCache: jikan api request failed", "error", err)
			return manga, errors.New("failed to find requested manga")
		}

		manga, err = s.cacheManga(resp)
		if err != nil {
			slog.Error("GetOrCache: failed to cache manga",
				"mal_id", malID,
				"err", err)
			return manga, errors.New("failed to cache manga")
		}
	}

	return manga, nil
}
