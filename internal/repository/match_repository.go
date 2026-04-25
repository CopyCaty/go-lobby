package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go-lobby/internal/model"

	"github.com/jmoiron/sqlx"
)

type MatchRepository struct {
	db *sqlx.DB
}

func NewMatchRepository(db *sqlx.DB) *MatchRepository {
	return &MatchRepository{db: db}
}

func (r *MatchRepository) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return r.db.BeginTxx(ctx, nil)
}

func (r *MatchRepository) CreateMatch(ctx context.Context, tx *sqlx.Tx, match *model.Match) (int64, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO gl_match (status, started_at, mode, room_id)
		VALUES (?, ?, ?, ?)
	`, match.Status, match.StartedAt, match.Mode, match.RoomID)
	if err != nil {
		return 0, fmt.Errorf("err:%w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("err:%w", err)
	}
	return id, nil
}

func (r *MatchRepository) GetMatchByID(ctx context.Context, matchID int64) (*model.Match, error) {
	var match model.Match
	err := r.db.GetContext(ctx, &match, `
		SELECT id, room_id, status, win_team_no, started_at, finished_at, mode
		FROM gl_match
		WHERE id = ?
	`, matchID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &match, nil
}

func (r *MatchRepository) UpdateMatch(ctx context.Context, match *model.Match) (*model.Match, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE gl_match
		SET status = ?, win_team_no = ?, started_at = ?, finished_at = ?, room_id = ?
		WHERE id = ?
	`, match.Status, match.WinTeamNo, match.StartedAt, match.FinishedAt, match.RoomID, match.ID)
	if err != nil {
		return nil, fmt.Errorf("err:%w", err)
	}
	updatedMatch, err := r.GetMatchByID(ctx, int64(match.ID))
	if err != nil {
		return nil, fmt.Errorf("err:%w", err)
	}
	return updatedMatch, nil
}

func (r *MatchRepository) AddMatchPlayer(ctx context.Context, tx *sqlx.Tx, matchPlayers []*model.MatchPlayer) error {
	for _, mp := range matchPlayers {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO gl_match_player (match_id, user_id, team_no)
			VALUES (?, ?, ?)
		`, mp.MatchID, mp.UserID, mp.TeamNo)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *MatchRepository) GetMatchPlayers(ctx context.Context, matchID int64) ([]*model.MatchPlayer, error) {
	var players []*model.MatchPlayer
	err := r.db.SelectContext(ctx, &players, `
		SELECT match_id, user_id, team_no
		FROM gl_match_player
		WHERE match_id = ?
	`, matchID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return players, nil
}
