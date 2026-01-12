package logic

import (
	"CrawlGameRules/models"
	"io"
	"log"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func ExtractGamesFromDoc(doc *goquery.Document, vendorId int, selector string) []models.GameRule {
	var games []models.GameRule

	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		href, exists := s.Attr("href")
		if exists {
			games = append(games, models.GameRule{
				Id:          0,
				Url:         href,
				TextContent: "",
				GameName:    text,
				Summary:     "",
				Status:      "pending",
				VendorId:    vendorId,
			})
		}
	})

	return games
}

func GetDocFromUrl(url string) (err error, doc *goquery.Document) {
	log.Printf("Retrieve page from %s", url)
	resp, err := client.Get(url)
	if err != nil {
		log.Print("Unable to retrieve html")
		return err, &goquery.Document{}
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Print("Unable to close body")
		}
	}(resp.Body)

	document, e := goquery.NewDocumentFromReader(resp.Body)
	if e != nil {
		log.Print("Unable to create document from reader")
		return e, &goquery.Document{}
	}

	return nil, document
}

func GetLinkFromDoc(doc *goquery.Document, selector string, textVerification *string) string {
	elem := doc.Find(selector)
	href, exists := elem.Attr("href")
	if exists {
		println(elem.Text())
		if textVerification == nil {
			return href
		}

		// verify if text is same as the link
		if elem.Text() == *textVerification {
			return href
		}
	}
	return ""
}

func GetTextFromDoc(doc *goquery.Document, selector string) string {
	elem := doc.Find(selector)
	return elem.Text()
}

func GetImgSrcFromDoc(doc *goquery.Document, selector string) string {
	elem := doc.Find(selector)
	src, exists := elem.Attr("src")
	if exists {
		return src
	}
	return ""
}

func GetTextFromSelection(sel *goquery.Selection, selector string) string {
	return strings.TrimSpace(sel.Find(selector).First().Text())
}

func GetLinkFromSelection(sel *goquery.Selection, selector string, contains *string) string {
	sel = sel.Find(selector).First()
	if contains != nil {
		if sel.Text() != *contains {
			return ""
		}
	}

	href, _ := sel.Attr("href")
	return href
}
