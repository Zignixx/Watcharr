package gamescore

import (
	"log/slog"

	"github.com/sbondCo/Watcharr/database/entity"
	"github.com/sbondCo/Watcharr/domain"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db}
}

func (s *Service) SaveScore(userId uint, req domain.GameScoreSaveRequest) error {
	gs := entity.GameScore{
		UserID:     userId,
		Game:       req.Game,
		Score:      req.Score,
		BestStreak: req.BestStreak,
	}
	if res := s.db.Create(&gs); res.Error != nil {
		slog.Error("SaveScore: failed to save game score", "error", res.Error.Error())
		return res.Error
	}
	return nil
}

func (s *Service) GetScores(userId uint) ([]domain.GameScoreResponse, error) {
	type result struct {
		Game        string
		HighScore   int
		BestStreak  int
		TimesPlayed int
	}
	var results []result
	res := s.db.Model(&entity.GameScore{}).
		Select("game, MAX(score) as high_score, MAX(best_streak) as best_streak, COUNT(*) as times_played").
		Where("user_id = ?", userId).
		Group("game").
		Find(&results)
	if res.Error != nil {
		slog.Error("GetScores: failed to get game scores", "error", res.Error.Error())
		return nil, res.Error
	}
	scores := make([]domain.GameScoreResponse, len(results))
	for i, r := range results {
		scores[i] = domain.GameScoreResponse{
			Game:        r.Game,
			HighScore:   r.HighScore,
			BestStreak:  r.BestStreak,
			TimesPlayed: r.TimesPlayed,
		}
	}
	return scores, nil
}
