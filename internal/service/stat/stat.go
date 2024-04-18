package stat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/GintGld/fizteh-radio/internal/lib/logger/sl"
	"github.com/GintGld/fizteh-radio/internal/models"
	"github.com/GintGld/fizteh-radio/internal/service"
	"github.com/GintGld/fizteh-radio/internal/storage"
)

var (
	infinity = time.Date(2100, 1, 1, 0, 0, 0, 0, time.Local)
)

type Stat struct {
	log             *slog.Logger
	listenerStorage ListenerStorage
	timeout         time.Duration

	listeners     map[int64]models.Listener
	listenerTiker map[int64]*time.Timer
	mutex         *sync.Mutex
}

type ListenerStorage interface {
	SaveListener(ctx context.Context, listener models.Listener) (int64, error)
	Listeners(ctx context.Context, start, stop time.Time) ([]models.Listener, error)
}

func New(
	log *slog.Logger,
	listenerStorage ListenerStorage,
	timeout time.Duration,
) *Stat {
	return &Stat{
		log:             log,
		listenerStorage: listenerStorage,
		timeout:         timeout,

		listeners:     make(map[int64]models.Listener),
		listenerTiker: make(map[int64]*time.Timer),
		mutex:         &sync.Mutex{},
	}
}

// RegisterListener registers new session.
func (s *Stat) RegisterListener() int64 {
	var id int64

	s.mutex.Lock()
	defer s.mutex.Unlock()

	for ; ; id = rand.Int63() {
		if _, ok := s.listeners[id]; !ok {
			break
		}
	}

	s.listeners[id] = models.Listener{
		ID:    id,
		Start: time.Now(),
		Stop:  infinity,
	}

	// Set timeout.
	s.setTimeout(id)

	return id
}

// PingListener updates timeout for session.
func (s *Stat) PingListener(id int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.listenerTiker[id]; !ok {
		return
	}
	s.listenerTiker[id].Stop()
	s.setTimeout(id)
}

func (s *Stat) setTimeout(id int64) {
	s.listenerTiker[id] = time.AfterFunc(s.timeout, func() { s.UnregisterListener(id) })
}

// UnregisterListener stops session.
func (s *Stat) UnregisterListener(id int64) {
	const op = "Stat.UnregisterListener"

	log := s.log.With(
		slog.String("op", op),
		slog.Int64("id", id),
	)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	listener := s.listeners[id]
	listener.Stop = time.Now().Add(-s.timeout)

	delete(s.listeners, id)
	delete(s.listenerTiker, id)

	if _, err := s.listenerStorage.SaveListener(context.Background(), listener); err != nil {
		log.Error("failed to save listener", slog.Any("listener", listener), sl.Err(err))
	}
}

// ListenersNumber returns current
// number of listeners.
func (s *Stat) ListenersNumber() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return len(s.listeners)
}

// Listeners returns sessions
// in given time interval.
func (s *Stat) Listeners(ctx context.Context, start, stop time.Time) ([]models.Listener, error) {
	const op = "stat.Listeners"

	log := s.log.With(
		slog.String("op", op),
	)

	res, err := s.listenerStorage.Listeners(ctx, start, stop)
	if err != nil {
		if errors.Is(err, storage.ErrContextCancelled) {
			log.Error("listenerStorage.Listeners timeout exceeded")
			return []models.Listener{}, service.ErrTimeout
		}
		log.Error("failed to get listeners", slog.Time("start", start), slog.Time("stop", stop), sl.Err(err))
		return []models.Listener{}, fmt.Errorf("%s: %w", op, err)
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Add current listeners
	// don't check listener's stop
	for _, l := range s.listeners {
		if l.Start.Before(stop) {
			res = append(res, l)
		}
	}

	return res, nil
}
