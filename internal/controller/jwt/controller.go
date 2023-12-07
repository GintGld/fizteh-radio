package jwtController

import (
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
)

type JWT struct {
	secret []byte
}

func New(secret []byte) *JWT {
	return &JWT{secret: secret}
}

func (jwtController *JWT) AuthRequired() func(*fiber.Ctx) error {
	return jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{Key: jwtController.secret},
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "authentication error",
			})
		},
	})
}
