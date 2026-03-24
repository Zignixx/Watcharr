package tierlist

import (
	"errors"
	"log/slog"

	"github.com/sbondCo/Watcharr/database/entity"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// GetTiers returns all tiers with their items for a user, ordered by position.
func (s *Service) GetTiers(userID uint) ([]entity.Tier, error) {
	var tiers []entity.Tier
	err := s.db.Where("user_id = ?", userID).
		Order("position ASC").
		Preload("TierItems", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("TierItems.Watched").
		Preload("TierItems.Watched.Content").
		Preload("TierItems.Watched.Game").
		Preload("TierItems.Watched.Game.Poster").
		Find(&tiers).Error
	return tiers, err
}

// GetPublicTiers returns a public user's tierlist (checks privacy).
func (s *Service) GetPublicTiers(userID uint, username string) ([]entity.Tier, error) {
	user := new(entity.User)
	res := s.db.Where("id = ? AND username = ?", userID, username).Take(&user)
	if res.Error != nil {
		return nil, errors.New("failed to find user")
	}
	if user.Private != nil && *user.Private {
		return nil, errors.New("this user's list is private")
	}
	return s.GetTiers(userID)
}

// CreateTier creates a new tier for a user.
func (s *Service) CreateTier(userID uint, req CreateTierRequest) (*entity.Tier, error) {
	// Get max position
	var maxPos int
	s.db.Model(&entity.Tier{}).Where("user_id = ?", userID).
		Select("COALESCE(MAX(position), -1)").Scan(&maxPos)

	tier := entity.Tier{
		UserID:    userID,
		Name:      req.Name,
		Color:     req.Color,
		TextColor: req.TextColor,
		Position:  maxPos + 1,
	}
	if err := s.db.Create(&tier).Error; err != nil {
		return nil, err
	}
	return &tier, nil
}

// UpdateTier updates a tier's properties.
func (s *Service) UpdateTier(userID uint, tierID uint, req UpdateTierRequest) error {
	result := s.db.Model(&entity.Tier{}).
		Where("id = ? AND user_id = ?", tierID, userID).
		Updates(map[string]interface{}{
			"name":       req.Name,
			"color":      req.Color,
			"text_color": req.TextColor,
		})
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}

// DeleteTier deletes a tier and all its items.
func (s *Service) DeleteTier(userID uint, tierID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete all tier items first
		if err := tx.Where("tier_id = ?", tierID).Delete(&entity.TierItem{}).Error; err != nil {
			return err
		}
		// Delete the tier
		result := tx.Where("id = ? AND user_id = ?", tierID, userID).Delete(&entity.Tier{})
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return result.Error
	})
}

// ReorderTiers updates the position of all tiers for a user.
func (s *Service) ReorderTiers(userID uint, tierIDs []uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range tierIDs {
			if err := tx.Model(&entity.Tier{}).
				Where("id = ? AND user_id = ?", id, userID).
				Update("position", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateTierItems bulk updates all tier item assignments.
// Items is a map of tierID -> []watchedID (in order).
func (s *Service) UpdateTierItems(userID uint, items map[uint][]uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// First, verify all tiers belong to user
		var tierIDs []uint
		for tierID := range items {
			tierIDs = append(tierIDs, tierID)
		}
		var count int64
		tx.Model(&entity.Tier{}).Where("id IN ? AND user_id = ?", tierIDs, userID).Count(&count)
		if int(count) != len(tierIDs) {
			return gorm.ErrRecordNotFound
		}

		// Delete existing items for these tiers
		if err := tx.Where("tier_id IN ?", tierIDs).Delete(&entity.TierItem{}).Error; err != nil {
			return err
		}

		// Insert new items
		for tierID, watchedIDs := range items {
			for pos, watchedID := range watchedIDs {
				item := entity.TierItem{
					TierID:    tierID,
					WatchedID: watchedID,
					Position:  pos,
				}
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// SyncRatings maps tier positions to ratings (0-10) for backwards compatibility.
func (s *Service) SyncRatings(userID uint) error {
	tiers, err := s.GetTiers(userID)
	if err != nil {
		return err
	}
	if len(tiers) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		totalTiers := len(tiers)
		for i, tier := range tiers {
			// Map tier position to rating range: top tier = 10, bottom tier = 1
			// Each tier gets a rating band
			tierRating := 10.0 - (float64(i) * 10.0 / float64(totalTiers))
			if tierRating < 1 {
				tierRating = 1
			}

			for _, item := range tier.TierItems {
				if err := tx.Model(&entity.Watched{}).
					Where("id = ? AND user_id = ?", item.WatchedID, userID).
					Update("rating", tierRating).Error; err != nil {
					slog.Error("SyncRatings: Failed to update rating", "watchedId", item.WatchedID, "error", err)
				}
			}
		}
		return nil
	})
}

// CreateDefaultTiers creates the default S/A/B/C/D tiers for a user.
func (s *Service) CreateDefaultTiers(userID uint) ([]entity.Tier, error) {
	defaults := []entity.Tier{
		{UserID: userID, Name: "S", Color: "#FF7F7F", TextColor: "#000000", Position: 0},
		{UserID: userID, Name: "A", Color: "#FFBF7F", TextColor: "#000000", Position: 1},
		{UserID: userID, Name: "B", Color: "#FFDF7F", TextColor: "#000000", Position: 2},
		{UserID: userID, Name: "C", Color: "#FFFF7F", TextColor: "#000000", Position: 3},
		{UserID: userID, Name: "D", Color: "#BFFF7F", TextColor: "#000000", Position: 4},
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		for i := range defaults {
			if err := tx.Create(&defaults[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return defaults, nil
}

// GetUntieredWatched returns all watched items that are NOT in any tier for this user.
func (s *Service) GetUntieredWatched(userID uint) ([]entity.Watched, error) {
	var watched []entity.Watched
	err := s.db.Where("user_id = ? AND id NOT IN (?)",
		userID,
		s.db.Model(&entity.TierItem{}).
			Select("watched_id").
			Joins("JOIN tiers ON tiers.id = tier_items.tier_id").
			Where("tiers.user_id = ? AND tier_items.deleted_at IS NULL AND tiers.deleted_at IS NULL", userID),
	).
		Preload("Content").
		Preload("Game").
		Preload("Game.Poster").
		Find(&watched).Error
	return watched, err
}

// GetAllPresets returns all user-created presets, ordered newest first.
func (s *Service) GetAllPresets() ([]entity.TierPreset, error) {
	var presets []entity.TierPreset
	err := s.db.
		Preload("Tiers", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Order("created_at DESC").
		Find(&presets).Error
	return presets, err
}

// CreatePreset creates a new shared preset.
func (s *Service) CreatePreset(userID uint, req CreatePresetRequest) (*entity.TierPreset, error) {
	preset := entity.TierPreset{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
	}
	for i, t := range req.Tiers {
		preset.Tiers = append(preset.Tiers, entity.TierPresetEntry{
			Name:      t.Name,
			Color:     t.Color,
			TextColor: t.TextColor,
			Position:  i,
		})
	}
	if err := s.db.Create(&preset).Error; err != nil {
		return nil, err
	}
	return &preset, nil
}

// DeletePreset deletes a preset if it belongs to the given user.
func (s *Service) DeletePreset(userID uint, presetID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("preset_id = ?", presetID).Delete(&entity.TierPresetEntry{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND user_id = ?", presetID, userID).Delete(&entity.TierPreset{})
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return result.Error
	})
}
