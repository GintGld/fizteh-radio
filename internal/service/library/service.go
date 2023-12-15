package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
	"github.com/GintGld/fizteh-radio/internal/storage"
)

type Library struct {
	log        *slog.Logger
	libStorage LibraryStorage
}

type LibraryStorage interface {
	SaveMedia(ctx context.Context, newMedia models.Media) (int64, error)
	Media(ctx context.Context, id int64) (models.Media, error)
	DeleteMedia(ctx context.Context, id int64) error
}

func New(
	log *slog.Logger,
	libStorage LibraryStorage,
) *Library {
	return &Library{
		log:        log,
		libStorage: libStorage,
	}
}

// TODO: in logging save editor name (put on context)

// NewMedia registers new editor in the system and returns media ID.
func (l *Library) NewMedia(ctx context.Context, newMedia models.Media) (int64, error) {
	const op = "Library.NewMedia"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("registering new media")

	id, err := l.libStorage.SaveMedia(ctx, newMedia)
	if err != nil {
		log.Error("failed to save media", sl.Err(err))
		return models.ErrEditorID, fmt.Errorf("%s: %w", op, err)
	}

	log.Info(
		"registered media",
		slog.Int64("id", id),
		slog.String("name", *newMedia.Name),
		slog.String("author", *newMedia.Author),
		slog.Int64("sourceID", *newMedia.SourceID),
	)

	return id, nil
}

// Media returns media model by given id.
//
// If media with given id does not exist, returns error.
func (l *Library) Media(ctx context.Context, id int64) (models.Media, error) {
	const op = "Library.Media"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("getting media")

	media, err := l.libStorage.Media(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrMediaNotFound) {
			log.Warn("media not found", slog.Int64("id", id))
			return models.Media{}, service.ErrMediaNotFound
		}
		log.Error("failed to get media", slog.Int64("id", id), sl.Err(err))
		return models.Media{}, err
	}

	log.Info("found media", slog.Int64("id", id))

	return media, nil
}

// DeleteMedia deletes media.
//
// If media with given id does not exist, returns error.
func (l *Library) DeleteMedia(ctx context.Context, id int64) error {
	const op = "Library.DeleteEditor"

	log := l.log.With(
		slog.String("op", op),
		slog.String("editorname", models.RootLogin),
	)

	log.Info("deleting media", slog.Int64("id", id))

	if err := l.libStorage.DeleteMedia(ctx, id); err != nil {
		if errors.Is(err, storage.ErrMediaNotFound) {
			log.Warn("media not found", slog.Int64("id", id))
			return fmt.Errorf("%s: %w", op, service.ErrMediaNotFound)
		}
		log.Error("failed to delete media", slog.Int64("id", id))
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
