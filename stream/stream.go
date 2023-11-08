package stream

// Struct includes all information needed to
// manage MPD streaming
type config struct {
	MinimumUpdatePeriod        string
	MinBufferTime              string
	ID                         int
	AvailabilityStartTime      string
	SuggestedPresentationDelay string
	FileDirectory              string
	SchedulePath               string
}

// Struct includes neccecary information
// for updating DASH manifest
type composition struct {
	file            *string   // path to file
	name            *string   // name (i.e. file name without extention)
	segmentDuration float64   // duration of segment
	meta            *metadata // metadata
}

var ScheduleGlobalConfig config

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
	cmp := composition{file: file, name: name, segmentDuration: segmentDuration}
	meta, err := newMeta(file)

	if err != nil {
		return nil, err
	}

	cmp.meta = meta

	return &cmp, nil
}
