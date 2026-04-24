package repository

import (
	"context"
	"database/sql"
	"errors"
	"go-lobby/internal/model"

	"github.com/jmoiron/sqlx"
)

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) FindByUserName(ctx context.Context, userName string) (*model.User, error) {
	var user model.User
	err := r.db.GetContext(ctx, &user, `
		SELECT id, user_name, nickname, password_hash, status, created_at, updated_at
		FROM gl_user
		WHERE user_name = ? AND STATUS != ?
	`, userName, model.UserStatusDeleted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, user *model.User) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO gl_user (user_name, nickname, password_hash, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, user.UserName, user.Nickname, user.PasswordHash, user.Status, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	var user model.User
	err := r.db.GetContext(ctx, &user, `
		SELECT id, user_name, nickname, password_hash, status, created_at, updated_at
		FROM gl_user
		WHERE id = ? AND STATUS != ?
	`, userID, model.UserStatusDeleted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}
