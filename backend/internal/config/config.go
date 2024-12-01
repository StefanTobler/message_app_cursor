package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ServerAddress string
	DatabaseURL   string
	JWTSecret     string
}

func Load() *Config {
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// Create data directory if it doesn't exist
	dataDir := filepath.Join(cwd, "data")
	os.MkdirAll(dataDir, 0755)

	// Default SQLite database path
	dbPath := filepath.Join(dataDir, "messenger.db")

	return &Config{
		ServerAddress: getEnv("SERVER_ADDRESS", ":8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "sqlite://"+dbPath),
		JWTSecret:     getEnv("JWT_SECRET", "your-secret-key"),
	}
}

// CleanDatabasePath returns a clean filesystem path from a database URL
func (c *Config) CleanDatabasePath() string {
	// Strip sqlite:// prefix if present
	dbPath := strings.TrimPrefix(c.DatabaseURL, "sqlite://")
	
	// If it's not an absolute path, make it relative to the current directory
	if !filepath.IsAbs(dbPath) {
		cwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		dbPath = filepath.Join(cwd, dbPath)
	}
	
	return dbPath
}

// UpdateDatabasePath updates the database path, maintaining the sqlite:// prefix if it was present
func (c *Config) UpdateDatabasePath(newPath string) {
	if strings.HasPrefix(c.DatabaseURL, "sqlite://") {
		c.DatabaseURL = "sqlite://" + newPath
	} else {
		c.DatabaseURL = newPath
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
} 