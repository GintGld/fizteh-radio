package auth

import (
	"context"
	"errors"

	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
	"github.com/gofiber/fiber/v2"
)

// TODO: make refresh toleks for better user experience

// New returns an fiber.App that will
// authorize editors (including root)
// and return JWT
func New(a Auth) *fiber.App {
	authCtr := authController{
		srv: a,
	}

	app := fiber.New()

	app.Post("/", authCtr.login)

	return app
}

type authController struct {
	srv Auth
}

type Auth interface {
	Login(ctx context.Context, login string, password string) (string, error)
}

// login
func (authCtr *authController) login(c *fiber.Ctx) error {
	form := new(models.EditorIn)

	if err := c.BodyParser(form); err != nil {
		return fiber.ErrBadRequest
	}

	if form.Login == "" {
		c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "login required",
		})
	}

	if form.Pass == "" {
		c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "password required",
		})
	}

	token, err := authCtr.srv.Login(context.TODO(), form.Login, form.Pass)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid credentials",
			})
		}

		c.Status(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"token": token,
	})
}
