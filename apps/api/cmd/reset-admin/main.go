package main

import (
	"log"
	"os"
	"strings"

	"fluxa-api/internal/config"
	"fluxa-api/internal/infrastructure/persistence/gormdb"
	"fluxa-api/internal/security"

	"gorm.io/gorm"
)

func main() {
	password := strings.TrimSpace(os.Getenv("RESET_ADMIN_PASSWORD"))
	if len(password) < 8 {
		log.Fatal("RESET_ADMIN_PASSWORD must contain at least 8 characters")
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		log.Fatal(err)
	}
	cfg := config.Load()
	cfg.DBDriver = "postgres"
	cfg.AutoMigrate = false
	db, err := gormdb.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}
	result := db.Model(&gormdb.UserModel{}).
		Where("user_name = ? OR user_email = ?", "admin", "admin@local").
		Updates(map[string]any{
			"user_password": hash,
			"auth_version":  gorm.Expr("auth_version + 1"),
		})
	if result.Error != nil {
		log.Fatal(result.Error)
	}
	if result.RowsAffected != 1 {
		log.Fatalf("expected one admin user, updated %d", result.RowsAffected)
	}
	log.Print("admin password reset; existing access tokens are invalidated")
}
