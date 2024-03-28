package controller

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

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

	app := fiber.New(fiber.Config{
		EnableSplittingOnParsers: true,
	})

	app.Use(jwtC.AuthRequired())

	// Media
	app.Get("/media", mediaCtr.searchMedia)
	app.Post("/media", mediaCtr.newMedia)
	app.Put("/media", mediaCtr.updateMedia)
	app.Get("/media/:id", mediaCtr.media)
	app.Get("/source/:id", mediaCtr.source)
	app.Delete("/media/:id", mediaCtr.deleteMedia)

	// Tags
	app.Get("/tag/types", mediaCtr.tagTypes)
	app.Get("/tag", mediaCtr.allTags)
	app.Post("/tag", mediaCtr.newTag)
	app.Put("/tag", mediaCtr.updateTag)
	app.Get("/tag/:id", mediaCtr.tag)
	app.Delete("/tag/:id", mediaCtr.deleteTag)
	app.Post("/tag/multi/:id", mediaCtr.multiTag)

	return app
}

type mediaController struct {
	srvMedia Media
	srvSrc   Source
	tmpDir   string
}

type Media interface {
	// Media
	SearchMedia(ctx context.Context, filter models.MediaFilter) ([]models.Media, error)
	NewMedia(ctx context.Context, media models.Media) (int64, error)
	UpdateMedia(ctx context.Context, media models.Media) error
	MultiTagMedia(ctx context.Context, tag models.Tag, mediaIds ...int64) error
	Media(ctx context.Context, id int64) (models.Media, error)
	DeleteMedia(ctx context.Context, id int64) error

	// Tags
	TagTypes(ctx context.Context) (models.TagTypes, error)
	AllTags(ctx context.Context) (models.TagList, error)
	SaveTag(ctx context.Context, tag models.Tag) (int64, error)
	UpdateTag(ctx context.Context, tag models.Tag) error
	Tag(ctx context.Context, id int64) (models.Tag, error)
	DeleteTag(ctx context.Context, id int64) error
}

type Source interface {
	UploadSource(ctx context.Context, path string, media *models.Media) error
	LoadSource(ctx context.Context, destDir string, media models.Media) (string, error)
	DeleteSource(ctx context.Context, media models.Media) error
}

// TODO: add support for AAC, WAV

// TODO: add PUT method for source

// searchMedia returns media list filtered and sorted
// by query criteria.
func (mediaCtr *mediaController) searchMedia(c *fiber.Ctx) error {
	var tags []string
	if s := c.Query("tags"); s != "" {
		tags = strings.Split(c.Query("tags"), ",")
	}

	filter := models.MediaFilter{
		Name:       c.Query("name"),
		Author:     c.Query("author"),
		Tags:       tags,
		MaxRespLen: c.QueryInt("res_len"),
	}

	lib, err := mediaCtr.srvMedia.SearchMedia(context.TODO(), filter)
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
		if errors.Is(err, service.ErrTagNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tag not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id": id,
	})
}

// updateMedia updates media information
func (mediaCtr *mediaController) updateMedia(c *fiber.Ctx) error {
	var request struct {
		Media models.Media `json:"media"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	if request.Media.ID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unexpected id",
		})
	}
	if request.Media.Name == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name required",
		})
	}
	if request.Media.Author == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "author required",
		})
	}

	if err := mediaCtr.srvMedia.UpdateMedia(context.TODO(), request.Media); err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "media not found",
			})
		}
		if errors.Is(err, service.ErrTagNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tag not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.SendStatus(fiber.StatusOK)
}

// multiTag add tag to media list
func (mediaCtr *mediaController) multiTag(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad id",
		})
	}

	tag, err := mediaCtr.srvMedia.Tag(context.TODO(), id)
	if err != nil {
		if errors.Is(err, service.ErrTagNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tag not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	var request struct {
		Ids []int64 `json:"ids"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	if len(request.Ids) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "no ids",
		})
	}

	if err := mediaCtr.srvMedia.MultiTagMedia(context.TODO(), tag, request.Ids...); err != nil {
		if errors.Is(err, service.ErrTagNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tag not found",
			})
		}
		return c.SendStatus(fiber.StatusBadRequest)
	}

	return c.SendStatus(fiber.StatusOK)
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

func (mediaCtr *mediaController) tagTypes(c *fiber.Ctx) error {
	tags, err := mediaCtr.srvMedia.TagTypes(context.TODO())
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"types": tags,
	})
}

// allTags returns all registered tags.
func (mediaCtr *mediaController) allTags(c *fiber.Ctx) error {
	tags, err := mediaCtr.srvMedia.AllTags(context.TODO())
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"tags": tags,
	})
}

// newTag create new tag.
func (mediaCtr *mediaController) newTag(c *fiber.Ctx) error {
	var request struct {
		Tag models.Tag `json:"tag"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	if request.Tag.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "tag name can't be empty",
		})
	}

	id, err := mediaCtr.srvMedia.SaveTag(context.TODO(), request.Tag)
	if err != nil {
		if errors.Is(err, service.ErrTagTypeNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tag type not found",
			})
		}
		if errors.Is(err, service.ErrTagExists) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tag already exists",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id": id,
	})
}

// tag returns tag by its id.
func (mediaCtr *mediaController) tag(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad id",
		})
	}

	tag, err := mediaCtr.srvMedia.Tag(context.TODO(), id)
	if err != nil {
		if errors.Is(err, service.ErrTagNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tag not found",
			})
		}
		if errors.Is(err, service.ErrTagTypeNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tag type not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"tag": tag,
	})
}

// updateTag updates tag.
func (mediaCtr *mediaController) updateTag(c *fiber.Ctx) error {
	var request struct {
		Tag models.Tag `json:"tag"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	if request.Tag.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name required",
		})
	}

	if err := mediaCtr.srvMedia.UpdateTag(context.TODO(), request.Tag); err != nil {
		if errors.Is(err, service.ErrTagTypeInvalid) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalied tag type",
			})
		}
		if errors.Is(err, service.ErrTagNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tag not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.SendStatus(fiber.StatusOK)
}

// deleteTag deletes tag by its id
func (mediaCtr *mediaController) deleteTag(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad id",
		})
	}

	if err := mediaCtr.srvMedia.DeleteTag(context.TODO(), id); err != nil {
		if errors.Is(err, service.ErrTagNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "tag not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.SendStatus(fiber.StatusOK)
}
