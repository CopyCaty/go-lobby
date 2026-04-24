package res

import "go-lobby/internal/matchqueue"

type JoinMatchQueueResponse struct {
	QueueStatus   string                   `json:"queue_status"`
	QueueTicketID string                   `json:"queue_icket_id,omitempty"`
	Mode          string                   `json:"mode"`
	RoomID        string                   `json:"room_id,omitempty"`
	Teams         []matchqueue.MatchedTeam `json:"teams,omitempty"`
}
