package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	"github.com/GintGld/fizteh-radio/internal/models"
)

type Content struct {
	log         *slog.Logger
	path        string
	chunkLenght time.Duration
	media       Media
	source      Source
}

func New(
	log *slog.Logger,
	path string,
	chunkLenght time.Duration,
	media Media,
	source Source,
) *Content {
	return &Content{
		log:         log,
		path:        path,
		chunkLenght: chunkLenght,
		media:       media,
		source:      source,
	}
}

type Media interface {
	Media(ctx context.Context, id int64) (models.Media, error)
}

type Source interface {
	LoadSource(ctx context.Context, destDir string, media models.Media) (string, error)
}

func (c *Content) Generate(ctx context.Context, s *models.Segment) error {
	const op = "Content.Generate"

	log := c.log.With(
		slog.String("op", op),
	)

	if err := c.generateDASHFiles(ctx, s); err != nil {
		log.Error("failed to generate chunks", sl.Err(err))
	}

	return nil
}

func (c *Content) CleanUp() {
	const op = "Content.CleanUp"

	log := c.log.With(
		slog.String("op", op),
	)

	if err := c.deleteAll(); err != nil {
		log.Error("failed to delete files", sl.Err(err))
	}

	log.Debug("deleted all files")
}
