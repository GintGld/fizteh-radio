package tests

import (
	"encoding/json"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/gavv/httpexpect/v2"
	"github.com/stretchr/testify/require"

	"github.com/GintGld/fizteh-radio/internal/lib/ffmpeg"
	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
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

func TestGetMedia(t *testing.T) {
	sourceFile := "./source/sample-9s.mp3"

	durationStr, err := ffmpeg.GetMeta(&sourceFile, "duration")
	require.NoError(t, err)

	seconds, err := strconv.ParseFloat(durationStr, 64)
	require.NoError(t, err)

	duration := time.Duration(seconds * 1000000000)

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
		WithFile("source", sourceFile).
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
	json.Path("$.media.duration").Number().IsEqual(duration)
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

func TestTagTypes(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	json := e.GET("/library/tag/types").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON()

	json.Object().Keys().ContainsOnly("types")
	for _, value := range json.Path("$.types").Array().Iter() {
		value.Object().Keys().ContainsOnly("id", "name")
	}
}

func TestNewTag(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	res := e.POST("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			Tag models.Tag `json:"tag"`
		}{
			Tag: randomTag(),
		}).
		Expect()

	res.Status(200).
		JSON().
		Object().
		Keys().
		ContainsOnly("id")
}

func TestAllTags(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	json := e.GET("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON()

	json.Object().Keys().ContainsOnly("tags")
	for _, value := range json.Path("$.tags").Array().Iter() {
		value.Object().Keys().ContainsOnly("id", "name", "type")
		value.Path("$.type").Object().Keys().ContainsOnly("id", "name")
	}
}

func TestDeleteTag(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	res := e.POST("/library/tag").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			Tag models.Tag `json:"tag"`
		}{
			Tag: randomTag(),
		}).
		Expect()

	idRaw := res.
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()
	id := int(idRaw)

	e.DELETE("/library/tag/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200)
}

func TestDeleteNotExistingTag(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Trying to delete not existing editor
	json := e.DELETE("/library/tag/{id}", gofakeit.Uint32()).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("tag not found")
}

func randomMedia() models.Media {
	return models.Media{
		Name:   ptr.Ptr(gofakeit.MovieName()),
		Author: ptr.Ptr(gofakeit.Name()),
	}
}

func randomTag() models.Tag {
	return models.Tag{
		Name: gofakeit.Adjective(),
		Type: models.TagType{
			ID: gofakeit.Int64(),
		},
	}
}
