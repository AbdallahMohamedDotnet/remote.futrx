package user

import "errors"

var (
	ErrInvalidEmail          = errors.New("invalid email")
	ErrInvalidRole           = errors.New("invalid role (must be admin or member)")
	ErrUserExists            = errors.New("user already exists")
	ErrUserNotFound          = errors.New("user not found")
	ErrCannotDemoteLastAdmin = errors.New("cannot demote the last admin")
	ErrCannotRemoveLastAdmin = errors.New("cannot remove the last admin")
)
