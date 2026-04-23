package service

import (
	"context"
	"errors"
	"go-lobby/internal/auth"
	"go-lobby/internal/dto/req"
	"go-lobby/internal/dto/res"
	"go-lobby/internal/model"
	"go-lobby/internal/repository"
	"time"
)

type UserService struct {
	repo       *repository.UserRepository
	jwtManager *auth.JWTManager
}

func NewUserService(repo *repository.UserRepository, jwtManager *auth.JWTManager) *UserService {
	return &UserService{
		repo:       repo,
		jwtManager: jwtManager,
	}
}

func (s *UserService) RegisterUser(ctx context.Context, req *req.UserRegisterRequest) (*model.User, error) {
	exist, err := s.repo.FindByUserName(ctx, req.UserName)
	if err != nil {
		return nil, err
	}
	if exist != nil {
		return nil, errors.New("用户名已存在")
	}
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		UserName:     req.UserName,
		Nickname:     req.Nickname,
		PasswordHash: passwordHash,
		Status:       model.UserStatusNormal,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	id, err := s.repo.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	if id == 0 {
		return nil, errors.New("用户注册失败")
	}
	user.ID = id
	return user, nil
}

func (s *UserService) LoginUser(ctx context.Context, req *req.UserLoginRequest) (*res.UserLoginResponse, error) {
	exist, err := s.repo.FindByUserName(ctx, req.UserName)
	if err != nil {
		return nil, err
	}
	if exist == nil || !auth.CheckPassword(req.Password, exist.PasswordHash) {
		return nil, errors.New("用户名或密码错误")
	}
	if exist.Status == model.UserStatusBanned {
		return nil, errors.New("账号已封禁")
	}
	token, expiresIn, err := s.jwtManager.GenerateToken(exist)
	if err != nil {
		return nil, err
	}
	return &res.UserLoginResponse{
		Token:     token,
		ExpiresIn: expiresIn,
		UserID:    exist.ID,
		UserName:  exist.UserName,
		Nickname:  exist.Nickname,
	}, nil
}
