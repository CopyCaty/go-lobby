package main

import (
	"go-lobby/internal/handler"
	"go-lobby/internal/repository"
	"go-lobby/internal/service"
	"log"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func main() {

	db, err := sqlx.Connect(
		"mysql",
		"root:123456@tcp(127.0.0.1:3305)/go_lobby?parseTime=true&loc=Local",
	)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	r := gin.Default()

	// Static test page for local manual testing.
	r.Static("/static", "./static")
	r.StaticFile("/", "./static/index.html")

	v1 := r.Group("/api/v1")
	{
		v1.POST("/users/register", userHandler.RegisterUser)
	}

	r.Run(":8080")
}
