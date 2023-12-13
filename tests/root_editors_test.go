package tests

import (
	"net/url"
	"strconv"
	"testing"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/gavv/httpexpect/v2"
	"github.com/stretchr/testify/require"

	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/tests/suite"
)

func TestGetEditors(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	e.GET("/root/editors").
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON().
		Object().
		Keys().
		ContainsOnly("editors")
}

func TestCreateNewEditor(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	login := gofakeit.Name()
	pass := suite.RandomFakePassword()

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	e.POST("/root/editors").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			User models.EditorIn `json:"editor"`
		}{
			User: models.EditorIn{
				Login: login,
				Pass:  pass,
			},
		}).Expect().
		Status(200).
		JSON().
		Object().
		Keys().
		ContainsOnly("id")
}

func TestDoubleCreateEditor(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	login := gofakeit.Name()
	pass := suite.RandomFakePassword()

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// create user correctly
	e.POST("/root/editors").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			User models.EditorIn `json:"editor"`
		}{
			User: models.EditorIn{
				Login: login,
				Pass:  pass,
			},
		}).Expect().
		Status(200).
		JSON().
		Object().
		Keys().
		ContainsOnly("id")

	// trying to create user with same login
	resp := e.POST("/root/editors").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			User models.EditorIn `json:"editor"`
		}{
			User: models.EditorIn{
				Login: login,
				Pass:  pass,
			},
		}).Expect().
		Status(400)

	json := resp.JSON()

	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("editor exists")
}

func TestGetEditor(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	login := gofakeit.Name()
	pass := suite.RandomFakePassword()

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Create new editors
	id := e.POST("/root/editors").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			User models.EditorIn `json:"editor"`
		}{
			User: models.EditorIn{
				Login: login,
				Pass:  pass,
			},
		}).Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	// Get editor
	json := e.GET("/root/editor/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200).
		JSON()

	// Check response
	json.Object().Keys().ContainsOnly("editor")
	json.Path("$.editor").Object().Keys().ContainsOnly("login", "id")
	json.Path("$.editor.login").String().IsEqual(login)
	json.Path("$.editor.id").Number().IsEqual(id)
}

func TestGetNotExistingEditor(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Get not existing editor
	uGet := "/root/editor/" + strconv.Itoa(int(gofakeit.Uint32()))
	json := e.GET(uGet).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	// Check response
	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("editor not found")
}

func TestDeleteEditor(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	login := gofakeit.Name()
	pass := suite.RandomFakePassword()

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Create new editor
	id := e.POST("/root/editors").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(struct {
			User models.EditorIn `json:"editor"`
		}{
			User: models.EditorIn{
				Login: login,
				Pass:  pass,
			},
		}).Expect().
		Status(200).
		JSON().
		Path("$.id").
		Number().
		Raw()

	// Delete editor
	e.DELETE("/root/editor/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(200)

	// Check the deletion
	json := e.GET("/root/editor/{id}", id).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("editor not found")
}

func TestDeleteNotExistingEditor(t *testing.T) {
	token, err := suite.RootLogin()
	require.NoError(t, err)

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// Trying to delete not existing editor
	json := e.DELETE("/root/editor/{id}", gofakeit.Uint32()).
		WithHeader("Authorization", "Bearer "+token).
		Expect().
		Status(400).
		JSON()

	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("editor not found")
}
