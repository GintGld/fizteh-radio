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
	"golang.org/x/crypto/bcrypt"
)

type Root struct {
	log        *slog.Logger
	edtStorage EditorStorage
}

type EditorStorage interface {
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
	edtStorage EditorStorage,
) *Root {
	return &Root{
		log:        log,
		edtStorage: edtStorage,
	}
}

// RegisterNewEditor registers new editor in the system and returns editor ID.
//
// If editor with given name already exists, returns error.
func (r *Root) RegisterNewEditor(ctx context.Context, form models.EditorIn) (int64, error) {
	const op = "Root.RegisterNewEditor"

	log := r.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("registering editor")

	passHash, err := bcrypt.GenerateFromPassword([]byte(form.Pass), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to generate password hash", sl.Err(err))

		return models.ErrEditorID, fmt.Errorf("%s: %w", op, err)
	}

	id, err := r.edtStorage.SaveEditor(ctx, form.Login, passHash)
	if err != nil {
		if errors.Is(err, storage.ErrEditorExists) {
			log.Warn("editor exists", slog.String("login", form.Login))
			return models.ErrEditorID, fmt.Errorf("%s: %w", op, service.ErrEditorExists)
		}
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("edtStorage.SaveEditor timeout exceeded")
			return models.ErrEditorID, service.ErrTimeout
		}
		log.Error("failed to save editor", sl.Err(err))

		return models.ErrEditorID, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("registered editor", slog.String("login", form.Login), slog.Int64("id", id))

	return id, nil
}

// DeleteEditor deletes editor.
//
// If editor with given name already exists, returns error.
func (r *Root) DeleteEditor(ctx context.Context, id int64) error {
	const op = "Root.DeleteEditor"

	log := r.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("deleting editor", slog.Int64("id", id))

	if err := r.edtStorage.DeleteEditor(ctx, id); err != nil {
		if errors.Is(err, storage.ErrEditorNotFound) {
			log.Warn("editor not found", slog.Int64("id", id))
			return fmt.Errorf("%s: %w", op, service.ErrEditorNotFound)
		}
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("edtStorage.DeleteEditor timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to delete editor", slog.Int64("id", id))
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// Editor returns editor model by given id.
//
// If editor with given id does not exist, returns error.
func (r *Root) Editor(ctx context.Context, id int64) (models.EditorOut, error) {
	const op = "Root.Editor"

	log := r.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("getting editor", slog.Int64("id", id))

	editor, err := r.edtStorage.Editor(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrEditorNotFound) {
			r.log.Warn("editor not found", slog.Int64("id", id))
			return models.EditorOut{}, fmt.Errorf("%s: %w", op, service.ErrEditorNotFound)
		}
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("edtStorage.Editor timeout exceeded")
			return models.EditorOut{}, service.ErrTimeout
		}
		log.Error("failed to get editor", sl.Err(err))
		return models.EditorOut{}, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("found editor", slog.Int64("id", id))
	return models.EditorOut{
		ID:    editor.ID,
		Login: editor.Login,
	}, nil
}

// AllEditors returns all editors.
//
// If there is no any editor, returns empty slice.
func (r *Root) AllEditors(ctx context.Context) ([]models.EditorOut, error) {
	const op = "Root.AllEditors"

	log := r.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("getting all editors")

	editors, err := r.edtStorage.AllEditors(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("edtStorage.AllEditors timeout exceeded")
			return []models.EditorOut{}, service.ErrTimeout
		}
		log.Error("failed to get editors", sl.Err(err))
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
