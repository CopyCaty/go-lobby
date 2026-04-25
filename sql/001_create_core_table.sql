-- +goose Up

CREATE TABLE IF NOT EXISTS gl_user (
  id BIGINT NOT NULL AUTO_INCREMENT,
  user_name VARCHAR(20) NOT NULL,
  nickname VARCHAR(20) NOT NULL,
  password_hash VARCHAR(128) NOT NULL,
  status SMALLINT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_gl_user_user_name (user_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS gl_gamemode (
  id BIGINT NOT NULL AUTO_INCREMENT,
  code VARCHAR(50) NOT NULL,
  name VARCHAR(128) NOT NULL,
  team_size INT NOT NULL,
  rank_enabled SMALLINT NOT NULL,
  status SMALLINT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_gl_gamemode_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS gl_match (
  id BIGINT NOT NULL AUTO_INCREMENT,
  status SMALLINT NOT NULL,
  win_team_no SMALLINT DEFAULT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  started_at DATETIME DEFAULT NULL,
  finished_at DATETIME DEFAULT NULL,
  mode VARCHAR(20) DEFAULT NULL,
  room_id VARCHAR(255) DEFAULT NULL,
  PRIMARY KEY (id),
  KEY idx_gl_match_room_id (room_id),
  KEY idx_gl_match_mode_status (mode, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS gl_match_player (
  id BIGINT NOT NULL AUTO_INCREMENT,
  match_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  team_no SMALLINT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_gl_match_player_match_user (match_id, user_id),
  KEY idx_gl_match_player_user_id (user_id),
  CONSTRAINT fk_gl_match_player_match
    FOREIGN KEY (match_id) REFERENCES gl_match(id)
    ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down

DROP TABLE IF EXISTS gl_match_player;
DROP TABLE IF EXISTS gl_match;
DROP TABLE IF EXISTS gl_gamemode;
DROP TABLE IF EXISTS gl_user;
