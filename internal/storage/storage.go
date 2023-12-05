package storage

import "errors"

var (
	ErrUserExists       = errors.New("user exists")
	ErrUserNotFound     = errors.New("user not found")
	ErrMediaExists      = errors.New("media exists")
	ErrMediaNotFound    = errors.New("media not found")
	ErrSegmentNotFound  = errors.New("segment not found")
	ErrSegmentIntersect = errors.New("segment intersects with others")
)
