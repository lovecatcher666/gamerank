package model

import (
	"time"
)

// Player 玩家信息
type Player struct {
	ID         string    `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	TotalScore int64     `json:"total_score" db:"total_score"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// PlayerScoreHistory 玩家分数历史
type PlayerScoreHistory struct {
	ID          int64     `json:"id" db:"id"`
	PlayerID    string    `json:"player_id" db:"player_id"`
	ScoreChange int64     `json:"score_change" db:"score_change"`
	FinalScore  int64     `json:"final_score" db:"final_score"`
	Reason      string    `json:"reason" db:"reason"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// RankInfo 排名信息
type RankInfo struct {
	PlayerID  string    `json:"playerId"`
	Rank      int       `json:"rank"`
	Score     int64     `json:"score"`
	Name      string    `json:"name,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

// LeaderboardConfig 排行榜配置
type LeaderboardConfig struct {
	Name          string `json:"name"`
	MaxPlayers    int    `json:"maxPlayers"`
	EnableCache   bool   `json:"enableCache"`
	CacheSize     int    `json:"cacheSize"`
	RankingMethod string `json:"rankingMethod"` // "standard" or "dense"
	RedisKey      string `json:"redisKey"`
}

// UpdateRequest 分数更新请求
type UpdateRequest struct {
	PlayerID  string `json:"playerId" binding:"required"`
	IncrScore int64  `json:"incrScore" binding:"required"`
	Name      string `json:"name,omitempty"`
	Reason    string `json:"reason,omitempty"`
}
