-- +goose Up

CREATE TABLE IF NOT EXISTS gl_rank_event (
  id BIGINT NOT NULL AUTO_INCREMENT,
  event_id VARCHAR(128) NOT NULL,
  match_id BIGINT NOT NULL,
  status SMALLINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_gl_rank_event_event_id (event_id),
  UNIQUE KEY uk_gl_rank_event_match_id (match_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS gl_player_rank (
  id BIGINT NOT NULL AUTO_INCREMENT,
  mode VARCHAR(20) NOT NULL,
  user_id BIGINT NOT NULL,
  score INT NOT NULL DEFAULT 1000,
  win_count INT NOT NULL DEFAULT 0,
  lose_count INT NOT NULL DEFAULT 0,
  match_count INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_gl_player_rank_season_mode_user (mode, user_id),
  KEY idx_gl_player_rank_season_mode_score (mode, score)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS gl_player_rank;
DROP TABLE IF EXISTS gl_rank_event;
