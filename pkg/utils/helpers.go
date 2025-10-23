package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"
	"unicode/utf8"
)

// GeneratePlayerID 生成玩家ID
func GeneratePlayerID(prefix string) string {
	timestamp := time.Now().UnixNano()
	random := rand.Intn(10000)

	data := fmt.Sprintf("%s%d%d", prefix, timestamp, random)
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:8]) // 取前8位
}

// ValidatePlayerID 验证玩家ID格式
func ValidatePlayerID(playerID string) bool {
	if playerID == "" {
		return false
	}

	// 简单的长度和字符验证
	if utf8.RuneCountInString(playerID) > 64 {
		return false
	}

	// 可以添加更多的验证逻辑
	return true
}

// CalculateDenseRank 计算密集排名
func CalculateDenseRank(scores []int64, currentScore int64) int {
	if len(scores) == 0 {
		return 1
	}

	// 去重并排序
	uniqueScores := make(map[int64]bool)
	for _, score := range scores {
		uniqueScores[score] = true
	}

	// 计算比当前分数高的唯一分数数量
	higherCount := 0
	for score := range uniqueScores {
		if score > currentScore {
			higherCount++
		}
	}

	return higherCount + 1
}

// SafeString 安全的字符串处理，防止空指针
func SafeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ParseDurationSafe 安全解析时间间隔
func ParseDurationSafe(durationStr string, defaultDuration time.Duration) time.Duration {
	if duration, err := time.ParseDuration(durationStr); err == nil {
		return duration
	}
	return defaultDuration
}

// ContainsString 检查字符串切片是否包含指定字符串
func ContainsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// Min 返回最小值
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max 返回最大值
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Clamp 限制值在指定范围内
func Clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
