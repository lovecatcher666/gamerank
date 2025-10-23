package repository

import (
	"context"
	"fmt"
	"time"

	"game-leaderboard/internal/model"
	"game-leaderboard/pkg/logger"

	"github.com/go-redis/redis/v8"
)

const (
	// Redis Key 定义
	LeaderboardKey     = "leaderboard:global"
	PlayerKeyPrefix    = "player:"
	PlayerCacheKey     = "player_cache"
	TopPlayersCacheKey = "top_players_cache"
)

type RedisRepository struct {
	client *redis.Client
	logger *logger.Logger
}

func NewRedisRepository(client *redis.Client) *RedisRepository {
	return &RedisRepository{
		client: client,
		logger: logger.NewLogger("redis_repository"),
	}
}

// UpdatePlayerScore 更新玩家分数（Redis Sorted Set）
func (r *RedisRepository) UpdatePlayerScore(ctx context.Context, playerID string, score int64, name string) error {
	// 使用 Sorted Set 存储排行榜，score 作为分数，playerID 作为成员
	_, err := r.client.ZAdd(ctx, LeaderboardKey, &redis.Z{
		Score:  float64(score),
		Member: playerID,
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to update player score in redis: %w", err)
	}

	// 存储玩家详细信息
	playerInfo := map[string]interface{}{
		"name":       name,
		"updated_at": time.Now().Unix(),
	}

	_, err = r.client.HSet(ctx, PlayerKeyPrefix+playerID, playerInfo).Result()
	if err != nil {
		return fmt.Errorf("failed to update player info in redis: %w", err)
	}

	// 设置过期时间（可选，防止数据无限增长）
	r.client.Expire(ctx, PlayerKeyPrefix+playerID, 7*24*time.Hour)

	r.logger.Debug("Updated player score in redis",
		"playerID", playerID,
		"score", score,
		"name", name)

	return nil
}

// GetPlayerRank 获取玩家排名
func (r *RedisRepository) GetPlayerRank(ctx context.Context, playerID string) (int64, error) {
	// ZREVRANK 返回从高到低的排名（0-based）
	rank, err := r.client.ZRevRank(ctx, LeaderboardKey, playerID).Result()
	if err != nil {
		if err == redis.Nil {
			return -1, ErrPlayerNotFound
		}
		return -1, fmt.Errorf("failed to get player rank: %w", err)
	}

	// 转换为 1-based 排名
	return rank + 1, nil
}

// GetPlayerScore 获取玩家分数
func (r *RedisRepository) GetPlayerScore(ctx context.Context, playerID string) (float64, error) {
	score, err := r.client.ZScore(ctx, LeaderboardKey, playerID).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, ErrPlayerNotFound
		}
		return 0, fmt.Errorf("failed to get player score: %w", err)
	}
	return score, nil
}

// GetTopPlayers 获取前N名玩家
func (r *RedisRepository) GetTopPlayers(ctx context.Context, n int64) ([]*model.RankInfo, error) {
	// ZREVRANGE 获取前N名（从高到低）
	result, err := r.client.ZRevRangeWithScores(ctx, LeaderboardKey, 0, n-1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get top players: %w", err)
	}

	rankings := make([]*model.RankInfo, 0, len(result))

	for i, z := range result {
		playerID := z.Member.(string)

		// 获取玩家详细信息
		name, err := r.getPlayerName(ctx, playerID)
		if err != nil {
			r.logger.Warn("Failed to get player name", "playerID", playerID, "error", err)
			name = ""
		}

		rankings = append(rankings, &model.RankInfo{
			PlayerID: playerID,
			Rank:     i + 1,
			Score:    int64(z.Score),
			Name:     name,
		})
	}

	return rankings, nil
}

// GetPlayerRankRange 获取玩家排名范围
func (r *RedisRepository) GetPlayerRankRange(ctx context.Context, playerID string, rangeNum int64) ([]*model.RankInfo, error) {
	// 先获取玩家排名
	rank, err := r.GetPlayerRank(ctx, playerID)
	if err != nil {
		return nil, err
	}

	// 计算范围（rank 是 1-based）
	start := rank - rangeNum/2 - 1
	if start < 0 {
		start = 0
	}
	end := start + rangeNum

	// 获取范围内的玩家
	result, err := r.client.ZRevRangeWithScores(ctx, LeaderboardKey, start, end).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get player rank range: %w", err)
	}

	rankings := make([]*model.RankInfo, 0, len(result))

	for i, z := range result {
		currentPlayerID := z.Member.(string)
		name, _ := r.getPlayerName(ctx, currentPlayerID)

		rankings = append(rankings, &model.RankInfo{
			PlayerID: currentPlayerID,
			Rank:     int(start) + i + 1,
			Score:    int64(z.Score),
			Name:     name,
		})
	}

	return rankings, nil
}

// GetLeaderboardSize 获取排行榜大小
func (r *RedisRepository) GetLeaderboardSize(ctx context.Context) (int64, error) {
	return r.client.ZCard(ctx, LeaderboardKey).Result()
}

// 获取玩家名称
func (r *RedisRepository) getPlayerName(ctx context.Context, playerID string) (string, error) {
	name, err := r.client.HGet(ctx, PlayerKeyPrefix+playerID, "name").Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}
	return name, nil
}

// HealthCheck 健康检查
func (r *RedisRepository) HealthCheck(ctx context.Context) error {
	_, err := r.client.Ping(ctx).Result()
	return err
}

// Close 关闭连接
func (r *RedisRepository) Close() error {
	return r.client.Close()
}
