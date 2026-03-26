package follow

import (
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/sbondCo/Watcharr/database/entity"
	"gorm.io/gorm"
)

// For end users to see.
type FollowPublic struct {
	CreatedAt    time.Time         `json:"createdAt"`
	FollowedUser entity.PublicUser `json:"followedUser"`
}

type FollowThoughts struct {
	FollowedUser entity.PublicUser    `json:"followedUser"`
	Thoughts     string               `json:"thoughts"`
	Status       entity.WatchedStatus `json:"status"`
	Rating       float64              `json:"rating"`
}

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		db,
	}
}

func (s *Service) FollowUser(currentUserId uint, toFollowUserId uint) (FollowPublic, error) {
	f := entity.Follow{UserID: currentUserId, FollowedUserID: toFollowUserId}
	res := s.db.Model(&entity.Follow{}).Create(&f)
	if res.Error != nil {
		slog.Error("followUser: Error on inserting follow.", "error", res.Error)
		err := "failed to insert follow"
		if res.Error == gorm.ErrDuplicatedKey {
			err = "already followed"
		}
		return FollowPublic{}, errors.New(err)
	}
	// Now get the row with preloaded followed user
	var nf entity.Follow
	res = s.db.Where("user_id = ? AND followed_user_id = ?", currentUserId, toFollowUserId).Preload("FollowedUser", "private = ?", 0).Take(&nf)
	if res.Error != nil {
		slog.Error("followUser: Couldn't fetch newly followed user.", "error", res.Error)
		return FollowPublic{}, errors.New("followed, but failed to fetch followed user")
	}
	return FollowPublic{CreatedAt: nf.CreatedAt, FollowedUser: nf.FollowedUser.GetSafe()}, nil
}

func (s *Service) UnfollowUser(currentUserId uint, toFollowUserId uint) (bool, error) {
	f := entity.Follow{UserID: currentUserId, FollowedUserID: toFollowUserId}
	res := s.db.Delete(&f)
	if res.Error != nil {
		slog.Error("unfollowUser: Error deleting follow.", "error", res.Error)
		err := "failed to remove follow"
		if res.Error == gorm.ErrRecordNotFound {
			err = "not following"
		}
		return false, errors.New(err)
	}
	return true, nil
}

// Get current users follows
func (s *Service) GetFollows(userId uint) ([]FollowPublic, error) {
	var follows []entity.Follow
	res := s.db.Where("user_id = ?", userId).Preload("FollowedUser", "private = ?", 0).Find(&follows)
	if res.Error != nil {
		slog.Error("getFollows: Error finding follows.", "error", res.Error)
		return []FollowPublic{}, errors.New("failed to find follows")
	}
	fpub := []FollowPublic{}
	for _, v := range follows {
		// Skip followed users without an ID..
		// this will be because they have made
		// their account private after we followed them.
		if v.FollowedUser.ID == 0 {
			continue
		}
		fpub = append(fpub, FollowPublic{CreatedAt: v.CreatedAt, FollowedUser: v.FollowedUser.GetSafe()})
	}
	return fpub, nil
}

// IsFollowing checks whether currentUserId follows targetUserId.
func (s *Service) IsFollowing(currentUserId uint, targetUserId uint) bool {
	var count int64
	s.db.Model(&entity.Follow{}).
		Where("user_id = ? AND followed_user_id = ?", currentUserId, targetUserId).
		Count(&count)
	return count > 0
}

// Get followed profile thoughts, rating, etc on specific content.
func (s *Service) GetFollowsThoughts(userId uint, mediaType string, mediaId string) ([]FollowThoughts, error) {
	var follows []entity.Follow
	res := s.db.Where("user_id = ?", userId).Preload("FollowedUser", "private = ? AND private_thoughts = ?", 0, 0).Find(&follows)
	if res.Error != nil {
		slog.Error("getFollows: Error finding follows.", "error", res.Error)
		return []FollowThoughts{}, errors.New("failed to find follows")
	}
	slog.Info("getFollowsThoughts")
	var followIds []uint
	for _, v := range follows {
		// Skip empty followedUsers.. they are private.
		if v.FollowedUser.ID == 0 {
			continue
		}
		followIds = append(followIds, v.FollowedUser.ID)
	}
	var contentOrGameId int
	if mediaType == "game" {
		// Get our content id from type and tmdbId
		var content entity.Game
		res = s.db.Where("igdb_id = ?", mediaId).Select("id").Find(&content)
		if res.Error != nil {
			slog.Error("getFollows: Error finding content from db.", "error", res.Error)
			return []FollowThoughts{}, errors.New("failed to find content")
		}
		contentOrGameId = content.ID
	} else if mediaType == "manga" {
		var content entity.Manga
		res = s.db.Where("mal_id = ?", mediaId).Select("id").Find(&content)
		if res.Error != nil {
			slog.Error("getFollows: Error finding manga from db.", "error", res.Error)
			return []FollowThoughts{}, errors.New("failed to find manga")
		}
		contentOrGameId = content.ID
	} else if mediaType == "movie" || mediaType == "tv" {
		// Get our content id from type and tmdbId
		var content entity.Content
		res = s.db.Where("type = ? AND tmdb_id = ?", mediaType, mediaId).Select("id").Find(&content)
		if res.Error != nil {
			slog.Error("getFollows: Error finding content from db.", "error", res.Error)
			return []FollowThoughts{}, errors.New("failed to find content")
		}
		contentOrGameId = content.ID
	} else {
		slog.Error("getFollows: Unrecognized media type (movie, tv, game or manga supported).", "media_type", mediaType)
		return []FollowThoughts{}, errors.New("unrecognized media type")
	}
	// Get list of followeds watcheds for this content
	var fw []entity.Watched
	if mediaType == "game" {
		res = s.db.Where("game_id = ? AND user_id IN ?", contentOrGameId, followIds).Find(&fw)
	} else if mediaType == "manga" {
		res = s.db.Where("manga_id = ? AND user_id IN ?", contentOrGameId, followIds).Find(&fw)
	} else {
		res = s.db.Where("content_id = ? AND user_id IN ?", contentOrGameId, followIds).Find(&fw)
	}
	if res.Error != nil {
		slog.Error("getFollows: Error finding followed watcheds from db.", "error", res.Error)
		return []FollowThoughts{}, errors.New("failed to find followed watcheds")
	}
	// Create followThoughts array by combining follows and fw(atcheds)
	ft := []FollowThoughts{}
	for _, v := range fw {
		var fu entity.PublicUser
		for _, f := range follows {
			if f.FollowedUser.ID == v.UserID {
				fu = f.FollowedUser.GetSafe()
				break
			}
		}
		// If we didn't find a related followedUser.. skip this watched entry
		if fu.ID == 0 {
			continue
		}
		ft = append(ft, FollowThoughts{FollowedUser: fu, Thoughts: v.Thoughts, Status: v.Status, Rating: v.Rating})
	}
	return ft, nil
}

// ContentKey is a struct for identifying content items in batch requests.
type ContentKey struct {
	Type string `json:"type"` // "movie", "tv", "game", "manga"
	ID   int    `json:"id"`   // tmdbId, igdbId, or malId
}

// SocialStatus represents a followed user's status for a content item.
type SocialStatus struct {
	FollowedUser entity.PublicUser    `json:"followedUser"`
	Status       entity.WatchedStatus `json:"status"`
}

// GetBatchSocialStatuses returns social status info for multiple content items at once.
// The result maps "type_id" (e.g. "movie_123") to a slice of SocialStatus.
func (s *Service) GetBatchSocialStatuses(userId uint, items []ContentKey) (map[string][]SocialStatus, error) {
	result := make(map[string][]SocialStatus)

	if len(items) == 0 {
		return result, nil
	}

	// Get followed users
	var follows []entity.Follow
	res := s.db.Where("user_id = ?", userId).Preload("FollowedUser", "private = ?", 0).Find(&follows)
	if res.Error != nil {
		slog.Error("getBatchSocialStatuses: Error finding follows.", "error", res.Error)
		return result, errors.New("failed to find follows")
	}

	var followIds []uint
	followMap := make(map[uint]entity.User)
	for _, v := range follows {
		if v.FollowedUser.ID == 0 {
			continue
		}
		followIds = append(followIds, v.FollowedUser.ID)
		followMap[v.FollowedUser.ID] = v.FollowedUser
	}

	if len(followIds) == 0 {
		return result, nil
	}

	// Group items by type for batch DB lookups
	movieTvItems := make(map[int]string) // tmdbId -> type ("movie"/"tv")
	var gameIDs []int
	var mangaIDs []int

	for _, item := range items {
		switch item.Type {
		case "movie", "tv":
			movieTvItems[item.ID] = item.Type
		case "game":
			gameIDs = append(gameIDs, item.ID)
		case "manga":
			mangaIDs = append(mangaIDs, item.ID)
		}
	}

	// Resolve content IDs for movie/tv
	if len(movieTvItems) > 0 {
		var contents []entity.Content
		var tmdbIds []int
		for id := range movieTvItems {
			tmdbIds = append(tmdbIds, id)
		}
		s.db.Where("tmdb_id IN ?", tmdbIds).Select("id, tmdb_id, type").Find(&contents)

		var contentIds []int
		contentIdMap := make(map[int]string) // contentId -> "type_tmdbId"
		for _, c := range contents {
			contentIds = append(contentIds, c.ID)
			contentIdMap[c.ID] = string(c.Type) + "_" + strconv.Itoa(c.TmdbID)
		}

		if len(contentIds) > 0 {
			var watcheds []entity.Watched
			s.db.Where("content_id IN ? AND user_id IN ?", contentIds, followIds).Find(&watcheds)
			for _, w := range watcheds {
				if w.ContentID == nil {
					continue
				}
				key, ok := contentIdMap[*w.ContentID]
				if !ok {
					continue
				}
				fu, ok := followMap[w.UserID]
				if !ok {
					continue
				}
				result[key] = append(result[key], SocialStatus{
					FollowedUser: fu.GetSafe(),
					Status:       w.Status,
				})
			}
		}
	}

	// Resolve game IDs
	if len(gameIDs) > 0 {
		var games []entity.Game
		s.db.Where("igdb_id IN ?", gameIDs).Select("id, igdb_id").Find(&games)

		var dbGameIds []int
		gameIdMap := make(map[int]int) // db game id -> igdbId
		for _, g := range games {
			dbGameIds = append(dbGameIds, g.ID)
			gameIdMap[g.ID] = g.IgdbID
		}

		if len(dbGameIds) > 0 {
			var watcheds []entity.Watched
			s.db.Where("game_id IN ? AND user_id IN ?", dbGameIds, followIds).Find(&watcheds)
			for _, w := range watcheds {
				if w.GameID == nil {
					continue
				}
				igdbId, ok := gameIdMap[*w.GameID]
				if !ok {
					continue
				}
				fu, ok := followMap[w.UserID]
				if !ok {
					continue
				}
				key := "game_" + strconv.Itoa(igdbId)
				result[key] = append(result[key], SocialStatus{
					FollowedUser: fu.GetSafe(),
					Status:       w.Status,
				})
			}
		}
	}

	// Resolve manga IDs
	if len(mangaIDs) > 0 {
		var mangas []entity.Manga
		s.db.Where("mal_id IN ?", mangaIDs).Select("id, mal_id").Find(&mangas)

		var dbMangaIds []int
		mangaIdMap := make(map[int]int) // db manga id -> malId
		for _, m := range mangas {
			dbMangaIds = append(dbMangaIds, m.ID)
			mangaIdMap[m.ID] = m.MalID
		}

		if len(dbMangaIds) > 0 {
			var watcheds []entity.Watched
			s.db.Where("manga_id IN ? AND user_id IN ?", dbMangaIds, followIds).Find(&watcheds)
			for _, w := range watcheds {
				if w.MangaID == nil {
					continue
				}
				malId, ok := mangaIdMap[*w.MangaID]
				if !ok {
					continue
				}
				fu, ok := followMap[w.UserID]
				if !ok {
					continue
				}
				key := "manga_" + strconv.Itoa(malId)
				result[key] = append(result[key], SocialStatus{
					FollowedUser: fu.GetSafe(),
					Status:       w.Status,
				})
			}
		}
	}

	return result, nil
}
