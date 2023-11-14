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
	Schedule scheduleSettings
	mpd      manifestAPI
}

type manifestSettings struct {
	UpdateFrequency time.Duration // MPD MinimumUpdateFrequency
	BufferTime      time.Duration // MPD MinBufferTime
	ID              int           // MPD ID
	StartTime       time.Time     // MPD AvailabilityStartTime
	SuggestedDelay  time.Duration // MPD SuggestedPresentationDelay
	Path            string        // path to dump dunamic manifest
}

type scheduleSettings struct {
	Source          []*composition // array of compositions that will be used in updating manifest
	UpdateFrequency time.Duration  // updating dynamic manifest frequency
}

// Struct includes neccecary information
// for upload composition to the manifest
type composition struct {
	id              int       // unique id for composition
	file            *string   // path to file
	name            *string   // name (i.e. file name without extention)
	author          *string   // author
	segmentDuration float64   // duration of segment
	meta            *metadata // metadata
}

// Important information from metadata
type metadata struct {
	duration      float64 // duration
	bitrate       int     // bitrate of composition
	sampling_rate int     // sampling rate of composition
	channels      int     // number of channels (1 or 2)
}

// Dir to place all files for streaming (placeholder)
const BaseDir = "tmp"

// Checks for a package
func Init() (*broadcastGrid, error) {
	if err := checkFFmpeg(); err != nil {
		return nil, err
	}

	return new(broadcastGrid), nil
}

// Reset manifest
func (bcg *broadcastGrid) Reset() {
	bcg.initMPD()
	clear(bcg.Schedule.Source)
}

// Creates new composition (reads metadata of file)
func NewComp(file *string, name *string, author *string, segmentDuration float64) (*composition, error) {
	cmp := composition{file: file, name: name, author: author, segmentDuration: segmentDuration}

	meta, err := newMeta(file)
	if err != nil {
		return nil, err
	}
	cmp.meta = meta

	return &cmp, nil
}

// Main function in package, stops by signal from context
// Looks at actual schedule and prepare files for loading by client
func (bcg *broadcastGrid) Run(ctx context.Context) error {
	for {

		time.Sleep(bcg.Schedule.UpdateFrequency)
	}
}
