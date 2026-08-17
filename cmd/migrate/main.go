// Command migrate runs forward SQL migrations from the migrations/
// directory against the configured database.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "postgres DSN")
	dir := flag.String("dir", "migrations", "migrations directory")
	flag.Parse()
	if *dsn == "" {
		*dsn = "postgres://pricing:pricing@localhost:5432/pricing?sslmode=disable"
	}
	db, err := sql.Open("pgx", *dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(*dir, "*.sql"))
	if err != nil {
		log.Fatal(err)
	}
	sort.Strings(files)
	for _, f := range files {
		log.Printf("applying %s", filepath.Base(f))
		b, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			log.Fatalf("apply %s: %v", f, err)
		}
	}
	fmt.Printf("applied %d migration(s)\n", len(files))
}
