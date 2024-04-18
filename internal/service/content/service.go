package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
)

type Content struct {
	log         *slog.Logger
	path        string
	chunkLength time.Duration
	media       Media
	source      Source
}

func New(
	log *slog.Logger,
	path string,
	chunkLength time.Duration,
	media Media,
	source Source,
) *Content {
	if err := os.MkdirAll(path+"/.cache", 0777); err != nil {
		log.Error(
			"failed to generate cache dir",
			slog.String("path", path+"/.cache"),
			sl.Err(err),
		)
	}
	log.Debug("created cache dir")

	return &Content{
		log:         log,
		path:        path,
		chunkLength: chunkLength,
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

func (c *Content) Init() error {
	const op = "Content.Init"

	log := c.log.With(
		slog.String("op", op),
	)

	if err := os.MkdirAll(c.path+"/.cache", 0777); err != nil {
		log.Error(
			"failed to create cache dir",
			slog.String("dir", c.path),
			sl.Err(err),
		)
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

// Generate generates dash content
// by given segment
func (c *Content) Generate(ctx context.Context, s models.Segment) error {
	const op = "Content.Generate"

	log := c.log.With(
		slog.String("op", op),
	)

	if err := c.generateDASHFiles(ctx, s); err != nil {
		if errors.Is(err, service.ErrTimeout) {
			log.Error("generateDASHFiles timeout exceeded")
			return service.ErrTimeout
		}
		log.Error("failed to generate chunks", sl.Err(err))
	}

	return nil
}

// ClearCache clears cache. Must be called
// regularly if dash works not in cyclic regime
func (c *Content) ClearCache() error {
	const op = "Content.ClearCache"

	log := c.log.With(
		slog.String("op", op),
	)

	if err := c.deleteCache(); err != nil {
		log.Error("failed to clear cache", sl.Err(err))
	}

	return nil
}

// CleanUp deletes all files create by
// Content struct. Must be called after
// stopping dash
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
