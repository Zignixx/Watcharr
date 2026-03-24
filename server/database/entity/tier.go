package entity

import "github.com/sbondCo/Watcharr/database/dbmodel"

type Tier struct {
	dbmodel.GormModel
	UserID    uint       `json:"-" gorm:"index"`
	Name      string     `json:"name" gorm:"not null"`
	Color     string     `json:"color" gorm:"not null;default:'#FF7F7F'"` // Background color
	TextColor string     `json:"textColor" gorm:"not null;default:'#000000'"`
	Position  int        `json:"position" gorm:"not null;default:0"`
	TierItems []TierItem `json:"tierItems,omitempty"`
}

type TierItem struct {
	dbmodel.GormModel
	TierID    uint     `json:"tierId" gorm:"index;not null"`
	WatchedID uint     `json:"watchedId" gorm:"index;not null"`
	Watched   *Watched `json:"watched,omitempty"`
	Position  int      `json:"position" gorm:"not null;default:0"`
}

// TierPreset is a user-created color/label preset that is shared with everyone.
type TierPreset struct {
	dbmodel.GormModel
	UserID      uint              `json:"userId" gorm:"index"`
	Name        string            `json:"name" gorm:"not null"`
	Description string            `json:"description"`
	Tiers       []TierPresetEntry `json:"tiers" gorm:"foreignKey:PresetID"`
}

type TierPresetEntry struct {
	dbmodel.GormModel
	PresetID  uint   `json:"presetId" gorm:"index;not null"`
	Name      string `json:"name" gorm:"not null"`
	Color     string `json:"color" gorm:"not null"`
	TextColor string `json:"textColor" gorm:"not null"`
	Position  int    `json:"position" gorm:"not null;default:0"`
}
