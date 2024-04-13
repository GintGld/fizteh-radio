package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/GintGld/fizteh-radio/internal/models"
)

func (s *Storage) SaveListener(ctx context.Context, listener models.Listener) (int64, error) {
	const op = "storage.sqlite.SaveListener"

	stmt, err := s.db.PrepareContext(ctx, "INSERT INTO listener(start, stop) VALUES(?, ?)")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, listener.Start.Unix(), listener.Stop.Unix())
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil

}

func (s *Storage) Listeners(ctx context.Context, start, stop time.Time) ([]models.Listener, error) {
	const op = "storage.sqlite.Listeners"

	stmt, err := s.db.PrepareContext(ctx, "SELECT id, start, stop FROM listener WHERE stop >= ? OR start <= ?")
	if err != nil {
		return []models.Listener{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, start.Unix(), stop.Unix())
	if err != nil {
		return []models.Listener{}, fmt.Errorf("%s: %w", op, err)
	}

	res := make([]models.Listener, 0)
	var (
		l models.Listener
		startInt,
		stopInt int64
	)

	for rows.Next() {
		if err := rows.Scan(&l.ID, &startInt, &stopInt); err != nil {
			return []models.Listener{}, fmt.Errorf("%s: %w", op, err)
		}
		l.Start = time.Unix(startInt, 0)
		l.Stop = time.Unix(stopInt, 0)
		res = append(res, l)
	}

	return res, nil
}
