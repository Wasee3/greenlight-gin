package data

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Update struct {
	Title   string         `json:"title" binding:"omitempty"`
	Year    int32          `json:"year" binding:"omitempty,gte=1999"`
	Runtime int32          `json:"runtime" binding:"omitempty"`
	Genres  pq.StringArray `json:"genres" binding:"omitempty"`
}

type Input struct {
	ID        int64     `json:"id" binding:"required"`
	CreatedAt time.Time `json:"-"`
	Title     string    `json:"title" binding:"required"`
	Year      int32     `json:"year" binding:"required,gte=1999"`
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

// Add a placeholder method for updating a specific record in the movies table.
func (m MovieModel) Update(c *gin.Context, movie *Movies) error {

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	err := m.db.WithContext(ctx).Save(&movie).Error
	if err != nil {
		return err
	}
	return nil
}

// Add a placeholder method for deleting a specific record from the movies table.
func (m MovieModel) Delete(id int64) error {
	return nil
}
