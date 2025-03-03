package main

import (
	"sync"
	"time"

	"github.com/Wasee3/greenlight-gin/internal/data"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	totalMoviesCount int64
	countMutex       sync.RWMutex
)

func openDB(cfg config) (*gorm.DB, error) {
	dsn := cfg.db.dsn
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// sqlDB, err := db.DB()
	// if err != nil {
	// 	log.Fatal("Failed to get database instance:", err)
	// }

	// Set connection pooling parameters
	// sqlDB.SetMaxOpenConns(app.cfg.maxOpenConns)                    // Maximum number of open connections
	// sqlDB.SetMaxIdleConns(app.cfg.maxIdleConns)                    // Maximum number of idle connections
	// sqlDB.SetConnMaxIdleTime(app.cfg.maxIdleTime)    // Idle connection timeout
	// app.logger.Info("database connection pool established")
	return db, nil
}

func UpdateMovieCount(db *gorm.DB) {
	ticker := time.NewTicker(30 * time.Second) // Adjust interval as needed
	defer ticker.Stop()

	for range ticker.C {
		var count int64
		_ = db.Model(&data.Movies{}).Count(&count).Error

		// Safely update the count
		countMutex.Lock()
		totalMoviesCount = count
		countMutex.Unlock()
	}
}
