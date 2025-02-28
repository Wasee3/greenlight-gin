package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Wasee3/greenlight-gin/internal/data"
	"github.com/gin-gonic/gin"
)

func (app *application) showMovieHandler(c *gin.Context) {
	idStr := c.Params.ByName("id")

	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing id parameter"})
		return
	}

	movie := data.Movie{
		ID:        123,
		CreatedAt: time.Now(),
		Title:     "Casablanca",
		Runtime:   102,
		Genres:    []string{"drama", "romance", "war"},
		Version:   1,
	}
	// Encode the struct to JSON and send it as the HTTP response.

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id parameter"})
		return
	}

	if id == movie.ID {
		c.JSON(http.StatusOK, movie)
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
	}
}

func (app *application) createMovieHandler(c *gin.Context) {

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1048576)

	var input struct {
		Title   string   `json:"title" binding:"required"`
		Year    int32    `json:"year" binding:"required,gte=1999"`
		Runtime int32    `json:"runtime" binding:"required"`
		Genres  []string `json:"genres" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Data received successfully",
		"data":    input,
	})
}
