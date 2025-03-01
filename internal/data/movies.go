package data

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

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

	if err := m.db.WithContext(c).Create(&movie).Error; err != nil {
		return err
	}
	return nil
}

// Add a placeholder method for fetching a specific record from the movies table.
func (m MovieModel) Get(id int64) (*Movies, error) {
	return nil, nil
}

// Add a placeholder method for updating a specific record in the movies table.
func (m MovieModel) Update(movie *Movies) error {
	return nil
}

// Add a placeholder method for deleting a specific record from the movies table.
func (m MovieModel) Delete(id int64) error {
	return nil
}
