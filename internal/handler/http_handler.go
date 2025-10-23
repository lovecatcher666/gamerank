package handler

import (
	"net/http"
	"strconv"
	"time"

	"game-leaderboard/internal/model"
	"game-leaderboard/internal/service"
	"game-leaderboard/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// 定义指标
var (
	requestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "endpoint", "status"})

	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "endpoint"})

	leaderboardUpdates = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "leaderboard_updates_total",
		Help: "Total number of leaderboard updates",
	}, []string{"player_id"})
)

type HTTPHandler struct {
	leaderboardService *service.LeaderboardService
	logger             *logger.Logger
}

func NewHTTPHandler(leaderboardService *service.LeaderboardService) *HTTPHandler {
	return &HTTPHandler{
		leaderboardService: leaderboardService,
		logger:             logger.NewLogger("http_handler"),
	}
}

// UpdateScore 更新玩家分数
// @Summary 更新玩家分数
// @Description 更新指定玩家的分数，如果玩家不存在则创建
// @Tags scores
// @Accept json
// @Produce json
// @Param request body model.UpdateRequest true "分数更新请求"
// @Success 200 {object} SuccessResponse "更新成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /scores [post]
func (h *HTTPHandler) UpdateScore(c *gin.Context) {
	start := time.Now()

	var req model.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.recordMetrics(c, "POST", "/scores", "400", start)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	if req.PlayerID == "" {
		h.recordMetrics(c, "POST", "/scores", "400", start)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "PlayerID is required",
			Message: "PlayerID cannot be empty",
		})
		return
	}

	if req.IncrScore == 0 {
		h.recordMetrics(c, "POST", "/scores", "400", start)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid score",
			Message: "Score increment cannot be zero",
		})
		return
	}

	ctx := c.Request.Context()
	err := h.leaderboardService.UpdateScore(ctx, req.PlayerID, req.IncrScore, req.Name, req.Reason)
	if err != nil {
		h.recordMetrics(c, "POST", "/scores", "500", start)
		h.logger.Error("Failed to update score",
			"playerID", req.PlayerID,
			"score", req.IncrScore,
			"error", err)

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update score",
			Message: err.Error(),
		})
		return
	}

	// 记录指标
	leaderboardUpdates.WithLabelValues(req.PlayerID).Inc()
	h.recordMetrics(c, "POST", "/scores", "200", start)

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Score updated successfully",
		Data: map[string]interface{}{
			"playerId":    req.PlayerID,
			"scoreChange": req.IncrScore,
			"timestamp":   time.Now(),
		},
	})
}

// GetPlayerRank 获取玩家排名
// @Summary 获取玩家排名
// @Description 获取指定玩家的当前排名信息
// @Tags ranks
// @Produce json
// @Param playerId path string true "玩家ID"
// @Success 200 {object} model.RankInfo "排名信息"
// @Failure 404 {object} ErrorResponse "玩家未找到"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /rank/{playerId} [get]
func (h *HTTPHandler) GetPlayerRank(c *gin.Context) {
	start := time.Now()
	playerID := c.Param("playerId")

	if playerID == "" {
		h.recordMetrics(c, "GET", "/rank/:playerId", "400", start)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "PlayerID is required",
			Message: "PlayerID parameter cannot be empty",
		})
		return
	}

	ctx := c.Request.Context()
	rankInfo, err := h.leaderboardService.GetPlayerRank(ctx, playerID)
	if err != nil {
		if err == service.ErrPlayerNotFound {
			h.recordMetrics(c, "GET", "/rank/:playerId", "404", start)
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Player not found",
				Message: "The specified player does not exist in the leaderboard",
			})
			return
		}

		h.recordMetrics(c, "GET", "/rank/:playerId", "500", start)
		h.logger.Error("Failed to get player rank",
			"playerID", playerID,
			"error", err)

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get player rank",
			Message: err.Error(),
		})
		return
	}

	h.recordMetrics(c, "GET", "/rank/:playerId", "200", start)
	c.JSON(http.StatusOK, rankInfo)
}

// GetTopN 获取前N名玩家
// @Summary 获取前N名玩家
// @Description 获取排行榜前N名玩家的排名信息
// @Tags ranks
// @Produce json
// @Param n path int true "前N名"
// @Success 200 {object} TopNResponse "前N名玩家列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /top/{n} [get]
func (h *HTTPHandler) GetTopN(c *gin.Context) {
	start := time.Now()
	nStr := c.Param("n")

	n, err := strconv.Atoi(nStr)
	if err != nil || n <= 0 {
		h.recordMetrics(c, "GET", "/top/:n", "400", start)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid N parameter",
			Message: "N must be a positive integer",
		})
		return
	}

	// 限制最大查询数量
	if n > 1000 {
		n = 1000
	}

	ctx := c.Request.Context()
	rankings, err := h.leaderboardService.GetTopN(ctx, n)
	if err != nil {
		h.recordMetrics(c, "GET", "/top/:n", "500", start)
		h.logger.Error("Failed to get top N players",
			"n", n,
			"error", err)

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get top players",
			Message: err.Error(),
		})
		return
	}

	h.recordMetrics(c, "GET", "/top/:n", "200", start)
	c.JSON(http.StatusOK, TopNResponse{
		Count:    len(rankings),
		Rankings: rankings,
	})
}

// GetPlayerRankRange 获取玩家周边排名
// @Summary 获取玩家周边排名
// @Description 获取指定玩家前后一定范围内的玩家排名信息
// @Tags ranks
// @Produce json
// @Param playerId path string true "玩家ID"
// @Param range path int true "范围大小"
// @Success 200 {object} RankRangeResponse "周边排名信息"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "玩家未找到"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /rank-range/{playerId}/{range} [get]
func (h *HTTPHandler) GetPlayerRankRange(c *gin.Context) {
	start := time.Now()
	playerID := c.Param("playerId")
	rangeStr := c.Param("range")

	if playerID == "" {
		h.recordMetrics(c, "GET", "/rank-range/:playerId/:range", "400", start)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "PlayerID is required",
			Message: "PlayerID parameter cannot be empty",
		})
		return
	}

	rangeNum, err := strconv.Atoi(rangeStr)
	if err != nil || rangeNum <= 0 {
		h.recordMetrics(c, "GET", "/rank-range/:playerId/:range", "400", start)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid range parameter",
			Message: "Range must be a positive integer",
		})
		return
	}

	// 限制最大范围
	if rangeNum > 100 {
		rangeNum = 100
	}

	ctx := c.Request.Context()
	rankings, err := h.leaderboardService.GetPlayerRankRange(ctx, playerID, rangeNum)
	if err != nil {
		if err == service.ErrPlayerNotFound {
			h.recordMetrics(c, "GET", "/rank-range/:playerId/:range", "404", start)
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "Player not found",
				Message: "The specified player does not exist in the leaderboard",
			})
			return
		}

		h.recordMetrics(c, "GET", "/rank-range/:playerId/:range", "500", start)
		h.logger.Error("Failed to get player rank range",
			"playerID", playerID,
			"range", rangeNum,
			"error", err)

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to get player rank range",
			Message: err.Error(),
		})
		return
	}

	h.recordMetrics(c, "GET", "/rank-range/:playerId/:range", "200", start)
	c.JSON(http.StatusOK, RankRangeResponse{
		PlayerID: playerID,
		Range:    rangeNum,
		Rankings: rankings,
	})
}

// HealthCheck 健康检查
// @Summary 健康检查
// @Description 检查服务健康状况
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse "健康状态"
// @Router /health [get]
func (h *HTTPHandler) HealthCheck(c *gin.Context) {
	start := time.Now()

	// 检查依赖服务状态
	ctx := c.Request.Context()
	redisHealthy := h.leaderboardService.CheckRedisHealth(ctx)
	mysqlHealthy := h.leaderboardService.CheckMySQLHealth(ctx)

	status := "healthy"
	if !redisHealthy || !mysqlHealthy {
		status = "degraded"
	}

	h.recordMetrics(c, "GET", "/health", "200", start)
	c.JSON(http.StatusOK, HealthResponse{
		Status:    status,
		Timestamp: time.Now(),
		Services: map[string]string{
			"redis": map[bool]string{true: "healthy", false: "unhealthy"}[redisHealthy],
			"mysql": map[bool]string{true: "healthy", false: "unhealthy"}[mysqlHealthy],
		},
	})
}

// RebuildLeaderboard 重建排行榜
// @Summary 重建排行榜
// @Description 从MySQL数据重建Redis排行榜（用于数据恢复）
// @Tags admin
// @Produce json
// @Success 200 {object} SuccessResponse "重建成功"
// @Failure 500 {object} ErrorResponse "重建失败"
// @Router /rebuild [post]
func (h *HTTPHandler) RebuildLeaderboard(c *gin.Context) {
	start := time.Now()

	ctx := c.Request.Context()
	err := h.leaderboardService.RebuildLeaderboard(ctx)
	if err != nil {
		h.recordMetrics(c, "POST", "/rebuild", "500", start)
		h.logger.Error("Failed to rebuild leaderboard", "error", err)

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to rebuild leaderboard",
			Message: err.Error(),
		})
		return
	}

	h.recordMetrics(c, "POST", "/rebuild", "200", start)
	c.JSON(http.StatusOK, SuccessResponse{
		Message:   "Leaderboard rebuilt successfully",
		Timestamp: time.Now(),
	})
}

// GetCacheStats 获取缓存统计
// @Summary 获取缓存统计
// @Description 获取本地缓存的统计信息
// @Tags admin
// @Produce json
// @Success 200 {object} CacheStatsResponse "缓存统计"
// @Router /cache/stats [get]
func (h *HTTPHandler) GetCacheStats(c *gin.Context) {
	start := time.Now()

	stats := h.leaderboardService.GetCacheStats()

	h.recordMetrics(c, "GET", "/cache/stats", "200", start)
	c.JSON(http.StatusOK, CacheStatsResponse{
		Stats: stats,
	})
}

// 记录指标
func (h *HTTPHandler) recordMetrics(c *gin.Context, method, endpoint, status string, start time.Time) {
	duration := time.Since(start).Seconds()

	requestCounter.WithLabelValues(method, endpoint, status).Inc()
	requestDuration.WithLabelValues(method, endpoint).Observe(duration)
}

// 响应结构体
type SuccessResponse struct {
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

type TopNResponse struct {
	Count    int               `json:"count"`
	Rankings []*model.RankInfo `json:"rankings"`
}

type RankRangeResponse struct {
	PlayerID string            `json:"playerId"`
	Range    int               `json:"range"`
	Rankings []*model.RankInfo `json:"rankings"`
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

type CacheStatsResponse struct {
	Stats map[string]interface{} `json:"stats"`
}
