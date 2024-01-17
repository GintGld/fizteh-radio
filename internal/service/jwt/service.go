package jwtService

import (
	"fmt"
	"time"

	"github.com/GintGld/fizteh-radio/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

type JWT struct {
	secret []byte
}

func New(secret []byte) *JWT {
	return &JWT{
		secret: secret,
	}
}

func (jwtStruct *JWT) NewToken(editor models.Editor, duration time.Duration) (string, error) {
	const op = "JWT.NewToken"

	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["uid"] = editor.ID
	claims["login"] = editor.Login
	claims["exp"] = time.Now().Add(duration).Unix()

	tokenString, err := token.SignedString(jwtStruct.secret)
	if err != nil {
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return tokenString, nil
}
