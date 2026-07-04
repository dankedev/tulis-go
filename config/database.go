package config

import (
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() {
	var err error
	var dialector gorm.Dialector

	switch AppConfig.DBDriver {
	case "postgres":
		dialector = postgres.Open(AppConfig.DatabaseURL)
	case "mysql":
		dialector = mysql.Open(AppConfig.DatabaseURL)
	case "sqlite":
		dialector = sqlite.Open(AppConfig.DatabaseURL)
	default:
		dialector = sqlite.Open("kontent.db")
	}

	DB, err = gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("Database connection established")
}
