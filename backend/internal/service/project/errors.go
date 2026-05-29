package project

import "errors"

var (
	ErrNameRequired = errors.New("name is required")
	ErrInvalidID    = errors.New("invalid project id")
	ErrNotFound     = errors.New("project not found")
)
