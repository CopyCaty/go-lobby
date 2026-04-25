package res

import (
	"go-lobby/internal/matchqueue"
	"go-lobby/internal/model"
)

type RoomInfoResponse struct {
	ID      string                      `json:"id"`
	MatchID int64                       `json:"match_id"`
	Mode    string                      `json:"mode"`
	Status  model.RoomStatus            `json:"status"`
	Teams   []matchqueue.MatchedTeam    `json:"teams"`
	Players map[int64]*model.RoomPlayer `json:"players"`
}
