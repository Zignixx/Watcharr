package jikan

import (
	"time"

	"github.com/sbondCo/Watcharr/domain"
)

type JikanSearchResponse struct {
	Pagination JikanPagination `json:"pagination"`
	Data       []JikanManga    `json:"data"`
}

type JikanPagination struct {
	LastVisiblePage int  `json:"last_visible_page"`
	HasNextPage     bool `json:"has_next_page"`
	CurrentPage     int  `json:"current_page"`
	Items           struct {
		Count   int `json:"count"`
		Total   int `json:"total"`
		PerPage int `json:"per_page"`
	} `json:"items"`
}

type JikanTitle struct {
	Type  string `json:"type"`
	Title string `json:"title"`
}

type JikanImages struct {
	JPG struct {
		ImageURL      string `json:"image_url"`
		SmallImageURL string `json:"small_image_url"`
		LargeImageURL string `json:"large_image_url"`
	} `json:"jpg"`
	WebP struct {
		ImageURL      string `json:"image_url"`
		SmallImageURL string `json:"small_image_url"`
		LargeImageURL string `json:"large_image_url"`
	} `json:"webp"`
}

type JikanPublished struct {
	From   *time.Time `json:"from"`
	To     *time.Time `json:"to"`
	String string     `json:"string"`
}

type JikanAuthor struct {
	MalID int    `json:"mal_id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	URL   string `json:"url"`
}

type JikanGenre struct {
	MalID int    `json:"mal_id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
}

type JikanManga struct {
	MalID          int            `json:"mal_id"`
	URL            string         `json:"url"`
	Images         JikanImages    `json:"images"`
	Titles         []JikanTitle   `json:"titles"`
	Title          string         `json:"title"`
	TitleEnglish   string         `json:"title_english"`
	TitleJapanese  string         `json:"title_japanese"`
	Type           string         `json:"type"`
	Chapters       int            `json:"chapters"`
	Volumes        int            `json:"volumes"`
	Status         string         `json:"status"`
	Publishing     bool           `json:"publishing"`
	Published      JikanPublished `json:"published"`
	Score          float64        `json:"score"`
	ScoredBy       int            `json:"scored_by"`
	Rank           int            `json:"rank"`
	Popularity     int            `json:"popularity"`
	Members        int            `json:"members"`
	Favorites      int            `json:"favorites"`
	Synopsis       string         `json:"synopsis"`
	Background     string         `json:"background"`
	Authors        []JikanAuthor  `json:"authors"`
	Genres         []JikanGenre   `json:"genres"`
	Themes         []JikanGenre   `json:"themes"`
	Demographics   []JikanGenre   `json:"demographics"`
	Serializations []JikanGenre   `json:"serializations"`
}

func (t *JikanManga) AsMedia() domain.Media {
	m := domain.Media{
		Type: domain.MediaTypeMALManga,
		IDs: domain.MediaIDs{
			MAL: t.MalID,
		},
		Name:    t.GetDisplayTitle(),
		Summary: t.Synopsis,
		Rating:  uint(t.Score),
	}

	// Use the large JPG image as poster
	if t.Images.JPG.LargeImageURL != "" {
		m.ExtPosterPath = t.Images.JPG.LargeImageURL
	} else if t.Images.JPG.ImageURL != "" {
		m.ExtPosterPath = t.Images.JPG.ImageURL
	}

	if t.Published.From != nil {
		m.ReleaseDate = *t.Published.From
	}

	// Genres
	for _, g := range t.Genres {
		m.Genres = append(m.Genres, domain.MediaGenre{
			ID:   uint(g.MalID),
			Name: g.Name,
		})
	}
	for _, g := range t.Themes {
		m.Genres = append(m.Genres, domain.MediaGenre{
			ID:   uint(g.MalID),
			Name: g.Name,
		})
	}

	// Manga-specific fields
	m.MangaChapters = t.Chapters
	m.MangaVolumes = t.Volumes
	m.MangaStatus = t.Status
	m.MangaAuthors = t.GetAuthors()

	return m
}

func (t *JikanManga) GetDisplayTitle() string {
	// Prefer English title, then default
	if t.TitleEnglish != "" {
		return t.TitleEnglish
	}
	return t.Title
}

func (t *JikanManga) GetAuthors() []string {
	authors := make([]string, 0, len(t.Authors))
	for _, a := range t.Authors {
		authors = append(authors, a.Name)
	}
	return authors
}
