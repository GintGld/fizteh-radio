package suite

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/joho/godotenv"

	"github.com/GintGld/fizteh-radio/internal/config"
	"github.com/GintGld/fizteh-radio/internal/models"
)

// Actual environment
var (
	_              = godotenv.Load("../.env")
	cfg            = config.MustLoadPath(os.Getenv("CONFIG_PATH"))
	rootPass       = os.Getenv("ROOT_PASS")
	passDefaultLen = 10
)

// RootLogin logins root user
func RootLogin() (string, error) {
	c := http.Client{Timeout: cfg.Timeout}

	bodyReq, err := json.Marshal(map[string]string{
		"login": "root",
		"pass":  rootPass,
	})

	if err != nil {
		return "", nil
	}

	url := "http://" + cfg.Address + "/login"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyReq))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	bodyResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var form struct {
		Token string `json:"token"`
	}

	if err = json.Unmarshal(bodyResp, &form); err != nil {
		return "", err
	}

	return form.Token, nil
}

// RootGetEditors gets all editors
func RootGetEditors() ([]models.EditorOut, error) {
	c := http.Client{Timeout: cfg.Timeout}

	token, err := RootLogin()
	if err != nil {
		return []models.EditorOut{}, err
	}

	url := "http://" + cfg.Address + "/root/editors"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []models.EditorOut{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.Do(req)
	if err != nil {
		return []models.EditorOut{}, err
	}

	defer resp.Body.Close()
	bodyResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return []models.EditorOut{}, err
	}

	var form struct {
		Editors []models.EditorOut `json:"Editors"`
	}

	if err = json.Unmarshal(bodyResp, &form); err != nil {
		return []models.EditorOut{}, err
	}

	return form.Editors, nil
}

func RandomFakePassword() string {
	return gofakeit.Password(true, true, true, true, false, passDefaultLen)
}
