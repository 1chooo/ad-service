package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := envOrDefault("PORT", "8080")

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	log.Printf("ad-service listening on %s", addr)
	if err := http.ListenAndServe(":"+addr, nil); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
