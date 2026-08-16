package storage

import (
	"errors"
)

var ErrMailFolderHasRules = errors.New("mail folder has rules")

var ErrNotFound = errors.New("not found")
var ErrInvalidAccountOrder = errors.New("invalid mail account order")
