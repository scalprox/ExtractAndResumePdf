package logic

import (
	"CrawlGameRules/models"
	"database/sql"
	"errors"
	"log"
)

// SaveGame is the entry point for any entry in the database. this function ensure no doublon is set, if a game already exist in the db, if the previous entry contains no rule, its use the new version to get rule.
func SaveGame(db *sql.DB, game *models.Game) {
	gameIdFromDb := checkExist(db, game)

	// game not already registered
	if gameIdFromDb == 0 {
		err := saveInDb(db, game)

		if err != nil {
			log.Println(err)
		}

		return
	}
	log.Printf("game already in db: %s", game.Name)
	return

}

func checkExist(db *sql.DB, game *models.Game) (id int) {
	var gameId int
	err := db.QueryRow("SELECT id FROM games_rule WHERE game_name = $1", game.Name+"%").Scan(&gameId)

	if errors.Is(err, sql.ErrNoRows) {
		return 0
	}

	if err != nil {
		return 0
	}

	if gameId != 0 {
		return gameId
	}

	return 0
}

func saveInDb(db *sql.DB, game *models.Game) error {
	editorId := retrieveEditor(db, game.Editor)

	if editorId == 0 {
		// need to create the editor
		err, newId := createEditor(db, game.Name)

		if err != nil {
			editorId = newId
		} else {
			log.Println("Error while creating the editor")
			return err
		}
	}

	_, err := db.Exec("INSERT INTO games_rule (url, game_name, status, vendor_id) VALUES ($1, $2, $3, $4)", game.Url, game.Name, "pending", editorId)

	if err != nil {
		log.Println("Error while creating new game in db")
		return err
	}

	return nil
}

// retrieveEditor return 0 if none found
func retrieveEditor(db *sql.DB, editorName string) (id int) {
	var editorId int

	err := db.QueryRow("SELECT id FROM vendors WHERE name = $1", editorName+"%").Scan(&editorId)

	if err != nil {
		return 0
	}

	if editorId != 0 {
		return editorId
	}

	return 0
}

func createEditor(db *sql.DB, editorName string) (error error, id int) {

	var editorId int
	err := db.QueryRow("INSERT INTO vendors (name, website_url) VALUES ($1, '') RETURNING id", editorName).Scan(&editorId)

	if err != nil {
		return err, 0
	}

	if editorId == 0 {
		return errors.New("id is not valid"), 0
	}

	return nil, editorId
}

func getGameFromId(db *sql.DB, id int) (error, *models.GameRule) {
	var gameRule models.GameRule

	err := db.QueryRow("SELECT id, url, text_content, game_name, summary, status, vendor_id FROM games_rule WHERE id = $1", id).Scan(&gameRule.Id, &gameRule.Url, &gameRule.TextContent, &gameRule.GameName, &gameRule.Summary, &gameRule.Status, &gameRule.VendorId)

	if err != nil {
		log.Println("Error while retrieving game from Id")
		return err, &models.GameRule{}
	}

	return nil, &gameRule
}

// isGameComplete check if the game is complete with the link to the game rules to download, the right editor set to it...
func isGameComplete(game *models.GameRule) bool {
	return game.Status == "finished"
}

func SaveGameDetail(db *sql.DB, game *models.GameDetail) error {
	_, err := db.Exec("INSERT INTO game_detail (name, link_to_rules, link_to_illustration, editor) VALUES ($1,$2,$3,$4)", game.Name, game.LinkToRules, game.LinkToIllustration, game.Editor)
	if err != nil {
		log.Println("Unable to save gameDetail")
		return err
	}
	return nil
}

func UpdateGameDetail(db *sql.DB, game *models.GameDetail) error {
	_, err := db.Exec("UPDATE game_detail SET ocr_result = $1, resume = $2, status = $3 WHERE id=$4", game.OcrResult, game.Resume, game.Status, game.Id)
	return err
}
