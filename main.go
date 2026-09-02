package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store := NewStore()
	h := NewHandler(store)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: h,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
