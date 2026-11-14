package main

import (
	"bytes"
	"capstone/capstone-service/internal/storage"
	"context"
	"errors"
	"fmt"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"os"
	"time"
)

func main() {
	_ = godotenv.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	failed := false
	if len(os.Args) < 2 || os.Args[1] != "--storage-only" {
		db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
		if err != nil {
			fmt.Println("Neon: konfigurasi tidak valid.")
			failed = true
		} else {
			defer db.Close()
			var role, database string
			var tables int
			err = db.QueryRow(ctx, "SELECT current_user,current_database(),(SELECT count(*) FROM information_schema.tables WHERE table_schema='capstone_service')").Scan(&role, &database, &tables)
			if err != nil {
				fmt.Printf("Neon: gagal memeriksa koneksi (%T).\n", err)
				failed = true
			} else if role == "neondb_owner" || tables < 7 {
				fmt.Println("Neon: role runtime atau migrasi belum sesuai.")
				failed = true
			} else {
				fmt.Printf("Neon PASS: database=%s, role=%s, tables=%d.\n", database, role, tables)
			}
		}
	}
	files, err := storage.New()
	if err != nil {
		fmt.Println("Storage: konfigurasi belum lengkap.")
		failed = true
	} else {
		key := "integration-check-" + uuid.NewString() + ".txt"
		data := []byte("capstone integration check")
		if err = files.Put(ctx, key, data, "text/plain"); err == nil {
			var actual []byte
			actual, err = files.Get(ctx, key)
			if err == nil && !bytes.Equal(actual, data) {
				err = fmt.Errorf("object mismatch")
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
			cleanupErr := files.Delete(cleanupCtx, key)
			cleanupCancel()
			if err == nil {
				err = cleanupErr
			}
		}
		if err != nil {
			var api smithy.APIError
			if errors.As(err, &api) {
				fmt.Printf("Storage gagal: %s.\n", api.ErrorCode())
			} else {
				fmt.Printf("Storage gagal (%T).\n", err)
			}
			failed = true
		} else {
			fmt.Println("Storage PASS: put/get/delete objek uji, tanpa mengubah objek lain.")
		}
	}
	if failed {
		os.Exit(1)
	}
}
