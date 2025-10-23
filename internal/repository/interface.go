package repository

import (
	"errors"
)

// 定义通用的错误
var (
	ErrPlayerNotFound = errors.New("player not found")
	ErrInvalidData    = errors.New("invalid data")
	ErrDuplicateEntry = errors.New("duplicate entry")
)
