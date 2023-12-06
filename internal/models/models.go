package models

import "time"

type User struct {
	ID       int64
	Login    string
	PassHash []byte
}

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
