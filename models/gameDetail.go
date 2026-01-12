package models

type GameDetail struct {
	Id                 int
	Name               string
	LinkToRules        string
	LinkToIllustration string
	Editor             string
	Status             string
	OcrResult          *string
	Resume             *string
}
