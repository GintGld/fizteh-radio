package tests

import (
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gavv/httpexpect/v2"

	"github.com/GintGld/fizteh-radio/internal/config"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/tests/suite"
)

// Actual environment
var (
	_        = godotenv.Load("../.env")
	cfg      = config.MustLoadPath(os.Getenv("CONFIG_PATH"))
	rootPass = os.Getenv("ROOT_PASS")
	secret   = os.Getenv("SECRET")
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
