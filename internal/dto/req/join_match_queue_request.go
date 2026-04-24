package req

type JoinMatchQueueRequest struct {
	Mode string `json:"mode" binding:"required"`
}
