package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"game-leaderboard/internal/config"
	"game-leaderboard/internal/handler"
	"game-leaderboard/internal/repository"
	"game-leaderboard/internal/service"
	"game-leaderboard/pkg/database"
	"game-leaderboard/pkg/logger"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg := config.LoadConfig()

	fmt.Println("cfg:", cfg)

	// 初始化数据库连接
	mysqlDB, err := database.NewMySQLConnection(cfg.MySQLDSN, cfg.MySQLMaxConns)
	if err != nil {
		log.Fatal("Failed to connect to MySQL:", err)
	}
	defer mysqlDB.Close()

	redisClient, err := database.NewRedisConnection(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
	defer redisClient.Close()

	// 初始化存储
	redisRepo := repository.NewRedisRepository(redisClient)
	mysqlRepo := repository.NewMySQLRepository(mysqlDB)

	// 初始化服务
	leaderboardService := service.NewLeaderboardService(
		redisRepo,
		mysqlRepo,
		cfg.RankingMethod,
		cfg.EnableCache,
	)

	// 启动时重建排行榜（确保数据一致性）
	if cfg.RebuildOnStart {
		ctx := context.Background()
		if err := leaderboardService.RebuildLeaderboard(ctx); err != nil {
			logger.NewLogger("main").Error("Failed to rebuild leaderboard", "error", err)
		}
	}

	// 初始化处理器
	httpHandler := handler.NewHTTPHandler(leaderboardService)

	// 设置 Gin
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// 中间件
	router.Use(gin.Recovery())
	router.Use(CORSMiddleware())

	// API 路由
	api := router.Group("/game/rank")
	{
		api.POST("/upscores", httpHandler.UpdateScore)
		api.GET("/user/:playerId", httpHandler.GetPlayerRank)
		api.GET("/top/:n", httpHandler.GetTopN)
		api.GET("/range/:playerId/:range", httpHandler.GetPlayerRankRange)
		api.GET("/health", httpHandler.HealthCheck)
		api.POST("/rebuild", httpHandler.RebuildLeaderboard)
		api.GET("/cache_stats", httpHandler.GetCacheStats)
	}

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	// 在 goroutine 中启动服务器
	go func() {
		log.Printf("Server starting on :%s", cfg.Port)
		log.Printf("Environment: %s", cfg.Environment)
		log.Printf("Ranking method: %s", cfg.RankingMethod)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// 给服务器 5 秒时间完成当前请求
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
