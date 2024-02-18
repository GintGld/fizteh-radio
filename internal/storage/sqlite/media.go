package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mattn/go-sqlite3"

	ptr "github.com/GintGld/fizteh-radio/internal/lib/utils/pointers"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/storage"
)

// AllMedia return all media in library.
func (s *Storage) AllMedia(ctx context.Context) ([]models.Media, error) {
	const op = "storage.sqlite.AllMedia"

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return []models.Media{}, fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback()

	libraryTags, err := s.allMediaSubTags(tx, ctx)
	if err != nil {
		return []models.Media{}, fmt.Errorf("%s: %w", op, err)
	}

	lib, err := s.allMediaSubBasicInfo(tx, ctx, libraryTags)
	if err != nil {
		return []models.Media{}, fmt.Errorf("%s: %w", op, err)
	}

	return lib, nil
}

// allMediaSubTags return map containing
// tag names for every media id.
func (s *Storage) allMediaSubTags(tx statementBuilder, ctx context.Context) (map[int64]models.TagList, error) {
	const op = "allMediaSubTags"

	var id int64
	libraryTags := make(map[int64]models.TagList, 0)
	stmt, err := tx.PrepareContext(ctx, `
		SELECT lt.media_id, t.name, tt.id, tt.name FROM libraryTag AS lt
		JOIN tag AS t ON t.id = lt.tag_id
		JOIN tagType AS tt ON t.type_id = tt.id
	`)
	if err != nil {
		return map[int64]models.TagList{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return map[int64]models.TagList{}, fmt.Errorf("%s: %w", op, err)
	}

	var tag models.Tag
	for rows.Next() {
		if err = rows.Scan(&id, &tag.Name, &tag.Type.ID, &tag.Type.Name); err != nil {
			return map[int64]models.TagList{}, fmt.Errorf("%s: %w", op, err)
		}

		if entry, ok := libraryTags[id]; ok {
			entry = append(entry, tag)
			libraryTags[id] = entry
		}
	}

	return libraryTags, nil
}

// allMediaSubBasicInfo returns media list.
func (s *Storage) allMediaSubBasicInfo(tx statementBuilder, ctx context.Context, libraryTags map[int64]models.TagList) ([]models.Media, error) {
	const op = "storage.sqlite.allMediaSubBasicInfo"

	stmt, err := tx.PrepareContext(ctx, `
		SELECT l.id, l.name, l.author, l.duration
		FROM library AS l;
	`)
	if err != nil {
		return []models.Media{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return []models.Media{}, fmt.Errorf("%s: %w", op, err)
	}

	lib := make([]models.Media, 0, len(libraryTags))
	var (
		id           int64
		name, author string
		durationMs   int64
		tagCount     int
	)
	for rows.Next() {
		if err = rows.Scan(&id, &name, &author, &durationMs, &tagCount); err != nil {
			return []models.Media{}, fmt.Errorf("%s: %w", op, err)
		}

		lib = append(lib, models.Media{
			ID:       ptr.Ptr(id),
			Name:     ptr.Ptr(name),
			Author:   ptr.Ptr(author),
			Duration: ptr.Ptr(time.Duration(durationMs) * time.Microsecond),
			Tags:     libraryTags[id],
		})
	}

	return lib, nil
}

// SaveMedia saves necessary information
// about media file to db.
func (s *Storage) SaveMedia(ctx context.Context, media models.Media) (int64, error) {
	const op = "storage.sqlite.SaveMedia"

	stmt, err := s.db.PrepareContext(ctx, "INSERT INTO library(name, author, duration, source_id) VALUES(?, ?, ?, ?)")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, *media.Name, *media.Author, media.Duration.Microseconds(), *media.SourceID)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrMediaExists)
		}

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

// UpdateMedia updates basic media information (without tags).
func (s *Storage) UpdateMediaBasicInfo(ctx context.Context, media models.Media) error {
	const op = "storage.sqlite.UpdateMedia"

	query := fmt.Sprintf(
		`UPDATE library SET name = "%s", author = "%s" WHERE id = "%d"`,
		*media.Name, *media.Author, *media.ID,
	)

	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if _, err := stmt.ExecContext(ctx); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// Media return media file by id.
func (s *Storage) Media(ctx context.Context, id int64) (models.Media, error) {
	const op = "storage.sqlite.Media"

	// start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Media{}, fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback()

	media, err := s.mediaSubBasicInfo(tx, ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrMediaNotFound) {
			return models.Media{}, storage.ErrMediaNotFound
		}
		return models.Media{}, fmt.Errorf("%s: %w", op, err)
	}

	tags, err := s.mediaSubTags(tx, ctx, id)
	if err != nil {
		return models.Media{}, fmt.Errorf("%s: %w", op, err)
	}

	tx.Commit()

	media.Tags = tags

	return media, nil
}

// mediaSubBasicInfo returns media with basic information
// bt its id.
func (s *Storage) mediaSubBasicInfo(tx statementBuilder, ctx context.Context, id int64) (models.Media, error) {
	const op = "storage.sqlite.mediaSubBasicInfo"

	stmt, err := tx.PrepareContext(ctx, "SELECT name, author, duration, source_id FROM library WHERE id = ?")
	if err != nil {
		return models.Media{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(ctx, id)

	var (
		sourceID     int64
		name, author string
		durationMuS  int64
	)

	err = row.Scan(&name, &author, &durationMuS, &sourceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Media{}, fmt.Errorf("%s: %w", op, storage.ErrMediaNotFound)
		}

		return models.Media{}, fmt.Errorf("%s: %w", op, err)
	}

	return models.Media{
		ID:       &id,
		SourceID: &sourceID,
		Name:     &name,
		Author:   &author,
		Duration: ptr.Ptr(time.Duration(durationMuS) * time.Microsecond),
	}, nil
}

// mediaSubTags returns tag list by given media id.
func (s *Storage) mediaSubTags(tx statementBuilder, ctx context.Context, id int64) (models.TagList, error) {
	const op = "storage.sqlite.mediaSubTags"

	stmt, err := tx.PrepareContext(ctx, `
		SELECT t.id, t.name, tt.id, tt.name
		FROM libraryTag AS lt
		JOIN tag as t ON t.id = lt.tag_id
		JOIN library AS l ON l.id = lt.media_id
		JOIN tagType AS tt ON tt.id = t.type_id
		WHERE l.id = ?
	`)
	if err != nil {
		return models.TagList{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, id)
	if err != nil {
		return models.TagList{}, fmt.Errorf("%s: %w", op, err)
	}

	var tag models.Tag
	tags := make(models.TagList, 0)

	for rows.Next() {
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Type.ID, &tag.Type.Name); err != nil {
			return models.TagList{}, fmt.Errorf("%s: %w", op, err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// DeleteMedia deletes media by id.
func (s *Storage) DeleteMedia(ctx context.Context, id int64) error {
	const op = "storage.sqlite.DeleteMedia"

	stmt, err := s.db.Prepare("DELETE FROM library WHERE id = ?")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	affectedRows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if affectedRows == 0 {
		return storage.ErrMediaNotFound
	}

	return nil
}

// TagTypes return available tag types. Returned error is always nil.
func (s *Storage) TagTypes(ctx context.Context) (models.TagTypes, error) {
	return s.tagCache.tagTypes, nil
}

// AllTags returns all registered tags. Returned error is always nil.
func (s *Storage) AllTags(ctx context.Context) (models.TagList, error) {
	s.tagCache.mutex.Lock()
	defer s.tagCache.mutex.Unlock()

	actualTagList := append(models.TagList(nil), s.tagCache.tagList...)

	return actualTagList, nil
}

// updateTagTypes gets availabel tag types.
// Since tag type list does not change during program,
// it should be called only once at the start.
func (s *Storage) updateTagTypes(ctx context.Context) error {
	const op = "storage.sqlite.updateTagTypes"

	stmt, err := s.db.PrepareContext(ctx, "SELECT id, name FROM tagType")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	var tagType models.TagType
	s.tagCache.tagTypes = make(models.TagTypes, 0)

	for rows.Next() {
		if err := rows.Scan(&tagType.ID, &tagType.Name); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
		s.tagCache.tagTypes = append(s.tagCache.tagTypes, tagType)
	}

	return nil
}

// updateTagList gets actual tag list.
// Should be called at the start of the session
// and after updating information in it.
func (s *Storage) updateTagList(ctx context.Context) error {
	const op = "storage.sqlite.updateTagList"

	stmt, err := s.db.PrepareContext(ctx, "SELECT id, name, type_id FROM tag")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	s.tagCache.mutex.Lock()
	defer s.tagCache.mutex.Unlock()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	// Expected that usually list length increases
	oldLength := len(s.tagCache.tagList)
	s.tagCache.tagList = make(models.TagList, 0, oldLength)

	var tag models.Tag

rows_loop:
	for rows.Next() {
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Type.ID); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		for _, tagType := range s.tagCache.tagTypes {
			if tagType.ID == tag.Type.ID {
				tag.Type.Name = tagType.Name
				s.tagCache.tagList = append(s.tagCache.tagList, tag)
				continue rows_loop
			}
		}
	}

	return nil
}

// SaveTag saves new tag
func (s *Storage) SaveTag(ctx context.Context, tag models.Tag) (int64, error) {
	const op = "storage.sqlite.SaveTag"

	defer s.updateTagList(ctx)

	s.tagCache.mutex.Lock()
	defer s.tagCache.mutex.Unlock()

	stmt, err := s.db.PrepareContext(ctx, `
		INSERT INTO tag(name, type_id)
		VALUES(?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	res, err := stmt.ExecContext(ctx, tag.Name, tag.Type.ID)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrTagExists)
		}

		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

// DeleteTag deletes tag by its name
func (s *Storage) DeleteTag(ctx context.Context, id int64) error {
	const op = "storage.sqlite.DeleteTag"

	defer s.updateTagList(ctx)

	s.tagCache.mutex.Lock()
	defer s.tagCache.mutex.Unlock()

	stmt, err := s.db.Prepare("DELETE FROM tag WHERE id = ?")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	affectedRows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if affectedRows == 0 {
		return storage.ErrTagNotFound
	}

	return nil
}

// TagMedia adds new tags to the media.
func (s *Storage) TagMedia(ctx context.Context, mediaId int64, tags ...models.Tag) error {
	const op = "storage.sqlite.TagMedia"

	if len(tags) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString("INSERT INTO libraryTag(media_id, tag_id) VALUES")
	for _, tag := range tags {
		fmt.Fprintf(&b, "(%d,%d),", mediaId, tag.ID)
	}
	query := strings.TrimSuffix(b.String(), ",") + ";"

	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if _, err := stmt.ExecContext(ctx); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// UntagMedia deletes tags from media.
func (s *Storage) UntagMedia(ctx context.Context, mediaId int64, tags ...models.Tag) error {
	const op = "storage.sqlite.UntagMedia"

	if len(tags) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString("DELETE FROM libraryTag WHERE (media_id, tag_id) IN (")
	for _, tag := range tags {
		fmt.Fprintf(&b, "(%d,%d),", mediaId, tag.ID)
	}
	query := strings.TrimSuffix(b.String(), ",") + ");"

	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if _, err := stmt.ExecContext(ctx); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
