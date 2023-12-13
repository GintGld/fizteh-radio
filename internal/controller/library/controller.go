package controller

import (
	"context"
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	jwtController "github.com/GintGld/fizteh-radio/internal/controller/jwt"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
)

// TODO: GET "/media" or "/library"

func New(libSrv Library, jwtC *jwtController.JWT) *fiber.App {
	libCtr := libraryController{
		srv: libSrv,
	}

	app := fiber.New()

	app.Use(jwtC.AuthRequired())

	app.Post("/media", libCtr.newMedia)
	app.Get("/media/:id", libCtr.media)
	app.Delete("/media/:id", libCtr.deleteMedia)

	return app
}

type libraryController struct {
	srv Library
}

type Library interface {
	NewMedia(ctx context.Context, newMedia models.Media) (int64, error)
	Media(ctx context.Context, id int64) (models.Media, error)
	DeleteMedia(ctx context.Context, id int64) error
}

// newMedia creates new media
func (libCtr *libraryController) newMedia(c *fiber.Ctx) error {
	type request struct {
		Media models.Media `json:"media"`
	}

	req := new(request)

	if err := c.BodyParser(req); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	if req.Media.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name required",
		})
	}

	if req.Media.Author == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "author required",
		})
	}

	id, err := libCtr.srv.NewMedia(context.TODO(), req.Media)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id": id,
	})
}

// media return json with media by id
func (libCtr *libraryController) media(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad id",
		})
	}

	media, err := libCtr.srv.Media(context.TODO(), id)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "media not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"media": media,
	})
}

// deleteEditor deletes editor
func (libCtr *libraryController) deleteMedia(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad id",
		})
	}

	err = libCtr.srv.DeleteMedia(context.TODO(), id)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "media not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.SendStatus(fiber.StatusOK)
}
