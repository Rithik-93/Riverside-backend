package pkg

import "os"

func LoadEnv() {
	if os.Getenv("DB_HOST") == "" {
		os.Setenv("DB_HOST", "localhost")
	}
	if os.Getenv("DB_PORT") == "" {
		os.Setenv("DB_PORT", "5432")
	}
	if os.Getenv("DB_USER") == "" {
		os.Setenv("DB_USER", "postgres")
	}
	if os.Getenv("DB_PASSWORD") == "" {
		os.Setenv("DB_PASSWORD", "password")
	}
	if os.Getenv("DB_NAME") == "" {
		os.Setenv("DB_NAME", "session_service")
	}
	if os.Getenv("JWT_ACCESS_SECRET") == "" {
		os.Setenv("JWT_ACCESS_SECRET", "your-access-secret-key")
	}
	if os.Getenv("JWT_REFRESH_SECRET") == "" {
		os.Setenv("JWT_REFRESH_SECRET", "your-refresh-secret-key")
	}
}