package controller

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	jwtController "github.com/GintGld/fizteh-radio/internal/controller/jwt"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
)

type scheduleController struct {
	schSrv Schedule
}

type Schedule interface {
	ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error)
	NewSegment(ctx context.Context, segment models.Segment) (int64, error)
	Segment(ctx context.Context, id int64) (models.Segment, error)
	DeleteSegment(ctx context.Context, id int64) error
}

func New(
	schSrv Schedule,
	jwtC *jwtController.JWT,
) *fiber.App {
	schCtr := scheduleController{
		schSrv: schSrv,
	}

	app := fiber.New()

	app.Use(jwtC.AuthRequired())

	app.Get("/", schCtr.scheduleCut)
	app.Post("/", schCtr.newSegment)
	app.Get("/:id", schCtr.segment)
	app.Delete("/:id", schCtr.deleteSegment)

	return app
}

// TODO: realize this methods

// TODO: move media validation from service to controller

// scheduleCut returns segments intersecting given interval
// if
func (schCtr *scheduleController) scheduleCut(c *fiber.Ctx) error {
	// Default values for cut
	start := time.Unix(0, 0)
	stop := time.Date(2100, 1, 1, 0, 0, 0, 0, time.Local)

	if unix := c.QueryInt("start"); unix != 0 {
		start = time.Unix(int64(unix), 0)
	}
	if unix := c.QueryInt("stop"); unix != 0 {
		stop = time.Unix(int64(unix), 0)
	}

	if start.After(stop) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid start value",
		})
	}

	segments, err := schCtr.schSrv.ScheduleCut(context.TODO(), start, stop)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"segments": segments,
	})
}

// newSegment registers new segment
func (schCtr *scheduleController) newSegment(c *fiber.Ctx) error {
	type request struct {
		Segment models.Segment `json:"segment"`
	}

	form := new(request)

	if err := c.BodyParser(form); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	if form.Segment.MediaID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "media not defined",
		})
	}
	if form.Segment.Start == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "start not defined",
		})
	}
	if form.Segment.BeginCut == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "beginCut not defined",
		})
	}
	if form.Segment.StopCut == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "stopCut not defined",
		})
	}
	if *form.Segment.BeginCut >= *form.Segment.StopCut {
		c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "beginCut is later that stopCut",
		})
	}

	id, err := schCtr.schSrv.NewSegment(context.TODO(), form.Segment)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "media not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id": id,
	})
}

// segment returns segment by id
func (schCtr *scheduleController) segment(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad id",
		})
	}

	segment, err := schCtr.schSrv.Segment(context.TODO(), id)
	if err != nil {
		if errors.Is(err, service.ErrSegmentNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "segment not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"segment": segment,
	})
}

// deleteSegment deletes segment by id
func (schCtr *scheduleController) deleteSegment(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad id",
		})
	}

	if err := schCtr.schSrv.DeleteSegment(context.TODO(), id); err != nil {
		if errors.Is(err, service.ErrSegmentNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "segment not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.SendStatus(fiber.StatusOK)
}
