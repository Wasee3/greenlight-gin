package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Wasee3/greenlight-gin/internal/data"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

func (app *application) showMovieHandler(c *gin.Context) {
	idStr := c.Params.ByName("id")

	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing id parameter"})
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id parameter"})
		return
	}

	var movie *data.Movies
	movie, err = app.models.Movies.Get(c, id)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { // Handle "not found" error specifically
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Movie with ID %d not found", id)})
		} else {
			app.logger.Error("Database error", "error", err) // Log unexpected errors
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	var input data.Input
	err = copier.Copy(&input, &movie)
	if err != nil {
		app.logger.Error("Copier error", "error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error by Copier"})
		return
	}
	c.JSON(http.StatusOK, input)

}

func (app *application) createMovieHandler(c *gin.Context) {

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1048576)

	// var input struct {
	// 	ID      int64    `json:"id" binding:"required"`
	// 	Title   string   `json:"title" binding:"required"`
	// 	Year    int32    `json:"year" binding:"required,gte=1999"`
	// 	Runtime int32    `json:"runtime" binding:"required"`
	// 	Genres  []string `json:"genres" binding:"required"`
	// }

	var input data.Input

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	movie := &data.Movies{
		ID:        input.ID,
		CreatedAt: time.Now(),
		Title:     input.Title,
		Year:      input.Year,
		Runtime:   input.Runtime,
		Genres:    input.Genres,
		Version:   1,
	}

	err := app.models.Movies.Insert(c, movie)

	if err != nil {
		app.logger.Error("Failed to insert movie", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Data received successfully",
		"data":    input,
	})
}
