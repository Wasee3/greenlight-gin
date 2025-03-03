package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Metadata struct {
	CurrentPage  int   `json:"current_page"`
	PageSize     int   `json:"page_size"`
	FirstPage    int   `json:"first_page"`
	LastPage     int   `json:"last_page"`
	TotalRecords int64 `json:"total_records"`
}

type Filters struct {
	Page     int    `form:"page" binding:"numeric,gte=1"`
	PageSize int    `form:"pagesize" binding:"numeric,gte=1"`
	Sort     string `form:"sort" binding:"alpha,oneof=id title year"`
	Order    string `form:"order" binding:"alpha,oneof=asc desc"`
	Pretty   bool   `form:"pretty" binding:"boolean"`
	Title    string `form:"title" binding:"omitempty"`
}

type Update struct {
	Title   string         `json:"title" binding:"omitempty"`
	Year    int32          `json:"year" binding:"omitempty,gte=1947"`
	Runtime int32          `json:"runtime" binding:"omitempty"`
	Genres  pq.StringArray `json:"genres" binding:"omitempty"`
}

type Input struct {
	ID        int64     `json:"id" binding:"required"`
	CreatedAt time.Time `json:"-"`
	Title     string    `json:"title" binding:"required"`
	Year      int32     `json:"year" binding:"required,gte=1947"`
	Runtime   int32     `json:"runtime" binding:"required"`
	Genres    []string  `json:"genres" binding:"required"`
	Version   int32     `json:"-"`
}

type Movies struct {
	ID        int64          `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	Title     string         `gorm:"not null"`
	Year      int32          `gorm:"not null"`
	Runtime   int32          `gorm:"not null"`
	Genres    pq.StringArray `gorm:"type:text[]"`
	Version   int32          `gorm:"default:1"`
}

type MovieModel struct {
	db *gorm.DB
}

// Add a placeholder method for inserting a new record in the movies table.
func (m MovieModel) Insert(c *gin.Context, movie *Movies) error {

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := m.db.WithContext(ctx).Create(&movie).Error; err != nil {
		return err
	}
	return nil
}

// Add a placeholder method for fetching a specific record from the movies table.
func (m MovieModel) Get(c *gin.Context, id int64) (*Movies, error) {

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var movie Movies
	err := m.db.WithContext(ctx).First(&movie, id).Error // Fetch movie with ID = 1
	if err != nil {
		return nil, err
	}
	return &movie, nil
}

func (m *MovieModel) UpdateMovieInTransaction(c *gin.Context, id int64, update Update) (*Movies, error) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var updatedMovie Movies

	// Start a transaction
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var movie Movies

		// Retrieve movie record inside the transaction
		if err := tx.Where("id = ?", id).First(&movie).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return fmt.Errorf("db_error: %w", err) // Wrap other DB errors
		}

		// Apply updates
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

		// Optimistic locking: Ensure the version matches before updating
		prevVersion := movie.Version
		movie.Version++

		result := tx.Model(&movie).
			Where("id = ? AND version = ?", movie.ID, prevVersion).
			Updates(&movie)

		if result.Error != nil {
			return fmt.Errorf("db_error: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return fmt.Errorf("concurrent_update: Movie was modified by another request")
		}

		// Store the updated movie for return
		updatedMovie = movie
		return nil // Commit transaction
	})

	if err != nil {
		return nil, err
	}

	return &updatedMovie, nil
}

// Add a placeholder method for deleting a specific record from the movies table.
func (m MovieModel) Delete(c *gin.Context, id int64) error {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 100*time.Second)
	defer cancel()

	// result := m.db.Debug().WithContext(ctx).Where("ID = ?", id).Delete(&Movies{}) // Prints Query
	result := m.db.WithContext(ctx).Where("ID = ?", id).Delete(&Movies{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound // Custom error if no rows were deleted
	}

	return nil
}

func (m MovieModel) List(c *gin.Context, filter *Filters) (*[]Movies, error) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 100*time.Second)
	defer cancel()

	offset := (filter.Page - 1) * filter.PageSize

	var movies []Movies
	err := m.db.WithContext(ctx).Order(filter.Sort + " " + filter.Order).Limit(filter.PageSize).Offset(offset).Find(&movies).Error
	if err != nil {
		return nil, err
	}

	return &movies, nil
}

func (m MovieModel) Search(c *gin.Context, filter *Filters) (*[]Movies, int64, error) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 100*time.Second)
	defer cancel()

	var movies []Movies
	var totalRecords int64
	err := m.db.WithContext(ctx).Raw(`
	SELECT * FROM movies WHERE to_tsvector('english', title) @@ plainto_tsquery(?) 
	ORDER BY ts_rank_cd(to_tsvector('english', title), plainto_tsquery(?)) DESC`, filter.Title, filter.Title).
		Scan(&movies).Error
	if err != nil {
		return nil, 0, err
	}

	if len(movies) > 0 {
		totalRecords = int64(len(movies))
	}

	return &movies, totalRecords, nil
}
