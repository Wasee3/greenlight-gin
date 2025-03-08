package main

import (
	"github.com/gin-gonic/gin"
)

func (app *application) routes() *gin.Engine {

	router := gin.Default()

	router.Use(gin.Recovery(), app.RateLimiterMiddleware())
	router.GET("/v1/healthcheck", app.healthcheckHandler)
	router.GET("/v1/movie/:id", app.ShowMovieHandler)
	router.POST("/v1/movie", app.CreateMovieHandler)
	router.GET("/v1/movie", app.ListMovieHandler)
	router.PUT("/v1/movie/:id", app.UpdateMovieHandler)
	router.DELETE("/v1/movie/:id", app.DeleteMovieHandler)
	router.POST("/v1/user/register", app.RegisterUserHandler)
	router.POST("/v1/user/login", app.LoginUserHandler)
	router.POST("/v1/user/password/reset", app.PasswordResetHandler)
	router.POST("/v1/token/refresh", app.RefreshTokenHandler)

	return router
}
