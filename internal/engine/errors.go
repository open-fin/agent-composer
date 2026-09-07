package engine

import "errors"

// ErrValidation marks a caller mistake.
var ErrValidation = errors.New("validation error")

// ErrConflict marks a state clash, such as registering a candidate twice.
var ErrConflict = errors.New("conflict")
