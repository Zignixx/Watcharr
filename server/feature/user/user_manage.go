package user

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"gorm.io/gorm"
)

type ManageService struct {
	db *gorm.DB
}

func NewManageService(db *gorm.DB) *ManageService {
	return &ManageService{
		db,
	}
}

func (s *ManageService) GetAll() ([]domain.ManagedUser, error) {
	users := []domain.ManagedUser{}
	if res := s.db.Model(&entity.User{}).Find(&users); res.Error != nil {
		slog.Error("GetAllUsers: Failed to fetch users from database", "error", res.Error)
		return []domain.ManagedUser{}, errors.New("failed to fetch users from database")
	}
	return users, nil
}

// Update a user. For management views, for admin to update another user.
func (s *ManageService) Manage(userId uint, ur domain.UpdateUserRequest) error {
	// Error now if no userId or any UpdateUserRequest property was provided.
	if userId == 0 || (ur.Permissions == nil && ur.Type == nil && ur.Username == nil) {
		slog.Error("ManageUser: invalid arguments", "user_id", userId)
		return errors.New("invalid arguments, ensure a valid userId and at least one property has been provided for updating")
	}
	toUpdate := map[string]interface{}{}
	if ur.Permissions != nil {
		if *ur.Permissions == 0 {
			// If removing all perms, set to default of 1 (PERM_NONE).
			// Will avoid confusion and possibly bugs later on, though I doubt
			// we'd ever be (directly) checking a user to ensure they have no perms.
			toUpdate["permissions"] = entity.PERM_NONE
		} else {
			toUpdate["permissions"] = *ur.Permissions
		}
	}
	if ur.Type != nil {
		t := *ur.Type
		if t == entity.WATCHARR_USER || t == entity.PROXY_USER {
			// Currently only swapping between watcharr/proxy user is supported.
			slog.Debug("ManageUser: User type is being updated.", "new_type", t)
			toUpdate["type"] = t
		} else {
			slog.Warn("ManageUser: User type will not be updated. Only watcharr/proxy types are supported for swapping.", "tried_type", t)
		}
	}
	if ur.Username != nil {
		newUsername := strings.TrimSpace(*ur.Username)
		if len(newUsername) == 0 || len(newUsername) > 50 {
			return errors.New("username must be between 1 and 50 characters")
		}
		for _, c := range newUsername {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
				return errors.New("username can only contain letters, numbers, underscores and hyphens")
			}
		}
		// Check if username is already taken (by another user of the same type)
		var existing entity.User
		res := s.db.Where("LOWER(username) = LOWER(?) AND id != ?", newUsername, userId).Take(&existing)
		if res.Error == nil {
			return errors.New("username is already taken")
		}
		toUpdate["username"] = newUsername
	}
	if res := s.db.Model(&entity.User{}).Where("id = ?", userId).Updates(toUpdate); res.Error != nil {
		slog.Error("ManageUser: failed to update user in database", "user_id", userId, "error", res.Error)
		return errors.New("failed to update user in database")
	}
	slog.Debug("ManageUser: A user has been updated", "user_id", userId)
	return nil
}

// Delete a user and all their associated data (soft delete).
func (s *ManageService) Delete(userId uint) error {
	if userId == 0 {
		return errors.New("invalid user id")
	}

	// Delete related data first
	tables := []struct {
		model interface{}
		desc  string
	}{
		{&entity.Watched{}, "watched items"},
		{&entity.WatchedSeason{}, "watched seasons"},
		{&entity.WatchedEpisode{}, "watched episodes"},
		{&entity.Tag{}, "tags"},
		{&entity.Tier{}, "tiers"},
		{&entity.TierItem{}, "tier items"},
		{&entity.Tierlist{}, "tierlists"},
		{&entity.Activity{}, "activities"},
		{&entity.Follow{}, "follows"},
	}

	for _, t := range tables {
		if res := s.db.Where("user_id = ?", userId).Delete(t.model); res.Error != nil {
			slog.Error("DeleteUser: failed to delete "+t.desc, "user_id", userId, "error", res.Error)
		}
	}

	// Also delete follows where this user is being followed
	if res := s.db.Where("followed_user_id = ?", userId).Delete(&entity.Follow{}); res.Error != nil {
		slog.Error("DeleteUser: failed to delete reverse follows", "user_id", userId, "error", res.Error)
	}

	// Delete the user itself
	if res := s.db.Where("id = ?", userId).Delete(&entity.User{}); res.Error != nil {
		slog.Error("DeleteUser: failed to delete user", "user_id", userId, "error", res.Error)
		return errors.New("failed to delete user")
	}

	slog.Info("DeleteUser: User has been deleted", "user_id", userId)
	return nil
}
