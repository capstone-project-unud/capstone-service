package main

import (
	"capstone/capstone-service/internal/httpapi"
	"capstone/capstone-service/internal/storage"
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	_ = godotenv.Load()
	if os.Getenv("DATABASE_URL") == "" || os.Getenv("AUTH_SERVICE_URL") == "" || os.Getenv("USER_SERVICE_URL") == "" || len(os.Getenv("AUTH_INTERNAL_TOKEN")) < 32 || len(os.Getenv("USER_INTERNAL_TOKEN")) < 32 {
		log.Fatal("Database, service URLs and internal tokens are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Invalid database configuration")
	}
	cfg.MaxConns = 12
	cfg.MinConns = 2
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		log.Fatal("Cannot initialize database")
	}
	defer db.Close()
	if db.Ping(ctx) != nil {
		log.Fatal("Cannot connect to database")
	}
	files, err := storage.New()
	if err != nil {
		log.Fatal(err)
	}
	app := httpapi.New(db, files)
	stop, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()
	go func() { <-stop.Done(); _ = app.ShutdownWithTimeout(10 * time.Second) }()
	host := os.Getenv("BIND_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
	if err = app.Listen(host + ":" + port); err != nil {
		log.Fatal(err)
	}
}
