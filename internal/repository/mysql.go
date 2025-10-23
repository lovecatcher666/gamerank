package repository

import (
	"context"
	"database/sql"
	"fmt"

	"game-leaderboard/internal/model"

	"github.com/jmoiron/sqlx"
)

type MySQLRepository struct {
	db *sqlx.DB
}

func NewMySQLRepository(db *sqlx.DB) *MySQLRepository {
	return &MySQLRepository{
		db: db,
	}
}

// UpsertPlayer 插入或更新玩家信息
func (m *MySQLRepository) UpsertPlayer(ctx context.Context, player *model.Player) error {
	query := `
		INSERT INTO players (id, name, total_score, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			total_score = VALUES(total_score),
			updated_at = NOW()
	`

	_, err := m.db.ExecContext(ctx, query, player.ID, player.Name, player.TotalScore)
	if err != nil {
		return fmt.Errorf("failed to upsert player: %w", err)
	}

	return nil
}

// RecordScoreHistory 记录分数变更历史
func (m *MySQLRepository) RecordScoreHistory(ctx context.Context, history *model.PlayerScoreHistory) error {
	query := `
		INSERT INTO player_score_history (player_id, score_change, final_score, reason, created_at)
		VALUES (?, ?, ?, ?, NOW())
	`

	_, err := m.db.ExecContext(ctx, query, history.PlayerID, history.ScoreChange, history.FinalScore, history.Reason)
	if err != nil {
		return fmt.Errorf("failed to record score history: %w", err)
	}

	return nil
}

// GetPlayer 获取玩家信息
func (m *MySQLRepository) GetPlayer(ctx context.Context, playerID string) (*model.Player, error) {
	var player model.Player
	query := `SELECT id, name, total_score, created_at, updated_at FROM players WHERE id = ?`

	err := m.db.GetContext(ctx, &player, query, playerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPlayerNotFound
		}
		return nil, fmt.Errorf("failed to get player: %w", err)
	}

	return &player, nil
}

// GetTopPlayersFromDB 从数据库获取前N名玩家（用于数据恢复）
func (m *MySQLRepository) GetTopPlayersFromDB(ctx context.Context, limit int) ([]*model.Player, error) {
	var players []*model.Player
	query := `SELECT id, name, total_score, created_at, updated_at 
			  FROM players 
			  ORDER BY total_score DESC, updated_at ASC 
			  LIMIT ?`

	err := m.db.SelectContext(ctx, &players, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top players from db: %w", err)
	}

	return players, nil
}

// GetAllPlayers 获取所有玩家（用于数据恢复）
func (m *MySQLRepository) GetAllPlayers(ctx context.Context) ([]*model.Player, error) {
	var players []*model.Player
	query := `SELECT id, name, total_score, created_at, updated_at FROM players`

	err := m.db.SelectContext(ctx, &players, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all players: %w", err)
	}

	return players, nil
}

// SaveLeaderboardSnapshot 保存排行榜快照
func (m *MySQLRepository) SaveLeaderboardSnapshot(ctx context.Context, snapshotData []byte, playerCount int) error {
	query := `INSERT INTO leaderboard_snapshots (snapshot_data, player_count, created_at) VALUES (?, ?, NOW())`

	_, err := m.db.ExecContext(ctx, query, snapshotData, playerCount)
	if err != nil {
		return fmt.Errorf("failed to save leaderboard snapshot: %w", err)
	}

	return nil
}

// HealthCheck 健康检查
func (m *MySQLRepository) HealthCheck(ctx context.Context) error {
	return m.db.PingContext(ctx)
}

// Close 关闭连接
func (m *MySQLRepository) Close() error {
	return m.db.Close()
}
