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

func (app *application) ShowMovieHandler(c *gin.Context) {
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

func (app *application) CreateMovieHandler(c *gin.Context) {

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1048576)

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

func (app *application) UpdateMovieHandler(c *gin.Context) {

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1048576)

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

	var update data.Update
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var movie *data.Movies
	movie, err2 := app.models.Movies.Get(c, id)

	if err2 != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { // Handle "not found" error specifically
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Movie with ID %d not found", id)})
		} else {
			app.logger.Error("Database error", "error", err2) // Log unexpected errors
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	if update.Title != "" {
		movie.Title = update.Title
	}
	if update.Year != 0 {
		movie.Year = update.Year
	}
	if update.Runtime != 0 {
		movie.Runtime = update.Runtime
	}
	if len(update.Genres) > 0 {
		uniqueGenres := make(map[string]bool)

		for _, genre := range movie.Genres {
			uniqueGenres[genre] = true
		}

		for _, genre := range update.Genres {
			if !uniqueGenres[genre] {
				movie.Genres = append(movie.Genres, genre)
				uniqueGenres[genre] = true
			}
		}
	}
	movie.Version++

	err = app.models.Movies.Update(c, movie)

	if err != nil {
		app.logger.Error("Failed to update movie", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}

	c.JSON(http.StatusOK, gin.H{"Updated": update})
}

func (app *application) DeleteMovieHandler(c *gin.Context) {
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

	err = app.models.Movies.Delete(c, id)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Movie with ID %d not found", id)})
		} else {
			app.logger.Error("Database error", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"Message": fmt.Sprintf("Movie with ID %d deleted", id)})
}
