package root

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	jwtController "github.com/GintGld/fizteh-radio/internal/controller/jwt"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
)

// TODO: move access check to special controller
// TODO: /editor PUT

// New returns fiber app that will
// handle requests special for root
func New(rootSrv Root, jwtC *jwtController.JWT) *fiber.App {
	rootCtr := rootController{
		srv: rootSrv,
	}

	app := fiber.New()

	// token validity -> root access -> handling request
	app.Use(jwtC.AuthRequired(), rootCtr.rootAccess)

	app.Get("/editors", rootCtr.allEditors)
	app.Post("/editors", rootCtr.newEditor)
	app.Get("/editor/:id", rootCtr.editor)
	app.Delete("/editor/:id", rootCtr.deleteEditor)

	return app
}

type rootController struct {
	srv Root
}

type Root interface {
	RegisterNewEditor(ctx context.Context, form models.EditorIn) (int64, error)
	AllEditors(ctx context.Context) ([]models.EditorOut, error)
	Editor(ctx context.Context, id int64) (models.EditorOut, error)
	DeleteEditor(ctx context.Context, id int64) error
}

// rootAccess check if the logged user is root,
// but doesn't check validity, because only jwtWare
// has access to the secret
func (rootCtr *rootController) rootAccess(c *fiber.Ctx) error {
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

// allEditors return json with all editors
func (rootCtr *rootController) allEditors(c *fiber.Ctx) error {
	editors, err := rootCtr.srv.AllEditors(context.TODO())
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"editors": editors,
	})
}

// editor return json with editor by id
func (rootCtr *rootController) editor(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad id",
		})
	}

	editor, err := rootCtr.srv.Editor(context.TODO(), id)
	if err != nil {
		if errors.Is(err, service.ErrEditorNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "editor not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"editor": models.EditorOut{
			ID:    editor.ID,
			Login: editor.Login,
		},
	})
}

// newEditor creates new editor
func (rootCtr *rootController) newEditor(c *fiber.Ctx) error {
	type request struct {
		User models.EditorIn `json:"editor"`
	}

	form := new(request)

	if err := c.BodyParser(form); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	id, err := rootCtr.srv.RegisterNewEditor(context.TODO(), form.User)
	if err != nil {
		if errors.Is(err, service.ErrEditorExists) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "editor exists",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id": id,
	})
}

// deleteEditor deletes editor
func (rootCtr *rootController) deleteEditor(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bad id",
		})
	}

	err = rootCtr.srv.DeleteEditor(context.TODO(), id)
	if err != nil {
		if errors.Is(err, service.ErrEditorNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "editor not found",
			})
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.SendStatus(fiber.StatusOK)
}
