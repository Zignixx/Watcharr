package entity

import "github.com/sbondCo/Watcharr/database/dbmodel"

type GameScore struct {
	dbmodel.GormModel
	UserID   uint   `gorm:"not null;index:idx_gamescore_user_game" json:"-"`
	Game     string `gorm:"not null;index:idx_gamescore_user_game" json:"game"`
	Score    int    `gorm:"not null" json:"score"`
	BestStreak int  `json:"bestStreak"`
}
