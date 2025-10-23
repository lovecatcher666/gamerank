-- 玩家基础信息表
CREATE TABLE IF NOT EXISTS players (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL DEFAULT '',
    total_score BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_total_score (total_score DESC),
    INDEX idx_updated_at (updated_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 玩家分数历史记录表
CREATE TABLE IF NOT EXISTS player_score_history (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    player_id VARCHAR(64) NOT NULL,
    score_change BIGINT NOT NULL,
    final_score BIGINT NOT NULL,
    reason VARCHAR(255) DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_player_id (player_id),
    INDEX idx_created_at (created_at DESC),
    FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 排行榜快照表（用于数据恢复和审计）
CREATE TABLE IF NOT EXISTS leaderboard_snapshots (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    snapshot_data JSON NOT NULL,
    player_count INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_created_at (created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;