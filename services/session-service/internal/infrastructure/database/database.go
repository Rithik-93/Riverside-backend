package database

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/lakeside/services/session-service/internal/domain"
	"github.com/lakeside/services/session-service/internal/infrastructure"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var migrationsFS embed.FS

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

	/*
		Configure GORM logger based on environment
		In development: log all queries (Info level) for debugging
	*/
	env := os.Getenv("ENV")
	var gormLogger logger.Interface

	if env == "development" {
		gormLogger = logger.Default.LogMode(logger.Info)
	} else {
		gormLogger = logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logger.Error,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 gormLogger,
		PrepareStmt:            true,
		SkipDefaultTransaction: false,
	})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if env == "development" {
		log.Println("Running AutoMigrate (development mode)...")
		err = db.AutoMigrate(
			&domain.User{},
			&domain.Session{},
			&infrastructure.Pod{},
			&infrastructure.PodParticipant{},
			&infrastructure.Recording{},
			&infrastructure.UserRecordingLink{},
		)
		if err != nil {
			log.Fatal("Failed to migrate database:", err)
		}
		log.Println("Database migrations completed successfully")
	} else {
		runMigrations(dsn)
	}

	log.Println("Database connected successfully")
	return db
}

func runMigrations(dsn string) {
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("Failed to open database for migrations:", err)
	}
	defer sqlDB.Close()

	driver, err := migratepostgres.WithInstance(sqlDB, &migratepostgres.Config{})
	if err != nil {
		log.Fatal("Failed to create migration driver:", err)
	}

	migrationsSubFS, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		log.Fatalf("Failed to get migrations subdirectory from embedded FS: %v", err)
	}

	sourceDriver, err := iofs.New(migrationsSubFS, ".")
	if err != nil {
		log.Fatalf("Failed to create migration source driver: %v", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		"postgres",
		driver)
	if err != nil {
		log.Fatalf("Failed to create migrate instance: %v", err)
	}

	version, dirty, err := m.Version()
	if err == migrate.ErrNilVersion {
		log.Println("Database has no migrations yet, will apply all migrations...")
	} else if err != nil {
		log.Fatalf("Failed to get migration version: %v", err)
	} else if dirty {
		log.Printf("Database is in dirty state at version %d, attempting to force clean state...", version)
		if err := m.Force(int(version)); err != nil {
			log.Printf("Force() failed: %v. Attempting to manually clean dirty flag...", err)
			if err := cleanDirtyFlag(sqlDB, int(version)); err != nil {
				log.Fatalf("Failed to clean dirty flag for version %d: %v. "+
					"The database is in an inconsistent state. "+
					"You may need to manually update the schema_migrations table: "+
					"UPDATE schema_migrations SET dirty = false WHERE version = %d;", version, err, version)
			}
			log.Printf("Dirty flag manually cleared at version %d", version)
		} else {
			log.Printf("Dirty state cleared at version %d, continuing with migrations...", version)
		}
	} else {
		log.Printf("Current database version: %d (dirty: %v)", version, dirty)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Production migrations completed successfully")
}

func cleanDirtyFlag(db *sql.DB, version int) error {
	_, err := db.Exec("UPDATE schema_migrations SET dirty = false WHERE version = $1", version)
	if err != nil {
		return fmt.Errorf("failed to update schema_migrations: %w", err)
	}
	return nil
}
