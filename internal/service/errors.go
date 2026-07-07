package service

import "errors"

var (
	ErrValidation = errors.New("validation")
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("conflict")
)
