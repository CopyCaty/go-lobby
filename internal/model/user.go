package model

import "time"

type User struct {
	ID           int64      `db:"id" json:"id"`
	UserName     string     `db:"user_name" json:"user_name"`
	Nickname     string     `db:"nickname" json:"nickname"`
	PasswordHash string     `db:"password_hash" json:"-"`
	Status       UserStatus `db:"status" json:"status"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

type UserStatus int16

const (
	UserStatusNormal  UserStatus = 1
	UserStatusBanned  UserStatus = 2
	UserStatusDeleted UserStatus = 3
)
