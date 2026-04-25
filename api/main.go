package main

import (
	"go-lobby/config"
	"go-lobby/internal/auth"
	"go-lobby/internal/handler"
	"go-lobby/internal/middleware"
	"go-lobby/internal/repository"
	"go-lobby/internal/service"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	db, err := sqlx.Connect(
		cfg.Database.Type,
		cfg.Database.DSN,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	gin.SetMode(gin.DebugMode)

	jwtManager := auth.NewJWTManager(cfg.JWT.Secret, time.Duration(cfg.JWT.ExpireSec)*time.Second)

	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo, jwtManager)
	userHandler := handler.NewUserHandler(userService)
	roomService := service.NewRoomService()
	roomHandler := handler.NewRoomHandler(roomService)
	matchRepo := repository.NewMatchRepository(db)
	matchService := service.NewMatchService(matchRepo)
	matchQueueService := service.NewMatchQueueService(matchService, roomService)
	matchHandler := handler.NewMatchQueueHandler(matchQueueService)

	r := gin.Default()

	// Static test page for local manual testing.
	r.Static("/static", "./static")
	r.StaticFile("/", "./static/index.html")

	v1 := r.Group("/api/v1")
	{
		v1.POST("/users/register", userHandler.RegisterUser)
		v1.POST("/users/login", userHandler.LoginUser)

		authGroup := v1.Group("/")
		authGroup.Use(middleware.AuthMiddleware(jwtManager))
		{
			authGroup.GET("/me", userHandler.Me)

			matchGroup := authGroup.Group("/match")
			{
				queueGroup := matchGroup.Group("/queue")
				{
					queueGroup.POST("/join", matchHandler.Join)
					queueGroup.GET("/status", matchHandler.Status)
					queueGroup.POST("/cancel", matchHandler.Cancel)
				}
			}
			roomGroup := authGroup.Group("/room")
			{
				roomGroup.GET("/:id", roomHandler.GetRoom)
				roomGroup.POST("/:id/ready", roomHandler.Ready)
			}

		}

	}

	r.Run(cfg.Server.Addr)
}
