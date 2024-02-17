package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/GintGld/fizteh-radio/internal/models"
)

type Storage struct {
	db *sql.DB

	tagCache tagCacheStruct
	tagTypes []models.TagType
}

// Cache available tag list since it does not update frequently
// and not going to be large.
type tagCacheStruct struct {
	TagList models.TagList
	Mutex   sync.Mutex
}

func New(storagePath string) (*Storage, error) {
	const op = "storage.sqlite.New"

	db, err := sql.Open("sqlite3", storagePath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	st := &Storage{
		db: db,
		tagCache: tagCacheStruct{
			TagList: make(models.TagList, 0),
		},
	}

	if err := st.recoverCaches(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return st, nil
}

func (s *Storage) recoverCaches() error {
	if err := s.updateTagTypes(context.Background()); err != nil {
		return err
	}
	if err := s.updateTagList(context.Background()); err != nil {
		return err
	}
	return nil
}

func (s *Storage) Stop() error {
	return s.db.Close()
}
