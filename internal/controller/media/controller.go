package controller

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gofiber/fiber/v2"

	jwtController "github.com/GintGld/fizteh-radio/internal/controller/jwt"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
)

// TODO: check if controller really delete tmp files

func New(
	srvMedia Media,
	srvSrc Source,
	jwtC *jwtController.JWT,
	tmpDir string,
) *fiber.App {
	mediaCtr := mediaController{
		srvMedia: srvMedia,
		srvSrc:   srvSrc,
		tmpDir:   tmpDir,
	}

	app := fiber.New()

	app.Use(jwtC.AuthRequired())

	app.Get("/media", mediaCtr.allMedia)
	app.Post("/media", mediaCtr.newMedia)
	app.Get("/media/:id", mediaCtr.media)
	app.Get("/source/:id", mediaCtr.source)
	app.Delete("/media/:id", mediaCtr.deleteMedia)

	return app
}

type mediaController struct {
	srvMedia Media
	srvSrc   Source
	tmpDir   string
}

type Media interface {
	AllMedia(ctx context.Context) ([]models.Media, error)
	NewMedia(ctx context.Context, newMedia models.Media) (int64, error)
	Media(ctx context.Context, id int64) (models.Media, error)
	DeleteMedia(ctx context.Context, id int64) error
}

type Source interface {
	UploadSource(ctx context.Context, path string, media *models.Media) error
	LoadSource(ctx context.Context, destDir string, media models.Media) (string, error)
	DeleteSource(ctx context.Context, media models.Media) error
}

// TODO: add support for AAC, WAV

// allMedia returns all media
func (mediaCtr *mediaController) allMedia(c *fiber.Ctx) error {
	lib, err := mediaCtr.srvMedia.AllMedia(context.TODO())
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"library": lib,
	})
}

// newMedia saves sended file and creates media
func (mediaCtr *mediaController) newMedia(c *fiber.Ctx) error {
	payload := c.FormValue("media")
	if payload == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "no media information",
		})
	}

	var media models.Media
	if err := json.Unmarshal([]byte(payload), &media); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid media information",
		})
	}

	if media.Name == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name required",
		})
	}
	if media.Author == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "author required",
		})
	}
	if media.ID != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unexpected id",
		})
	}
	if media.Duration != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unexpected duration",
		})
	}

	file, err := c.FormFile("source")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid file",
		})
	}

	fileType := file.Header.Get("Content-Type")
	if fileType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "content-type not found",
		})
	}

	// recognize MIME-type (allow only auido.mpeg == .mp3)
	if fileType != "application/octet-stream" && fileType != "audio/mpeg" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unsupported mime-type",
		})
	} else if fileType == "application/octet-stream" {
		reader, err := file.Open()
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		mimeType, err := mimetype.DetectReader(reader)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		if !mimeType.Is("audio/mpeg") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "unsupported mime-type",
			})
		}
	}

	tmpFile, err := os.CreateTemp(mediaCtr.tmpDir, "*.mp3")
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	tmpFileName := tmpFile.Name()
	defer tmpFile.Close()

	if err := c.SaveFile(file, tmpFileName); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	defer os.Remove(tmpFileName)

	// TODO: move this code to goroutine

	// TODO: enhance error statuses
	if err := mediaCtr.srvSrc.UploadSource(context.TODO(), tmpFileName, &media); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	id, err := mediaCtr.srvMedia.NewMedia(context.TODO(), media)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id": id,
	})
}

// media return json with media by id
func (mediaCtr *mediaController) media(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad id",
		})
	}

	media, err := mediaCtr.srvMedia.Media(context.TODO(), id)
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

// source returns source file
// corresponding to media
func (mediaCtr *mediaController) source(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad id",
		})
	}

	media, err := mediaCtr.srvMedia.Media(context.TODO(), id)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "media not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	// TODO: enhance error statuses
	sourceFile, err := mediaCtr.srvSrc.LoadSource(context.TODO(), mediaCtr.tmpDir, media)
	if err != nil {
		c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).SendFile(sourceFile)
}

// deleteEditor deletes editor
func (mediaCtr *mediaController) deleteMedia(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad id",
		})
	}

	// TODO: enhance error statuses
	media, err := mediaCtr.srvMedia.Media(context.TODO(), id)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "media not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	// TODO: enhance error statuses
	if err = mediaCtr.srvSrc.DeleteSource(context.TODO(), media); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	if err = mediaCtr.srvMedia.DeleteMedia(context.TODO(), id); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.SendStatus(fiber.StatusOK)
}
