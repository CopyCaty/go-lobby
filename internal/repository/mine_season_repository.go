package repository

import (
	"context"
	"database/sql"
	"go-lobby/internal/model"

	"github.com/jmoiron/sqlx"
)

type MineSeasonRepository struct {
	db *sqlx.DB
}

func NewMineSeasonRepository(db *sqlx.DB) *MineSeasonRepository {
	return &MineSeasonRepository{db: db}
}

func (r *MineSeasonRepository) GetActiveSeason(ctx context.Context) (*model.MineSeason, error) {
	var season model.MineSeason
	err := r.db.GetContext(ctx, &season, `
		SELECT id, code, seed, mine_rate, algorithm_version, status, started_at, ended_at, created_at, updated_at
		FROM gl_mine_season
		WHERE status = ?
		  AND started_at <= NOW()
		  AND (ended_at IS NULL OR ended_at > NOW())
		ORDER BY started_at DESC, id DESC
		LIMIT 1
	`, model.MineSeasonStatusActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &season, nil
}
