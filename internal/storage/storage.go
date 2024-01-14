package storage

import "errors"

var (
	ErrEditorExists   = errors.New("editor exists")
	ErrEditorNotFound = errors.New("editor not found")

	ErrMediaExists   = errors.New("media exists")
	ErrMediaNotFound = errors.New("media not found")

	ErrSegmentNotFound = errors.New("segment not found")
)
