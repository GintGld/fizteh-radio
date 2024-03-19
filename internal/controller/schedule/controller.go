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
	dj     DJ
}

type Schedule interface {
	ScheduleCut(ctx context.Context, start time.Time, stop time.Time) ([]models.Segment, error)
	NewSegment(ctx context.Context, segment models.Segment) (int64, error)
	Segment(ctx context.Context, id int64) (models.Segment, error)
	DeleteSegment(ctx context.Context, id int64) error
	ClearSchedule(ctx context.Context, from time.Time) error
}

type DJ interface {
	SetConfig(conf models.AutoDJConfig)
	Config() models.AutoDJConfig
	Run(ctx context.Context) error
	IsPlaying() bool
	Stop()
}

func New(
	schSrv Schedule,
	dj DJ,
	jwtC *jwtController.JWT,
) *fiber.App {
	schCtr := scheduleController{
		schSrv: schSrv,
		dj:     dj,
	}

	app := fiber.New()

	app.Use(jwtC.AuthRequired())

	app.Get("/", schCtr.scheduleCut)
	app.Post("/", schCtr.newSegment)
	app.Get("/:id", schCtr.segment)
	app.Delete("/:id", schCtr.deleteSegment)
	app.Delete("/", schCtr.clearSchedule)

	app.Get("/dj/config", schCtr.getDJConfig)
	app.Post("/dj/config", schCtr.setDJConfig)
	app.Get("/dj/start", schCtr.startDJ)
	app.Get("dj/status", schCtr.isPlaying)
	app.Get("/dj/stop", schCtr.stopDJ)

	return app
}

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

	id, err := schCtr.schSrv.NewSegment(context.TODO(), form.Segment)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "media not found",
			})
		}
		if errors.Is(err, service.ErrCutOutOfBounds) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "cut out of bounds",
			})
		}
		if errors.Is(err, service.ErrBeginAfterStop) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "begin after stop",
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

// clearSchedule clear schedule from given timestamp
func (schCtr *scheduleController) clearSchedule(c *fiber.Ctx) error {
	fromInt := c.QueryInt("from", -1)
	if fromInt == -1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": `"from" not defined`,
		})
	}

	from := time.Unix(int64(fromInt), 0)

	if err := schCtr.schSrv.ClearSchedule(context.TODO(), from); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.SendStatus(fiber.StatusOK)
}

// getDJConfig returns autodj config.
func (schCtr *scheduleController) getDJConfig(c *fiber.Ctx) error {
	conf := schCtr.dj.Config()
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"config": conf,
	})
}

// setDJConfig updates autodj config.
func (schCtr *scheduleController) setDJConfig(c *fiber.Ctx) error {
	var request struct {
		Conf models.AutoDJConfig `json:"config"`
	}

	if err := c.BodyParser(&request); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	schCtr.dj.SetConfig(request.Conf)

	return c.SendStatus(fiber.StatusOK)
}

// startDJ start autodj.
func (schCtr *scheduleController) startDJ(c *fiber.Ctx) error {
	go schCtr.dj.Run(context.TODO())

	return c.SendStatus(fiber.StatusOK)
}

// isPlaying returns autodj status.
func (schCtr *scheduleController) isPlaying(c *fiber.Ctx) error {
	isPlaying := schCtr.dj.IsPlaying()
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"playing": isPlaying,
	})
}

// stopDJ stops autodj.
func (schCtr *scheduleController) stopDJ(c *fiber.Ctx) error {
	go schCtr.dj.Stop()

	return c.SendStatus(fiber.StatusOK)
}
