// Command api is the HTTP entry point for the pricing service.
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/application"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/cache"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/httpapi"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/observability"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/storage"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {

	m := observability.NewMetrics(reg)
	mem := storage.NewMemoryStore()
	pcache := cache.New(1024, 5*time.Minute)
	identity := application.BuildIdentity{
		Commit:    getenv("BUILD_COMMIT", "dev"),
		BuiltAt:   getenv("BUILD_BUILT_AT", time.Now().UTC().Format(time.RFC3339)),
		DatasetID: getenv("BUILD_DATASET_ID", ""),
	}

	if mode == "postgres" {
		dsn := getenv("DATABASE_URL", "postgres://pricing:pricing@localhost:5432/pricing?sslmode=disable")
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			log.Fatalf("open db: %v", err)
		}
		// register driver
		_ = stdlib.GetDefaultDriver
		if err := db.Ping(); err != nil {
			log.Printf("postgres unavailable, falling back to memory: %v", err)
		} else {
			app := application.NewPG(storage.NewPGStore(db), pcache, m, identity)
			s := httpapi.New(app)
			srv := &http.Server{Addr: addr, Handler: s, ReadHeaderTimeout: 5 * time.Second}
			runWithShutdown(srv)
			return
		}
	}
	app := application.New(mem, pcache, m, identity)
	s := httpapi.New(app)
	srv := &http.Server{Addr: addr, Handler: s, ReadHeaderTimeout: 5 * time.Second}
	runWithShutdown(srv)
}

func runWithShutdown(srv *http.Server) {
	go func() {
		log.Printf("api listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
