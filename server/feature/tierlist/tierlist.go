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
	s := &Service{db: db}
	s.migrateExistingTiers()
	return s
}

// migrateExistingTiers creates a default Tierlist for users who have tiers without a tierlist_id.
func (s *Service) migrateExistingTiers() {
	// Find distinct user IDs that have tiers with tierlist_id = 0
	var userIDs []uint
	s.db.Model(&entity.Tier{}).
		Where("tierlist_id = 0 OR tierlist_id IS NULL").
		Distinct("user_id").
		Pluck("user_id", &userIDs)

	for _, uid := range userIDs {
		slog.Info("migrateExistingTiers: Creating default tierlist for user", "userId", uid)
		tl := entity.Tierlist{
			UserID:   uid,
			Name:     "Tierlist",
			Position: 0,
		}
		if err := s.db.Create(&tl).Error; err != nil {
			slog.Error("migrateExistingTiers: Failed to create tierlist", "userId", uid, "error", err)
			continue
		}
		// Update all orphan tiers for this user to belong to the new tierlist
		s.db.Model(&entity.Tier{}).
			Where("user_id = ? AND (tierlist_id = 0 OR tierlist_id IS NULL)", uid).
			Update("tierlist_id", tl.ID)
	}
}

// ==================== Tierlist CRUD ====================

// GetTierlists returns all tierlists for a user, ordered by position.
func (s *Service) GetTierlists(userID uint) ([]entity.Tierlist, error) {
	var lists []entity.Tierlist
	err := s.db.Where("user_id = ?", userID).
		Order("position ASC").
		Find(&lists).Error
	return lists, err
}

// GetPublicTierlists returns a public user's tierlists (checks privacy).
func (s *Service) GetPublicTierlists(userID uint, username string) ([]entity.Tierlist, error) {
	user := new(entity.User)
	res := s.db.Where("id = ? AND username = ?", userID, username).Take(&user)
	if res.Error != nil {
		return nil, errors.New("failed to find user")
	}
	if user.Private != nil && *user.Private {
		return nil, errors.New("this user's list is private")
	}
	var lists []entity.Tierlist
	err := s.db.Where("user_id = ?", userID).
		Order("position ASC").
		Find(&lists).Error
	return lists, err
}

// CreateTierlist creates a new tierlist for a user.
func (s *Service) CreateTierlist(userID uint, req CreateTierlistRequest) (*entity.Tierlist, error) {
	var maxPos int
	s.db.Model(&entity.Tierlist{}).Where("user_id = ?", userID).
		Select("COALESCE(MAX(position), -1)").Scan(&maxPos)

	tl := entity.Tierlist{
		UserID:   userID,
		Name:     req.Name,
		Position: maxPos + 1,
	}
	if err := s.db.Create(&tl).Error; err != nil {
		return nil, err
	}
	return &tl, nil
}

// UpdateTierlist updates a tierlist's name.
func (s *Service) UpdateTierlist(userID uint, tierlistID uint, req UpdateTierlistRequest) error {
	result := s.db.Model(&entity.Tierlist{}).
		Where("id = ? AND user_id = ?", tierlistID, userID).
		Update("name", req.Name)
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}

// DeleteTierlist deletes a tierlist and all its tiers and items.
func (s *Service) DeleteTierlist(userID uint, tierlistID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Get all tier IDs in this tierlist
		var tierIDs []uint
		tx.Model(&entity.Tier{}).
			Where("tierlist_id = ? AND user_id = ?", tierlistID, userID).
			Pluck("id", &tierIDs)

		// Delete tier items
		if len(tierIDs) > 0 {
			if err := tx.Where("tier_id IN ?", tierIDs).Delete(&entity.TierItem{}).Error; err != nil {
				return err
			}
		}

		// Delete tiers
		if err := tx.Where("tierlist_id = ? AND user_id = ?", tierlistID, userID).Delete(&entity.Tier{}).Error; err != nil {
			return err
		}

		// Delete the tierlist
		result := tx.Where("id = ? AND user_id = ?", tierlistID, userID).Delete(&entity.Tierlist{})
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return result.Error
	})
}

// DuplicateTierlist copies a tierlist with all its tiers and items.
func (s *Service) DuplicateTierlist(userID uint, tierlistID uint) (*entity.Tierlist, error) {
	// Get the source tierlist
	var src entity.Tierlist
	if err := s.db.Where("id = ? AND user_id = ?", tierlistID, userID).First(&src).Error; err != nil {
		return nil, errors.New("tierlist not found")
	}

	// Get max position for the new tierlist
	var maxPos int
	s.db.Model(&entity.Tierlist{}).Where("user_id = ?", userID).
		Select("COALESCE(MAX(position), -1)").Scan(&maxPos)

	var newTL entity.Tierlist
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Create the new tierlist
		newTL = entity.Tierlist{
			UserID:   userID,
			Name:     src.Name + " (Copy)",
			Position: maxPos + 1,
		}
		if err := tx.Create(&newTL).Error; err != nil {
			return err
		}

		// Get source tiers with items
		var srcTiers []entity.Tier
		if err := tx.Where("tierlist_id = ? AND user_id = ?", tierlistID, userID).
			Order("position ASC").
			Preload("TierItems", func(db *gorm.DB) *gorm.DB {
				return db.Order("position ASC")
			}).
			Find(&srcTiers).Error; err != nil {
			return err
		}

		// Copy each tier and its items
		for _, st := range srcTiers {
			newTier := entity.Tier{
				UserID:     userID,
				TierlistID: newTL.ID,
				Name:       st.Name,
				Color:      st.Color,
				TextColor:  st.TextColor,
				Position:   st.Position,
			}
			if err := tx.Create(&newTier).Error; err != nil {
				return err
			}
			for _, si := range st.TierItems {
				newItem := entity.TierItem{
					TierID:    newTier.ID,
					WatchedID: si.WatchedID,
					Position:  si.Position,
				}
				if err := tx.Create(&newItem).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &newTL, nil
}

// resolveListID returns the tierlist ID to use.
// If tierlistID > 0, returns it directly.
// Otherwise returns the first tierlist for the user (by position).
func (s *Service) resolveListID(userID uint, tierlistID uint) uint {
	if tierlistID > 0 {
		return tierlistID
	}
	var tl entity.Tierlist
	s.db.Where("user_id = ?", userID).Order("position ASC").First(&tl)
	return tl.ID
}

// ==================== Tier CRUD (scoped to tierlist) ====================

// GetTiers returns all tiers with their items for a user's tierlist, ordered by position.
func (s *Service) GetTiers(userID uint, tierlistID uint) ([]entity.Tier, error) {
	listID := s.resolveListID(userID, tierlistID)
	if listID == 0 {
		return []entity.Tier{}, nil
	}
	var tiers []entity.Tier
	err := s.db.Where("user_id = ? AND tierlist_id = ?", userID, listID).
		Order("position ASC").
		Preload("TierItems", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("TierItems.Watched").
		Preload("TierItems.Watched.Content").
		Preload("TierItems.Watched.Game").
		Preload("TierItems.Watched.Game.Poster").
		Preload("TierItems.Watched.Manga").
		Preload("TierItems.Watched.Manga.Poster").
		Find(&tiers).Error
	return tiers, err
}

// GetPublicTiers returns a public user's tierlist tiers (checks privacy).
func (s *Service) GetPublicTiers(userID uint, username string, tierlistID uint) ([]entity.Tier, error) {
	user := new(entity.User)
	res := s.db.Where("id = ? AND username = ?", userID, username).Take(&user)
	if res.Error != nil {
		return nil, errors.New("failed to find user")
	}
	if user.Private != nil && *user.Private {
		return nil, errors.New("this user's list is private")
	}
	return s.GetTiers(userID, tierlistID)
}

// CreateTier creates a new tier for a user in a specific tierlist.
func (s *Service) CreateTier(userID uint, req CreateTierRequest) (*entity.Tier, error) {
	listID := s.resolveListID(userID, req.TierlistID)
	if listID == 0 {
		return nil, errors.New("no tierlist found")
	}

	// Get max position within this tierlist
	var maxPos int
	s.db.Model(&entity.Tier{}).Where("user_id = ? AND tierlist_id = ?", userID, listID).
		Select("COALESCE(MAX(position), -1)").Scan(&maxPos)

	tier := entity.Tier{
		UserID:     userID,
		TierlistID: listID,
		Name:       req.Name,
		Color:      req.Color,
		TextColor:  req.TextColor,
		Position:   maxPos + 1,
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
		if err := tx.Where("tier_id = ?", tierID).Delete(&entity.TierItem{}).Error; err != nil {
			return err
		}
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
func (s *Service) UpdateTierItems(userID uint, items map[uint][]uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var tierIDs []uint
		for tierID := range items {
			tierIDs = append(tierIDs, tierID)
		}
		var count int64
		tx.Model(&entity.Tier{}).Where("id IN ? AND user_id = ?", tierIDs, userID).Count(&count)
		if int(count) != len(tierIDs) {
			return gorm.ErrRecordNotFound
		}

		if err := tx.Where("tier_id IN ?", tierIDs).Delete(&entity.TierItem{}).Error; err != nil {
			return err
		}

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

// SyncRatings maps tier positions to ratings for a specific tierlist.
func (s *Service) SyncRatings(userID uint, tierlistID uint) error {
	tiers, err := s.GetTiers(userID, tierlistID)
	if err != nil {
		return err
	}
	if len(tiers) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		totalTiers := len(tiers)
		for i, tier := range tiers {
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

// CreateDefaultTiers creates the default S/A/B/C/D tiers for a tierlist.
func (s *Service) CreateDefaultTiers(userID uint, tierlistID uint) ([]entity.Tier, error) {
	listID := s.resolveListID(userID, tierlistID)
	if listID == 0 {
		return nil, errors.New("no tierlist found")
	}

	defaults := []entity.Tier{
		{UserID: userID, TierlistID: listID, Name: "S", Color: "#FF7F7F", TextColor: "#000000", Position: 0},
		{UserID: userID, TierlistID: listID, Name: "A", Color: "#FFBF7F", TextColor: "#000000", Position: 1},
		{UserID: userID, TierlistID: listID, Name: "B", Color: "#FFDF7F", TextColor: "#000000", Position: 2},
		{UserID: userID, TierlistID: listID, Name: "C", Color: "#FFFF7F", TextColor: "#000000", Position: 3},
		{UserID: userID, TierlistID: listID, Name: "D", Color: "#BFFF7F", TextColor: "#000000", Position: 4},
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

// GetUntieredWatched returns watched items not in any tier of the specified tierlist.
func (s *Service) GetUntieredWatched(userID uint, tierlistID uint) ([]entity.Watched, error) {
	listID := s.resolveListID(userID, tierlistID)
	if listID == 0 {
		// No tierlist, return all watched items
		var watched []entity.Watched
		err := s.db.Where("user_id = ?", userID).
			Preload("Content").
			Preload("Game").
			Preload("Game.Poster").
			Preload("Manga").
			Preload("Manga.Poster").
			Find(&watched).Error
		return watched, err
	}

	var watched []entity.Watched
	err := s.db.Where("user_id = ? AND id NOT IN (?)",
		userID,
		s.db.Model(&entity.TierItem{}).
			Select("watched_id").
			Joins("JOIN tiers ON tiers.id = tier_items.tier_id").
			Where("tiers.user_id = ? AND tiers.tierlist_id = ? AND tier_items.deleted_at IS NULL AND tiers.deleted_at IS NULL", userID, listID),
	).
		Preload("Content").
		Preload("Game").
		Preload("Game.Poster").
		Preload("Manga").
		Preload("Manga.Poster").
		Find(&watched).Error
	return watched, err
}

// ==================== Presets ====================

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
