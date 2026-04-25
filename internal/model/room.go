package model

import (
	"go-lobby/internal/matchqueue"
	"time"
)

type RoomStatus string

const (
	RoomStatusWaiting  RoomStatus = "waiting"
	RoomStatusPlaying  RoomStatus = "playing"
	RoomStatusFinished RoomStatus = "finished"
)

type Room struct {
	ID      string
	MatchID int64
	Mode    string
	Status  RoomStatus
	Teams   []matchqueue.MatchedTeam
	Players map[int64]*RoomPlayer
}

type RoomPlayer struct {
	UserID    int64     `json:"user_id"`
	UserName  string    `json:"user_name"`
	TeamNo    int8      `json:"team_no"`
	Online    bool      `json:"online"`
	Ready     bool      `json:"ready"`
	UpdatedAt time.Time `json:"updated_at"`
}
