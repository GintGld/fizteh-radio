package service

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEditorNotFound     = errors.New("editor not found")
	ErrEditorExists       = errors.New("editor exists")

	ErrInvalidToken = errors.New("invalid token")
	ErrTimeoutToken = errors.New("timeout token")

	ErrMediaNotFound = errors.New("editor not found")
)
