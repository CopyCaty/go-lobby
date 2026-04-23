package req

type UserLoginRequest struct {
	UserName string `json:"user_name" binding:"required,min=6,max=20"`
	Password string `json:"password" binding:"required,min=8,max=20"`
}
