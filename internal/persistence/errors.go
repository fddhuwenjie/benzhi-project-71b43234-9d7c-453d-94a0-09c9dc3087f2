package persistence

import "errors"

import "benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"

var ErrNotFound = errors.New("申请不存在")

type ConflictError struct {
	Expected int
	Actual   int
}

func (e *ConflictError) Error() string { return "持久化修订号冲突" }

type ActiveDuplicateError struct {
	ApplicationID string
	Status        domain.Status
	Revision      int
}

func (e *ActiveDuplicateError) Error() string { return "同一古树存在活跃迁移申请" }
