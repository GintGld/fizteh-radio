package controller

import "github.com/gofiber/fiber/v2"

func New(
	manifestPath string,
	contentDir string,
) *fiber.App {
	app := fiber.New()

	app.Get("/mpd", func(c *fiber.Ctx) error {
		return c.SendFile(manifestPath)
	})
	app.Get("/:id/:file", func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return c.SendStatus(fiber.StatusNotFound)
		}

		file := c.Params("file")
		if file == "" {
			return c.SendStatus(fiber.StatusNotFound)
		}

		return c.SendFile(contentDir + "/" + id + "/" + file)
	})

	return app
}
