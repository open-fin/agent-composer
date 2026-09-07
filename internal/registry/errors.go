package registry

import "errors"

// ErrValidation marks a caller mistake: the API maps it onto 400.
var ErrValidation = errors.New("validation error")

// ErrConflict marks a state clash, such as registering a capability slug twice.
var ErrConflict = errors.New("conflict")
