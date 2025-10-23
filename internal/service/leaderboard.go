package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"game-leaderboard/internal/cache"
	"game-leaderboard/internal/model"
	"game-leaderboard/internal/repository"
	"game-leaderboard/pkg/logger"
)

// 定义服务级别的错误
var (
	ErrPlayerNotFound = fmt.Errorf("player not found")
	ErrInvalidRange   = fmt.Errorf("invalid range")
)

type LeaderboardService struct {
	redisRepo        *repository.RedisRepository
	mysqlRepo        *repository.MySQLRepository
	rankingMethod    string
	enableCache      bool
	cache            *cache.LocalCache
	mu               sync.RWMutex
	logger           *logger.Logger
	snapshotInterval time.Duration
	lastSnapshot     time.Time
}

func NewLeaderboardService(redisRepo *repository.RedisRepository, mysqlRepo *repository.MySQLRepository, rankingMethod string, enableCache bool) *LeaderboardService {
	service := &LeaderboardService{
		redisRepo:        redisRepo,
		mysqlRepo:        mysqlRepo,
		rankingMethod:    rankingMethod,
		enableCache:      enableCache,
		logger:           logger.NewLogger("leaderboard_service"),
		snapshotInterval: 1 * time.Hour, // 每小时快照一次
	}

	if enableCache {
		service.cache = cache.NewLocalCache(10000) // 缓存10000个结果
	}

	// 启动后台任务
	go service.backgroundTasks()

	return service
}

// UpdateScore 更新玩家分数
func (s *LeaderboardService) UpdateScore(ctx context.Context, playerID string, incrScore int64, name, reason string) error {
	// 1. 先更新 MySQL（作为数据源）
	currentPlayer, err := s.mysqlRepo.GetPlayer(ctx, playerID)
	if err != nil && err != repository.ErrPlayerNotFound {
		return fmt.Errorf("failed to get player from mysql: %w", err)
	}

	var finalScore int64
	if currentPlayer != nil {
		finalScore = currentPlayer.TotalScore + incrScore
	} else {
		finalScore = incrScore
	}

	// 更新 MySQL 玩家表
	player := &model.Player{
		ID:         playerID,
		Name:       name,
		TotalScore: finalScore,
	}

	if err := s.mysqlRepo.UpsertPlayer(ctx, player); err != nil {
		return fmt.Errorf("failed to update player in mysql: %w", err)
	}

	// 记录分数变更历史
	history := &model.PlayerScoreHistory{
		PlayerID:    playerID,
		ScoreChange: incrScore,
		FinalScore:  finalScore,
		Reason:      reason,
	}

	if err := s.mysqlRepo.RecordScoreHistory(ctx, history); err != nil {
		s.logger.Warn("Failed to record score history", "error", err)
	}

	// 2. 更新 Redis（作为排行榜存储）
	if err := s.redisRepo.UpdatePlayerScore(ctx, playerID, finalScore, name); err != nil {
		// Redis 更新失败，记录错误但不要完全失败
		s.logger.Error("Failed to update redis leaderboard",
			"playerID", playerID,
			"error", err)
		// 可以加入重试机制
	}

	// 3. 清除相关缓存
	if s.enableCache {
		s.cache.ClearPlayerRank(playerID)
		s.cache.ClearTopN()
	}

	s.logger.Info("Player score updated",
		"playerID", playerID,
		"scoreChange", incrScore,
		"finalScore", finalScore,
		"reason", reason)

	return nil
}

// GetPlayerRank 获取玩家排名
func (s *LeaderboardService) GetPlayerRank(ctx context.Context, playerID string) (*model.RankInfo, error) {
	// 尝试从缓存获取
	if s.enableCache {
		if cached, ok := s.cache.GetPlayerRank(playerID); ok {
			return cached, nil
		}
	}

	// 从 Redis 获取排名和分数
	rank, err := s.redisRepo.GetPlayerRank(ctx, playerID)
	if err != nil {
		if err == repository.ErrPlayerNotFound {
			return nil, ErrPlayerNotFound
		}
		return nil, err
	}

	score, err := s.redisRepo.GetPlayerScore(ctx, playerID)
	if err != nil {
		if err == repository.ErrPlayerNotFound {
			return nil, ErrPlayerNotFound
		}
		return nil, err
	}

	// 获取玩家名称
	player, err := s.mysqlRepo.GetPlayer(ctx, playerID)
	if err != nil {
		if err == repository.ErrPlayerNotFound {
			// 如果 MySQL 中没有，但 Redis 中有，创建一个基本的玩家信息
			player = &model.Player{
				ID:   playerID,
				Name: "",
			}
		} else {
			return nil, err
		}
	}

	rankInfo := &model.RankInfo{
		PlayerID:  playerID,
		Rank:      int(rank),
		Score:     int64(score),
		Name:      player.Name,
		UpdatedAt: player.UpdatedAt,
	}

	// 应用排名策略（密集排名）
	if s.rankingMethod == "dense" {
		rankInfo.Rank = s.calculateDenseRank(ctx, playerID, int64(score))
	}

	// 缓存结果
	if s.enableCache {
		s.cache.SetPlayerRank(playerID, rankInfo)
	}

	return rankInfo, nil
}

// GetTopN 获取前N名玩家
func (s *LeaderboardService) GetTopN(ctx context.Context, n int) ([]*model.RankInfo, error) {
	if n <= 0 {
		return nil, fmt.Errorf("invalid N: %d", n)
	}

	// 尝试从缓存获取
	if s.enableCache {
		if cached, ok := s.cache.GetTopN(n); ok {
			return cached, nil
		}
	}

	// 从 Redis 获取前N名
	rankings, err := s.redisRepo.GetTopPlayers(ctx, int64(n))
	if err != nil {
		return nil, err
	}

	// 应用密集排名策略
	if s.rankingMethod == "dense" {
		rankings = s.applyDenseRanking(rankings)
	}

	// 缓存结果
	if s.enableCache {
		s.cache.SetTopN(n, rankings)
	}

	return rankings, nil
}

// GetPlayerRankRange 获取玩家周边排名
func (s *LeaderboardService) GetPlayerRankRange(ctx context.Context, playerID string, rangeNum int) ([]*model.RankInfo, error) {
	if rangeNum <= 0 {
		return nil, fmt.Errorf("invalid range: %d", rangeNum)
	}

	rankings, err := s.redisRepo.GetPlayerRankRange(ctx, playerID, int64(rangeNum))
	if err != nil {
		if err == repository.ErrPlayerNotFound {
			return nil, ErrPlayerNotFound
		}
		return nil, err
	}

	// 应用密集排名策略
	if s.rankingMethod == "dense" {
		rankings = s.applyDenseRanking(rankings)
	}

	return rankings, nil
}

// 计算密集排名
func (s *LeaderboardService) calculateDenseRank(ctx context.Context, playerID string, score int64) int {
	// 获取排行榜大小
	size, err := s.redisRepo.GetLeaderboardSize(ctx)
	if err != nil {
		s.logger.Warn("Failed to get leaderboard size for dense ranking", "error", err)
		return 0
	}

	// 获取比当前玩家分数高的玩家数量
	// 注意：这只是一个近似值，实际实现可能需要更复杂的逻辑
	topPlayers, err := s.redisRepo.GetTopPlayers(ctx, size)
	if err != nil {
		s.logger.Warn("Failed to get top players for dense ranking", "error", err)
		return 0
	}

	// 计算唯一分数的数量
	uniqueScores := make(map[int64]bool)
	for _, player := range topPlayers {
		uniqueScores[player.Score] = true
	}

	// 计算比当前分数高的唯一分数数量
	higherCount := 0
	for uniqueScore := range uniqueScores {
		if uniqueScore > score {
			higherCount++
		}
	}

	return higherCount + 1
}

// 应用密集排名到结果集
func (s *LeaderboardService) applyDenseRanking(rankings []*model.RankInfo) []*model.RankInfo {
	if len(rankings) == 0 {
		return rankings
	}

	denseRank := 1
	lastScore := rankings[0].Score

	for i, rankInfo := range rankings {
		if rankInfo.Score != lastScore {
			denseRank++
			lastScore = rankInfo.Score
		}
		rankings[i].Rank = denseRank
	}

	return rankings
}

// 后台任务
func (s *LeaderboardService) backgroundTasks() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 定期创建快照
		if time.Since(s.lastSnapshot) > s.snapshotInterval {
			s.createSnapshot(context.Background())
		}

		// 健康检查
		s.healthCheck(context.Background())
	}
}

// 创建排行榜快照
func (s *LeaderboardService) createSnapshot(ctx context.Context) {
	players, err := s.mysqlRepo.GetAllPlayers(ctx)
	if err != nil {
		s.logger.Error("Failed to get players for snapshot", "error", err)
		return
	}

	snapshotData, err := json.Marshal(players)
	if err != nil {
		s.logger.Error("Failed to marshal snapshot data", "error", err)
		return
	}

	if err := s.mysqlRepo.SaveLeaderboardSnapshot(ctx, snapshotData, len(players)); err != nil {
		s.logger.Error("Failed to save leaderboard snapshot", "error", err)
		return
	}

	s.lastSnapshot = time.Now()
	s.logger.Info("Leaderboard snapshot created", "playerCount", len(players))
}

// 健康检查
func (s *LeaderboardService) healthCheck(ctx context.Context) {
	if err := s.redisRepo.HealthCheck(ctx); err != nil {
		s.logger.Error("Redis health check failed", "error", err)
	}

	if err := s.mysqlRepo.HealthCheck(ctx); err != nil {
		s.logger.Error("MySQL health check failed", "error", err)
	}
}

// CheckRedisHealth 检查 Redis 健康状态
func (s *LeaderboardService) CheckRedisHealth(ctx context.Context) bool {
	if err := s.redisRepo.HealthCheck(ctx); err != nil {
		s.logger.Error("Redis health check failed", "error", err)
		return false
	}
	return true
}

// CheckMySQLHealth 检查 MySQL 健康状态
func (s *LeaderboardService) CheckMySQLHealth(ctx context.Context) bool {
	if err := s.mysqlRepo.HealthCheck(ctx); err != nil {
		s.logger.Error("MySQL health check failed", "error", err)
		return false
	}
	return true
}

// GetCacheStats 获取缓存统计
func (s *LeaderboardService) GetCacheStats() map[string]interface{} {
	if s.cache != nil {
		return s.cache.GetStats()
	}
	return map[string]interface{}{
		"enabled": false,
	}
}

// RebuildLeaderboard 从 MySQL 重建 Redis 排行榜（用于数据恢复）
func (s *LeaderboardService) RebuildLeaderboard(ctx context.Context) error {
	s.logger.Info("Starting leaderboard rebuild from MySQL")

	players, err := s.mysqlRepo.GetAllPlayers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get players from mysql: %w", err)
	}

	// 批量更新 Redis
	for _, player := range players {
		if err := s.redisRepo.UpdatePlayerScore(ctx, player.ID, player.TotalScore, player.Name); err != nil {
			s.logger.Warn("Failed to update player in redis during rebuild",
				"playerID", player.ID,
				"error", err)
		}
	}

	s.logger.Info("Leaderboard rebuild completed", "playerCount", len(players))
	return nil
}
