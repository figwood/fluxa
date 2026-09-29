package main

import (
	"log"

	"fluxa-api/internal/config"
	"fluxa-api/internal/infrastructure/persistence/gormdb"
)

func main() {
	cfg := config.Load()
	cfg.DBDriver = "postgres"
	cfg.AutoMigrate = true
	db, err := gormdb.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()
	log.Printf("PostgreSQL schema migration completed")
}
