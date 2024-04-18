package storage

import "errors"

var (
	ErrEditorExists   = errors.New("editor exists")
	ErrEditorNotFound = errors.New("editor not found")

	ErrMediaExists   = errors.New("media exists")
	ErrMediaNotFound = errors.New("media not found")

	ErrTagExists   = errors.New("tag exists")
	ErrTagNotFound = errors.New("tag not found")

	ErrSegmentNotFound              = errors.New("segment not found")
	ErrSegmentAlreadyProtected      = errors.New("segment already protected")
	ErrSegmentAlreadyAttachedToLive = errors.New("segment already attach to live")

	ErrContextCancelled = errors.New("context cancelled")
)
