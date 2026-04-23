package res

type UserLoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
	UserID    int64  `json:"user_id"`
	UserName  string `json:"user_name"`
	Nickname  string `json:"nickname"`
}
