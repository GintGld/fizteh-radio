package stream

import (
	"time"
)

// Struct includes all information needed to
// manage MPD streaming
type config struct {
	MinimumUpdatePeriod        string
	MinBufferTime              string
	ID                         int
	AvailabilityStartTime      time.Time
	SuggestedPresentationDelay string
	FileDirectory              string
	SchedulePath               string
}

// Struct includes neccecary information
// for updating DASH manifest
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

var ScheduleGlobalConfig config

// Dir where to place all file for streaming
const BaseDir = "tmp"

// Checks for a package
func StreamInit() error {
	return checkFFmpeg()
}

// Create new DASH manifest with given parameters
func InitSchedule() {
	initMPD()
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
