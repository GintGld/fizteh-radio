package tests

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/gavv/httpexpect/v2"
	"github.com/stretchr/testify/require"

	"github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
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

	media := randomMedia()
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	res := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", "./source/sample-9s.mp3").
		WithFormField("media", string(mediaStr)).
		Expect()

	res.Status(200).
		JSON().
		Object().
		Keys().
		ContainsOnly("id")
}

// TODO check duration correctness

func TestGetMedia(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Create new media
	media := randomMedia()
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	// post media
	id := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", "./source/sample-9s.mp3").
		WithFormField("media", string(mediaStr)).
		Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	// Get media
	json := e.GET("/library/media/{id}", int64(id)).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON()

	// Check response
	json.Object().Keys().ContainsOnly("media")
	json.Path("$.media").Object().Keys().ContainsOnly("id", "name", "author", "duration")
	json.Path("$.media.name").String().IsEqual(*media.Name)
	json.Path("$.media.author").String().IsEqual(*media.Author)
	// json.Path("$.media.duration").Number()
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
	json := e.GET("/library/media/{id}", gofakeit.Uint32()).
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
	media := randomMedia()
	mediaStr, err := json.Marshal(media)
	require.NoError(t, err)

	id := e.POST("/library/media").
		WithHeader("Authorization", "Bearer "+token).
		WithMultipart().
		WithFile("source", "./source/sample-9s.mp3").
		WithFormField("media", string(mediaStr)).
		Expect().
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
		Name:   pointers.Pointer(gofakeit.MovieName()),
		Author: pointers.Pointer(gofakeit.Name()),
	}
}
