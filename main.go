package main

import (
	"log"
	"net/http"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"

	"github.com/tomicleveling/core/pkg/authenticator"
	"github.com/tomicleveling/core/pkg/router"
)

func main() {
	log.Println("Starting the application...")
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Failed to load the env vars: %v", err)
	}

	auth, err := authenticator.New()
	if err != nil {
		log.Fatalf("Failed to initialize the authenticator: %v", err)
	}

	mux := djrouter.InitRouter(auth)
	err = http.ListenAndServe(":3000", mux)
	if err != nil {
		log.Fatal(err)
	}
}
