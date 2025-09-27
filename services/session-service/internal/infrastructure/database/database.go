package database

import (
	"fmt"
	"log"
	"os"

	"github.com/lakeside/services/session-service/internal/domain"
	"github.com/lakeside/services/session-service/internal/infrastructure"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect() *gorm.DB {
	var dsn string
	
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		dsn = dbURL
	} else {
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
			os.Getenv("DB_HOST"),
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_NAME"),
			os.Getenv("DB_PORT"),
		)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if shouldMigrate(db) {
		log.Println("Running database migrations...")
		err = db.AutoMigrate(
			&domain.User{}, 
			&domain.Session{},
			&infrastructure.Pod{},
			&infrastructure.PodParticipant{},
		)
		if err != nil {
			log.Fatal("Failed to migrate database:", err)
		}
		log.Println("Database migrations completed successfully")
	} else {
		log.Println("Database tables already exist, skipping migration")
	}

	log.Println("Database connected successfully")
	return db
}

func shouldMigrate(db *gorm.DB) bool {
	// Check if users table exists (our main table)
	if !db.Migrator().HasTable(&domain.User{}) {
		return true
	}
	
	if !db.Migrator().HasTable(&domain.Session{}) {
		return true
	}
	
	if !db.Migrator().HasTable(&infrastructure.Pod{}) {
		return true
	}
	
	if !db.Migrator().HasTable(&infrastructure.PodParticipant{}) {
		return true
	}
	
	return false
}
