package logic

import (
	"CrawlGameRules/models"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	RulesFolderPath    = "./files/rules"
	RulesImgFolderPath = "./files/rules_img"
)

var client = &http.Client{Timeout: 5 * time.Minute}

func GetJsonFromPostUrl(url string, id int) (error error, data map[string]interface{}) {
	payload := map[string]int{
		"id": id,
	}
	payloadBody, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(payloadBody))
	var result map[string]interface{}
	if err != nil {
		log.Println("Error while retrieving json")
		return err, result
	}

	if resp.StatusCode != 200 {
		log.Println("Error for asked resource")
		log.Println(resp.Status)
		return errors.New("status not ok"), result
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		log.Println("Error wrong content type received")
		log.Println(ct)
		return errors.New("wrong content type"), result
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println("Unable to close body reader")
		}
	}(resp.Body)

	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		log.Println("Unable to read json")
		return err, result
	}

	return nil, result
}

func DownloadPdfFromLink(url string, gameId int) error {
	log.Printf("-- Start download id %d --", gameId)
	pathToRules := filepath.Join(RulesFolderPath)
	filePath := filepath.Join(pathToRules, strconv.Itoa(gameId)+".pdf")
	out, err := os.Create(filePath)
	if err != nil {
		log.Println("Unable to create file")
		return err
	}

	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			log.Println("Unable to close file out")
		}
	}(out)

	resp, err := client.Get(url)
	if err != nil {
		log.Println("Unable to get file")
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println("Unable to close body")
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error while downloading file with status : %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		return fmt.Errorf("error wrong content type : %s", ct)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Println("Unable to copy file")
		return err
	}

	log.Println("✅ Pdf saved")
	return nil
}

func ExtractImageOfPdf(fileId string) error {
	wslPdfPath := os.Getenv("WSL_PDF_PATH")
	wslPdfImgOutPath := os.Getenv("WSL_IMG_OUTPUT_PATH")
	if wslPdfPath == "" || wslPdfImgOutPath == "" {
		return errors.New("wsl path for pdf folder of output img folder are not set")
	}

	outputDir := filepath.Join(RulesImgFolderPath, fileId)

	err := os.MkdirAll(outputDir, 0777)
	if err != nil {
		return err
	}

	pdfPath := wslPdfPath + fileId + ".pdf"
	outputPrefix := wslPdfImgOutPath + fileId + "/page"

	cmd := exec.Command(
		"wsl",
		"pdftoppm",
		pdfPath,
		outputPrefix,
		"-png",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func ExtractTextFromImage(imagePath string) (*models.OcrResult, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", filepath.Base(imagePath))
	io.Copy(part, file)
	writer.Close()

	resp, err := client.Post("http://localhost:8000/ocr", writer.FormDataContentType(), body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ocr parser returned : %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	var result models.OcrResult

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func ExtractTextFromImages(images []models.OcrQuery) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := make(chan models.OcrResponse)
	errChan := make(chan error, 1)

	for _, image := range images {
		go func(img models.OcrQuery) {
			result, err := ExtractTextFromImage(img.ImagePath)
			if err != nil {
				select {
				case errChan <- err:
					cancel()
				default:
				}
				return
			}

			select {
			case results <- models.OcrResponse{
				Id:     img.Id,
				Result: *result,
			}:
			case <-ctx.Done():
				return
			}
		}(image)
	}

	var responses []models.OcrResponse

	for i := 0; i < len(images); i++ {
		select {
		case err := <-errChan:
			return "", err
		case res := <-results:
			responses = append(responses, res)
		}
	}

	sort.Slice(responses, func(i, j int) bool {
		return responses[i].Id < responses[j].Id
	})

	jsonBytes, err := json.Marshal(responses)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

func ExtractPageNumberFromText(text string) (int, bool) {
	re := regexp.MustCompile(`(?i)page[-\s]*(\d+)`)
	matches := re.FindStringSubmatch(text)

	if len(matches) < 2 {
		return 0, false
	}

	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}

	return n, true
}

func ResumeTextWithQwen(text string) (string, error) {
	ollamaSecret := os.Getenv("OLLAMA_API")
	if ollamaSecret == "" {
		return "", errors.New("ollama api secret key is not set")
	}

	url := "https://ollama.com/api/generate"

	payload := map[string]interface{}{
		"model":      "qwen3-vl:235b",
		"system":     Prompt,
		"prompt":     text,
		"stream":     true,
		"keep_alive": "0",
		"options": map[string]interface{}{
			"num_predict": 8192,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ollamaSecret)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf(
			"ollama API returned status %d (%s), headers: %v, body: %s",
			resp.StatusCode,
			resp.Status,
			resp.Header,
			string(body),
		)
	}

	scanner := bufio.NewScanner(resp.Body)
	var result strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		var chunk struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			fmt.Printf("failed to parse chunk: %s\n", line)
			continue
		}
		result.WriteString(chunk.Response)

		if chunk.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading streamed response: %w", err)
	}

	return result.String(), nil
}
