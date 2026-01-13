package main

import (
	"CrawlGameRules/models"
	"CrawlGameRules/workers"
	"database/sql"
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func Db() *sql.DB {
	pgUrl := os.Getenv("PG_URL")
	if pgUrl == "" {
		panic("Url for database is not set")
	}

	db, err := sql.Open("postgres", pgUrl)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Unable to load .env file")
	}

	var db = Db()

	rows, err := db.Query("SELECT id, name, link_to_rules, link_to_illustration, editor, status, ocr_result, resume FROM game_detail WHERE status = 'pending' ORDER BY id LIMIT 1")
	if err != nil {
		log.Fatal(err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Println(err)
		}
	}(rows)

	jobs := make(chan models.GameDetail)

	var wg sync.WaitGroup
	numWorkers := 3

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go workers.ProcessPdf(db, i, jobs, &wg)
	}

	for rows.Next() {
		var game models.GameDetail
		if err := rows.Scan(&game.Id, &game.Name, &game.LinkToRules, &game.LinkToIllustration, &game.Editor, &game.Status, &game.OcrResult, &game.Resume); err != nil {
			log.Println(err)
			continue
		}
		jobs <- game
	}

	close(jobs)
	wg.Wait()
}
