package stream

import (
	"context"
	"time"
)

// Struct accumulating all information needed to
// manage MPD streaming.
// Broadcast Grid consist of prepared files (compositions)
type broadcastGrid struct {
	Manifest manifestSettings
	mpd      manifestAPI
	Store    contentGenerator
}

type manifestSettings struct {
	UpdateFrequency time.Duration // MPD MinimumUpdateFrequency
	BufferTime      time.Duration // MPD MinBufferTime
	ID              int           // MPD ID
	StartTime       time.Time     // MPD AvailabilityStartTime
	SuggestedDelay  time.Duration // MPD SuggestedPresentationDelay
	Path            string        // path to dump dunamic manifest
}

type contentGenerator interface {
	getNextComposition() (*composition, error)
}

// Struct includes neccecary information
// for upload composition to the manifest
type composition struct {
	id              int           // unique id for composition
	file            string        // path to file
	segmentDuration time.Duration // duration of segment
	meta            metadata      // metadata
}

// Important information from metadata
type metadata struct {
	duration      time.Duration // duration
	bitrate       int           // bitrate of composition
	sampling_rate int           // sampling rate of composition
	channels      int           // number of channels (1 or 2)
}

// Dir to place all files for streaming (placeholder)
const BaseDir = "tmp"

// Checks for a package
func Init(StartTime time.Time, BufferTime time.Duration, UpdateFrequency time.Duration) (*broadcastGrid, error) {
	if err := checkFFmpeg(); err != nil {
		return nil, err
	}

	bcg := new(broadcastGrid)

	bcg.Manifest.StartTime = StartTime
	bcg.Manifest.BufferTime = BufferTime
	bcg.Manifest.UpdateFrequency = UpdateFrequency

	bcg.initMPD()

	return bcg, nil
}

// Reset manifest
func (bcg *broadcastGrid) Reset() {
	bcg.initMPD()
}

// Creates new composition (reads metadata of file)
func NewComp(id int, file string, segmentDuration time.Duration) (*composition, error) {
	cmp := composition{id: id, file: file, segmentDuration: segmentDuration}

	meta, err := newMeta(&file)
	if err != nil {
		return nil, err
	}
	cmp.meta = *meta

	return &cmp, nil
}

// Looks at actual schedule and prepare files for loading by client.
// Main function in package, stops by signal from context.
func (bcg *broadcastGrid) Run(ctx context.Context) error {
	// Time "horizon" of prepared data
	horizon := bcg.Manifest.StartTime

	// get first composition
	cmp, err := bcg.Store.getNextComposition()
	if err != nil {
		return err
	}

	if err = bcg.addNewComposition(cmp); err != nil {
		return err
	}

	// Increase horizon due to new composition in manifest
	horizon = horizon.Add(cmp.meta.duration)

	// Main loop, can be stopped via Context
	for {
		if time.Until(horizon) < 10*time.Second { // placeholder
			cmp, err := bcg.Store.getNextComposition()
			if err != nil {
				return err
			}

			if err = bcg.addNewComposition(cmp); err != nil {
				return err
			}

			if err = bcg.dump(); err != nil {
				return err
			}

			if err = bcg.deleteAlreadyPlayed(); err != nil {
				return err
			}

			horizon = horizon.Add(cmp.meta.duration)
		}

		// Check if Context was closed
		// In other case wait to make one more iteration
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second): // placeholder
		}
	}
}
