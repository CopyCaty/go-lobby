package service

import (
	"context"
	"errors"
	"go-lobby/internal/auth"
	"go-lobby/internal/dto/req"
	"go-lobby/internal/model"
	"go-lobby/internal/repository"
	"time"
)

type UserService struct {
	repo *repository.UserRepository
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{
		repo: repo,
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
