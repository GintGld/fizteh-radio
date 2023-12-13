package root

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
	"github.com/GintGld/fizteh-radio/internal/storage"
	"github.com/gofiber/fiber/v2/log"
	"golang.org/x/crypto/bcrypt"
)

// TODO: move here some functions from auth that will
// be accessible only from root interface

type Root struct {
	log        *slog.Logger
	usrManager EditorManager
}

type EditorManager interface {
	SaveEditor(
		ctx context.Context,
		login string,
		passHash []byte,
	) (int64, error)
	Editor(ctx context.Context, id int64) (models.Editor, error)
	DeleteEditor(ctx context.Context, id int64) error
	AllEditors(ctx context.Context) ([]models.Editor, error)
}

func New(
	log *slog.Logger,
	usrManager EditorManager,
) *Root {
	return &Root{
		log:        log,
		usrManager: usrManager,
	}
}

// RegisterNewEditor registers new editor in the system and returns editor ID.
//
// If editor with given name already exists, returns error.
func (r *Root) RegisterNewEditor(ctx context.Context, form models.EditorIn) (int64, error) {
	const op = "Root.RegisterNewEditor"

	r.log.Info("registering editor")

	passHash, err := bcrypt.GenerateFromPassword([]byte(form.Pass), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to generate password hash", sl.Err(err))

		return models.ErrEditorID, fmt.Errorf("%s: %w", op, err)
	}

	id, err := r.usrManager.SaveEditor(ctx, form.Login, passHash)
	if err != nil {
		if errors.Is(err, storage.ErrEditorExists) {
			r.log.Warn("editor exists", slog.String("login", form.Login))
			return models.ErrEditorID, fmt.Errorf("%s: %w", op, service.ErrEditorExists)
		}
		r.log.Error("failed to save editor", sl.Err(err))

		return models.ErrEditorID, fmt.Errorf("%s: %w", op, err)
	}

	r.log.Info("registered editor", slog.String("login", form.Login), slog.Int64("id", id))

	return id, nil
}

// DeleteEditor deletes editor
func (r *Root) DeleteEditor(ctx context.Context, id int64) error {
	const op = "Root.DeleteEditor"

	r.log.Info("deleting editor", slog.Int64("id", id))

	if err := r.usrManager.DeleteEditor(ctx, id); err != nil {
		if errors.Is(err, storage.ErrEditorNotFound) {
			r.log.Warn("editor not found", slog.Int64("id", id))
			return fmt.Errorf("%s: %w", op, service.ErrEditorNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// Editor returns editor model by given id.
//
// If editor with given id does not exist, returns error
func (r *Root) Editor(ctx context.Context, id int64) (models.EditorOut, error) {
	const op = "Root.Editor"

	r.log.Info("getting editor", slog.Int64("id", id))

	editor, err := r.usrManager.Editor(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrEditorNotFound) {
			r.log.Warn("editor not found", slog.Int64("id", id))
			return models.EditorOut{}, fmt.Errorf("%s: %w", op, service.ErrEditorNotFound)
		}
		r.log.Error("failed to get editor", sl.Err(err))
		return models.EditorOut{}, fmt.Errorf("%s: %w", op, err)
	}

	r.log.Info("found editor", slog.Int64("id", id))
	return models.EditorOut{
		ID:    editor.ID,
		Login: editor.Login,
	}, nil
}

// AllEditors returns all editors.
//
// If there is no any editor, returns empty slice
func (r *Root) AllEditors(ctx context.Context) ([]models.EditorOut, error) {
	const op = "Root.AllEditors"

	r.log.Info("getting all editors")

	editors, err := r.usrManager.AllEditors(ctx)
	if err != nil {
		r.log.Error("failed to get editors", sl.Err(err))
		return []models.EditorOut{}, fmt.Errorf("%s: %w", op, err)
	}

	editorsOut := make([]models.EditorOut, 0, len(editors))

	for _, ed := range editors {
		editorsOut = append(editorsOut, models.EditorOut{
			ID:    ed.ID,
			Login: ed.Login,
		})
	}

	return editorsOut, nil
}
