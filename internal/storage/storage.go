package storage

import "errors"

var (
	ErrEditorExists     = errors.New("editor exists")
	ErrEditorNotFound   = errors.New("editor not found")
	ErrMediaExists      = errors.New("media exists")
	ErrMediaNotFound    = errors.New("media not found")
	ErrSegmentExists    = errors.New("segment exists")
	ErrSegmentNotFound  = errors.New("segment not found")
	ErrSegmentIntersect = errors.New("segment intersects with others")
)
