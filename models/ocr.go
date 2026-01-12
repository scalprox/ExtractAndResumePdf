package models

type OcrResponse struct {
	Id     int
	Result OcrResult
	Error  error
}

type OcrResult struct {
	Blocks []OcrBlock `json:"blocks"`
}

type OcrBlock struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

type OcrQuery struct {
	Id        int
	ImagePath string
}
