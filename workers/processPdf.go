package workers

import (
	"CrawlGameRules/logic"
	"CrawlGameRules/models"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

func ProcessPdf(db *sql.DB, id int, jobs <-chan models.GameDetail, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		log.Printf("⌛ Worker %d handle job %d", id, job.Id)
		ocrDone := *job.OcrResult != ""

		if !ocrDone {
			// extract img from the pdf
			err := logic.ExtractImageOfPdf(strconv.Itoa(job.Id))
			if err != nil {
				log.Println("Unable to extract image from pdf")
				log.Printf("❌ Worker %d encounter an error : %s", id, err)
				continue
			}

			// ocr the images
			imagesPath := filepath.Join(logic.RulesImgFolderPath, strconv.Itoa(job.Id))

			entries, err := os.ReadDir(imagesPath)
			if err != nil {
				log.Println("Unable to read images from folder")
				log.Fatal(err)
			}

			var ocrQueries []models.OcrQuery

			// count number of images in folder
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				name := entry.Name()

				page, ok := logic.ExtractPageNumberFromText(name)
				if ok {
					ocrQueries = append(ocrQueries, models.OcrQuery{
						Id:        page,
						ImagePath: imagesPath + "/" + name,
					})
				}
			}

			if len(ocrQueries) == 0 {
				log.Println("No image found")
				log.Printf("❌ Worker %d encounter an error : %s", id, err)
				continue
			}

			ocrExtraction, err := logic.ExtractTextFromImages(ocrQueries)
			if err != nil {
				log.Println("Unable to extract text from images with ocr")
				log.Printf("❌ Worker %d encounter an error : %s", id, err)
				continue
			}
			job.OcrResult = &ocrExtraction
		}

		// ask ai to resume the ocr
		resume, err := logic.ResumeTextWithQwen(*job.OcrResult)
		if err != nil {
			log.Println("Unable to resume with ollama")
			log.Printf("❌ Worker %d encounter an error : %s", id, err)
			continue
		}

		// store in db
		job.Resume = &resume
		job.Status = "finished"

		err = logic.UpdateGameDetail(db, &job)
		if err != nil {
			log.Println("Unable to update gameDetail")
			log.Printf("❌ Worker %d encounter an error : %s", id, err)
			continue
		}

		log.Printf("✅ Worker %d finished job %d", id, job.Id)
	}
}
