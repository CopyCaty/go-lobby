package res

import (
	"go-lobby/internal/matchqueue"
	"time"
)

type StatusMatchQueueResponse struct {
	UserID    int64                    `json:"user_id"`
	MatchID   int64                    `json:"match_id,omitempty"`
	Mode      string                   `json:"mode"`
	Status    matchqueue.QueueStatus   `json:"status"`
	TicketID  string                   `json:"ticket_id"`
	RoomID    string                   `json:"room_id,omitempty"`
	Teams     []matchqueue.MatchedTeam `json:"teams,omitempty"`
	UpdatedAt time.Time                `json:"updated_at"`
}
