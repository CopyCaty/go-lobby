package matchqueue

import "time"

type QueueStatus string

const (
	QueueStatusInit      QueueStatus = "init"
	QueueStatusMatching  QueueStatus = "matching"
	QueueStatusMatched   QueueStatus = "matched"
	QueueStatusCancelled QueueStatus = "cancelled"
)

type MatchedTeam struct {
	TeamID  int8    `json:"team_id"`
	UserIDs []int64 `json:"user_ids"`
}

type MatchQueueResult struct {
	RoomID string        `json:"room_id"`
	Mode   string        `json:"mode"`
	Teams  []MatchedTeam `json:"teams"`
}

type QueueEntry struct {
	UserID      int64
	UserName    string
	Mode        string
	EnqueueTime time.Time
	TicketID    string
}

type QueueUserState struct {
	UserID      int64
	UserName    string
	Mode        string
	Status      QueueStatus
	MatchID     int64
	TicketID    string
	RoomID      string
	Teams       []MatchedTeam
	EnqueueTime time.Time
	UpdatedAt   time.Time
}
