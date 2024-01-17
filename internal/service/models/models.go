package models

import "time"

// TODO: split into different files when become too big

type EditorIn struct {
	Login string `json:"login"`
	Pass  string `json:"pass"`
}

type EditorOut struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type Editor struct {
	ID       int64  `json:"id"`
	Login    string `json:"login"`
	PassHash []byte `json:"pass"`
}

const (
	ErrEditorID int64 = 0

	RootID    int64 = -1
	RootLogin       = "root"
)

type Media struct {
	ID       *int64         `json:"id"`
	Name     *string        `json:"name"`
	Author   *string        `json:"author"`
	Duration *time.Duration `json:"duration"`
	SourceID *int64         `json:"-"`
}

type Segment struct {
	ID       *int64         `json:"id"`
	MediaID  *int64         `json:"mediaID"`
	Start    *time.Time     `json:"start"`
	BeginCut *time.Duration `json:"beginCut"`
	StopCut  *time.Duration `json:"stopCut"`
}
