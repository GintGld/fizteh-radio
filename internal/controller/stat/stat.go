package stat

import (
	"context"
	"strconv"
	"time"

	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/gofiber/fiber/v2"
)

func New(
	timeout time.Duration,
	stat Stat,
) *fiber.App {
	statCtr := &statController{
		timeout: timeout,
		stat:    stat,
	}

	app := fiber.New()

	app.Get("/listener", statCtr.newListener)
	app.Get("/listener/ping", statCtr.pingListener)
	app.Get("/listeners/number", statCtr.listenersNumber)
	app.Get("/listeners", statCtr.listeners)

	return app
}

type statController struct {
	timeout time.Duration
	stat    Stat
}

type Stat interface {
	RegisterListener() int64
	PingListener(id int64)
	ListenersNumber() int
	Listeners(ctx context.Context, start, stop time.Time) ([]models.Listener, error)
}

func (statCtr *statController) newListener(c *fiber.Ctx) error {
	id := statCtr.stat.RegisterListener()

	c.Cookie(&fiber.Cookie{
		Name:        "session",
		Value:       strconv.FormatInt(id, 10),
		SessionOnly: true,
	})

	return c.SendStatus(fiber.StatusOK)
}

func (statCtr *statController) pingListener(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Cookies("session"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad cookie",
		})
	}

	statCtr.stat.PingListener(id)

	return c.SendStatus(fiber.StatusOK)
}

func (statCtr *statController) listenersNumber(c *fiber.Ctx) error {
	n := statCtr.stat.ListenersNumber()

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"listeners": n,
	})
}

func (statCtr *statController) listeners(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), statCtr.timeout)
	defer cancel()

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

	res, err := statCtr.stat.Listeners(ctx, start, stop)
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"listeners": res,
	})
}
