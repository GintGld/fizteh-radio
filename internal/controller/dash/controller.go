package controller

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	jwtController "github.com/GintGld/fizteh-radio/internal/controller/jwt"
	"github.com/GintGld/fizteh-radio/internal/models"
)

// TODO: refactor root start/stop

func New(
	manifestPath string,
	contentDir string,
	jwtCtr *jwtController.JWT,
	dash DashService,
) *fiber.App {
	app := fiber.New()

	app.Get("/start", rootAccess, func(c *fiber.Ctx) error {
		go dash.Run(context.TODO())
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/stop", rootAccess, func(c *fiber.Ctx) error {
		go dash.Stop()
		return c.SendStatus(fiber.StatusOK)
	})

	return app
}

type DashService interface {
	Run(context.Context) error
	Stop()
}

// rootAccess check if the logged user is root,
// but doesn't check validity, because only jwtWare
// has access to the secret
func rootAccess(c *fiber.Ctx) error {
	auth := c.Get(fiber.HeaderAuthorization)

	jwtSplitted := strings.Split(auth, " ")
	if len(jwtSplitted) != 2 || jwtSplitted[0] != "Bearer" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid JWT",
		})
	}

	token := jwtSplitted[1]
	claims := jwt.MapClaims{}
	_, _, err := jwt.NewParser().ParseUnverified(token, claims)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid JWT",
		})
	}

	if claims["login"] != models.RootLogin {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "available for root only",
		})
	}

	return c.Next()
}
