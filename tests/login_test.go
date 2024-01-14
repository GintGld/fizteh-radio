package tests

import (
	"net/url"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/gavv/httpexpect/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/tests/suite"
)

// Correctness of login root
// checks http responce and JWT
func TestLoginRoot(t *testing.T) {
	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	timestamp := time.Now()

	resp := e.POST("/login").
		WithJSON(models.EditorIn{
			Login: "root",
			Pass:  rootPass,
		}).
		Expect().
		Status(200)

	json := resp.JSON()

	// response must be {"token" : "string"}
	json.Object().Keys().ContainsOnly("token")

	// extract token value as string
	tokenString := json.Path("$.token").String().Raw()

	claims := jwt.MapClaims{}

	// parse token
	token, err := jwt.NewParser().ParseWithClaims(tokenString, claims, func(_ *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	// validate token
	require.Truef(t, token.Valid, "Invalid token")
	require.NoError(t, err, "Unrecognized error during token parsing %w", err)

	// token must be {"uid": "int64", login : "string", exp: "int64"}
	expectedKeys := []string{"uid", "login", "exp"}
	keys := make([]string, 0, len(claims))
	for k := range claims {
		keys = append(keys, k)
	}
	assert.ElementsMatchf(t, expectedKeys, keys, "JWT claims don't match")

	// validate token values
	// (give some gap for TTL due to uncertainty)
	const deltaSeconds = 1
	assert.Equal(t, models.RootLogin, claims["login"].(string))
	assert.Equal(t, models.RootID, int64(claims["uid"].(float64)))
	assert.InDelta(t, timestamp.Add(cfg.TokenTTL).Unix(), claims["exp"].(float64), deltaSeconds)
}

func TestFailLoginRoot(t *testing.T) {
	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	resp := e.POST("/login").
		WithJSON(models.EditorIn{
			Login: "root",
			Pass:  suite.RandomFakePassword(),
		}).
		Expect().
		Status(400)

	json := resp.JSON()

	// check returned error
	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("invalid credentials")
}

func TestLoginEditor(t *testing.T) {
	// login root
	tokenRoot, err := suite.RootLogin()
	require.NoError(t, err)

	login := gofakeit.Name()
	pass := suite.RandomFakePassword()

	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	// create new editor
	id := e.POST("/root/editors").
		WithHeader("Authorization", "Bearer "+tokenRoot).
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

	timestamp := time.Now()

	// login editor
	json := e.POST("/login").
		WithJSON(map[string]string{
			"login": login,
			"pass":  pass,
		}).Expect().
		Status(200).
		JSON()

	// response must be {"token" : "string"}
	json.Object().Keys().ContainsOnly("token")
	tokenString := json.Path("$.token").String().Raw()

	claims := jwt.MapClaims{}

	// parse token
	token, err := jwt.NewParser().ParseWithClaims(tokenString, claims, func(_ *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	// validate token
	require.Truef(t, token.Valid, "Invalid token")
	require.NoError(t, err, "Unrecognized error during token parsing %w", err)

	// token must be {"uid": "int64", login : "string", exp: "int64"}
	expectedKeys := []string{"uid", "login", "exp"}
	keys := make([]string, 0, len(claims))
	for k := range claims {
		keys = append(keys, k)
	}
	assert.ElementsMatchf(t, expectedKeys, keys, "JWT claims don't match")

	// validate token values
	// (give some gap for TTL due to uncertainty)
	const deltaSeconds = 1
	assert.Equal(t, login, claims["login"].(string))
	assert.Equal(t, id, claims["uid"].(float64))
	assert.InDelta(t, timestamp.Add(cfg.TokenTTL).Unix(), claims["exp"].(float64), deltaSeconds)
}

func TestFailLoginEditor(t *testing.T) {
	u := url.URL{
		Scheme: "http",
		Host:   cfg.Address,
	}
	e := httpexpect.Default(t, u.String())

	resp := e.POST("/login").
		WithJSON(models.EditorIn{
			Login: gofakeit.Name(),
			Pass:  suite.RandomFakePassword(),
		}).
		Expect().
		Status(400)

	json := resp.JSON()

	// check returned error
	json.Object().Keys().ContainsOnly("error")
	json.Path("$.error").String().IsEqualFold("invalid credentials")
}
