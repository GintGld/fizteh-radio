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
	ErrEditorID int64 = -2

	RootID    int64 = -1
	RootLogin       = "root"
)

type Media struct {
	ID       int64
	Name     string
	Author   string
	Duration time.Duration
}

type Segment struct {
	ID       int64
	MediaID  int64
	Period   int64
	Start    time.Time
	BeginCut time.Duration
	StopCut  time.Duration
}
