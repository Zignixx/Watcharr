package entity

import "time"

// For storing cached manga, so we can serve the basic local data for watched list to work
type Manga struct {
	ID        int       `json:"id" gorm:"primaryKey;autoIncrement"`
	UpdatedAt time.Time `json:"updatedAt"`
	MalID     int       `json:"malId" gorm:"uniqueIndex;not null"`
	Title     string    `json:"title"`
	// Full URL to the poster image (from Jikan/MAL)
	PosterURL string `json:"posterUrl"`
	Synopsis  string `json:"synopsis"`
	// Publishing start date
	ReleaseDate *time.Time `json:"releaseDate,omitempty"`
	Score       float64    `json:"score"`
	ScoredBy    int        `json:"scoredBy"`
	Status      string     `json:"status"`
	Chapters    int        `json:"chapters"`
	Volumes     int        `json:"volumes"`
	// Pipe-delimited strings
	Authors string `json:"authors"`
	Genres  string `json:"genres"`
	// Id to poster image row (cached manga cover)
	PosterID *uint  `json:"-"`
	Poster   *Image `json:"poster,omitempty"`
}
