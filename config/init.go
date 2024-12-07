package config

import (
	"os"

	"github.com/joho/godotenv"
)

func init() {
	if os.Getenv("PRODUCTION") != "true" {
		if err := godotenv.Load(); err != nil {
			panic(err)
		}
	}

	database = new(databaseCfg).init()
}
