package req

type UserRegisterRequest struct {
	UserName string `json:"user_name" binding:"required,min=6,max=20"`
	Nickname string `json:"nickname" binding:"required,min=6,max=20"`
	Password string `json:"password" binding:"required,min=8,max=20"`
}
