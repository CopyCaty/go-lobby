package repository

import (
	"context"
	"errors"
	"go-lobby/internal/model"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

const mysqlDuplicateEntryError uint16 = 1062

type RankRepository struct {
	db *sqlx.DB
}

func NewRankRepository(db *sqlx.DB) *RankRepository {
	return &RankRepository{
		db: db,
	}
}

func (r *RankRepository) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return r.db.BeginTxx(ctx, nil)
}

func (r *RankRepository) UpdatePlayerRank(
	ctx context.Context,
	tx *sqlx.Tx,
	mode string,
	userID int64,
	scoreDelta int,
	winCount int,
	loseCount int,
) (int64, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO gl_player_rank (mode, user_id, score, win_count, lose_count, match_count)
		VALUES (?, ?, 1000 + ?, ?, ?, 1)
		ON DUPLICATE KEY UPDATE
			score = score + ?,
			win_count = win_count + ?,
			lose_count = lose_count + ?,
			match_count = match_count + 1
	`, mode, userID, scoreDelta, winCount, loseCount, scoreDelta, winCount, loseCount)
	if err != nil {
		return 0, err
	}
	playerRankID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return playerRankID, nil
}

func (r *RankRepository) AddRankEvent(
	ctx context.Context,
	tx *sqlx.Tx,
	rankEvent *model.RankEvent,
) (int64, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO gl_rank_event (event_id, match_id, status)
		VALUES(?, ?, ?)
	`, rankEvent.EventID, rankEvent.MatchID, rankEvent.Status)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func IsDuplicateKeyError(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == mysqlDuplicateEntryError
	}
	return false
}
