package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpdelivery "github.com/1chooo/ad-service/internal/delivery/http"
	"github.com/1chooo/ad-service/internal/repository"
	"github.com/1chooo/ad-service/internal/service"
)

func main() {
	addr := ":" + envOrDefault("PORT", "8080")
	databaseURL := envOrDefault("DATABASE_URL", "postgres://ad:ad@localhost:5432/ad_service?sslmode=disable")

	ctx := context.Background()
	repo, err := repository.NewAdRepository(ctx, databaseURL)
	if err != nil {
		log.Fatalf("initialize repository: %v", err)
	}
	defer repo.Close()

	svc := service.NewAdService(repo)

	if err := svc.RefreshCache(ctx); err != nil {
		log.Fatalf("warm cache: %v", err)
	}

	refreshCtx, stopRefresh := context.WithCancel(context.Background())
	defer stopRefresh()
	go runCacheRefresher(refreshCtx, svc)

	handler := httpdelivery.NewHandler(svc)
	server := &http.Server{
		Addr:              addr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("ad-service listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen and serve: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
}

func runCacheRefresher(ctx context.Context, svc *service.AdService) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := svc.RefreshCache(ctx); err != nil {
				log.Printf("refresh cache: %v", err)
			}
		}
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
