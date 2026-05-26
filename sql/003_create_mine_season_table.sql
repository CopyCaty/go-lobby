-- +goose Up

CREATE TABLE IF NOT EXISTS gl_mine_season (
  id BIGINT NOT NULL AUTO_INCREMENT,
  code VARCHAR(50) NOT NULL,
  seed VARCHAR(128) NOT NULL,
  mine_rate DECIMAL(6,5) NOT NULL,
  algorithm_version INT NOT NULL,
  status SMALLINT NOT NULL,
  started_at DATETIME NOT NULL,
  ended_at DATETIME DEFAULT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_gl_mine_season_code (code),
  KEY idx_gl_mine_season_status_time (status, started_at, ended_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO gl_mine_season (code, seed, mine_rate, algorithm_version, status, started_at)
VALUES ('s1-cn-mvp', 'go-lobby-mine-season-s1-local-dev-seed', 0.15000, 1, 1, '2026-01-01 00:00:00')
ON DUPLICATE KEY UPDATE code = code;

-- +goose Down

DROP TABLE IF EXISTS gl_mine_season;
