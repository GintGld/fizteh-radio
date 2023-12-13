package tests

import (
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/gavv/httpexpect/v2"
	"github.com/stretchr/testify/require"

	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/tests/suite"
)

func TestCreateNewMedia(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			User models.Media `json:"media"`
		}{
			User: randomMedia(),
		}).Expect().
		Status(200).
		JSON().
		Object().
		Keys().
		ContainsOnly("id")
}

func TestGetMedia(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	media := randomMedia()

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Create new media
	id := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			User models.Media `json:"media"`
		}{
			User: media,
		}).Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	// Get editor
	json := e.GET("/library/media/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON()

	// Check response
	json.Object().Keys().ContainsOnly("media")
	json.Path("$.media").Object().Keys().ContainsOnly("id", "name", "author", "duration")
	json.Path("$.media.name").String().IsEqual(media.Name)
	json.Path("$.media.author").String().IsEqual(media.Author)
	json.Path("$.media.duration").Number().IsEqual(media.Duration)
}

func TestGetNotExistingMedia(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Get not existing editor
	uGet := "/library/media/" + strconv.Itoa(int(gofakeit.Uint32()))
	json := e.GET(uGet).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	// Check response
	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("media not found")
}

func TestDeleteMedia(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Create new editor
	id := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			User models.Media `json:"media"`
		}{
			User: randomMedia(),
		}).Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	// Delete editor
	e.DELETE("/library/media/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200)

	// Check the deletion
	json := e.GET("/library/media/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("media not found")
}

func TestDeleteNotExistingMedia(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Trying to delete not existing editor
	json := e.DELETE("/library/media/{id}", gofakeit.Uint32()).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("media not found")
}

func randomMedia() models.Media {
	return models.Media{
		Name:     gofakeit.MovieName(),
		Author:   gofakeit.Name(),
		Duration: time.Duration(gofakeit.IntRange(100, 100000)) * time.Millisecond,
	}
}
