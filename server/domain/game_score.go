package domain

type GameScoreSaveRequest struct {
	Game       string `json:"game" binding:"required,oneof=trivia highlow poster emoji timeline rating oddone"`
	Score      int    `json:"score" binding:"min=0"`
	BestStreak int    `json:"bestStreak" binding:"min=0"`
}

type GameScoreResponse struct {
	Game        string `json:"game"`
	HighScore   int    `json:"highScore"`
	BestStreak  int    `json:"bestStreak"`
	TimesPlayed int    `json:"timesPlayed"`
}
