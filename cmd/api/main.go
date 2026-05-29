package main

import (
	"context"
	"log"
	"time"

	"go-lobby/config"
	"go-lobby/internal/auth"
	"go-lobby/internal/cache"
	"go-lobby/internal/handler"
	"go-lobby/internal/middleware"
	"go-lobby/internal/mq"
	"go-lobby/internal/repository"
	"go-lobby/internal/service"
	"go-lobby/internal/ws"

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

	redisClient := cache.NewRedisClient(&cfg.Redis)
	if err := cache.PingRedis(context.Background(), redisClient); err != nil {
		log.Fatal(err)
	}
	defer redisClient.Close()

	gin.SetMode(gin.DebugMode)

	publisher, err := mq.NewPublisher(cfg.RabbitMQ)
	if err != nil {
		log.Fatal(err)
	}
	defer publisher.Close()

	jwtManager := auth.NewJWTManager(cfg.JWT.Secret, time.Duration(cfg.JWT.ExpireSec)*time.Second)

	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo, jwtManager)
	userHandler := handler.NewUserHandler(userService)
	roomHub := ws.NewRoomHub()
	mapHub := ws.NewMapHub()
	roomService := service.NewRoomService()
	roomHandler := handler.NewRoomHandler(roomService, roomHub)
	matchRepo := repository.NewMatchRepository(db)
	matchService := service.NewMatchService(matchRepo, publisher)

	matchQueueRepo := repository.NewMatchQueueRepository(redisClient)
	matchQueueService := service.NewMatchQueueService(matchService, roomService, matchQueueRepo)
	matchQueueHandler := handler.NewMatchQueueHandler(matchQueueService)
	matchHandler := handler.NewMatchHandler(matchService)

	rankRepo := repository.NewRankRepository(db)
	leaderboardRepo := repository.NewLeaderboardRepository(redisClient)
	rankService := service.NewRankService(rankRepo, matchRepo, leaderboardRepo)
	leaderboardHandler := handler.NewLeaderboardHandler(rankService)
	mineSeasonRepo := repository.NewMineSeasonRepository(db)
	mineChunkStateRepo := repository.NewMineChunkStateRepository(redisClient)
	chunkService := service.NewChunkServiceWithDeps(mineSeasonRepo, mineChunkStateRepo, cfg.Mine.ChunkClosureDurationValue())
	chunkHandler := handler.NewChunkHandler(chunkService, mapHub)

	r := gin.Default()

	// Static test page for local manual testing.
	r.Static("/static", "./static")
	r.StaticFile("/", "./static/index.html")

	v1 := r.Group("/api/v1")
	{
		v1.POST("/users/register", userHandler.RegisterUser)
		v1.POST("/users/login", userHandler.LoginUser)
		v1.GET("/map/chunks", chunkHandler.ListChunks)
		v1.GET("/map/chunks/:chunk_id", chunkHandler.GetChunk)
		v1.GET("/map/chunks/:chunk_id/snapshot", chunkHandler.GetSnapshot)

		authGroup := v1.Group("/")
		authGroup.Use(middleware.AuthMiddleware(jwtManager))
		{
			authGroup.GET("/leaderboard", leaderboardHandler.Top)
			wsGroup := authGroup.Group("/ws")
			{
				wsHandler := handler.NewWSHandler(roomService, roomHub)
				mapWSHandler := handler.NewMapWSHandler(mapHub)
				wsGroup.GET("/room/:id", wsHandler.JoinRoom)
				wsGroup.GET("/map", mapWSHandler.JoinMap)
			}

			authGroup.GET("/me", userHandler.Me)
			authGroup.POST("/map/chunks/:chunk_id/open", chunkHandler.OpenCell)
			authGroup.POST("/map/chunks/:chunk_id/flag", chunkHandler.FlagCell)

			matchGroup := authGroup.Group("/match")
			{
				queueGroup := matchGroup.Group("/queue")
				{
					queueGroup.POST("/join", matchQueueHandler.Join)
					queueGroup.GET("/status", matchQueueHandler.Status)
					queueGroup.POST("/cancel", matchQueueHandler.Cancel)
				}
				matchGroup.POST("/result", matchHandler.SetMatchResult)
				matchGroup.GET("/:id", matchHandler.GetMatchInfo)
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
