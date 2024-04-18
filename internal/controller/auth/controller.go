package auth

import (
	"context"
	"errors"
	"time"

	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
	"github.com/gofiber/fiber/v2"
)

// TODO: make refresh toleks for better user experience

// New returns an fiber.App that will
// authorize editors (including root)
// and return JWT
func New(
	timeout time.Duration,
	a Auth,
) *fiber.App {
	authCtr := authController{
		timeout: timeout,
		srv:     a,
	}

	app := fiber.New()

	app.Post("/", authCtr.login)

	return app
}

type authController struct {
	timeout time.Duration
	srv     Auth
}

type Auth interface {
	Login(ctx context.Context, login string, password string) (string, error)
}

// login
func (authCtr *authController) login(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), authCtr.timeout)
	defer cancel()

	form := new(models.EditorIn)

	if err := c.BodyParser(form); err != nil {
		return fiber.ErrBadRequest
	}

	if form.Login == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "login required",
		})
	}

	if form.Pass == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "password required",
		})
	}

	token, err := authCtr.srv.Login(ctx, form.Login, form.Pass)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid credentials",
			})
		}

		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"token": token,
	})
}
