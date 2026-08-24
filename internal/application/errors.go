package application

import (
	"errors"
	"fmt"
	"strings"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

type repositoryOperationError struct {
	operation string
	message   string
}

func newRepositoryOperationError(operation string, err error) error {
	return &repositoryOperationError{operation: operation, message: err.Error()}
}

func (e *repositoryOperationError) Error() string {
	return fmt.Sprintf("持久化操作 %s 失败: %s", e.operation, e.message)
}

func validateMeta(meta CommandMeta) error {
	if meta.ExpectedRevision < 1 {
		return domain.NewValidation("revision_required", "revision", "必须提供正整数修订号")
	}
	if strings.TrimSpace(meta.RequestID) == "" {
		return domain.NewValidation("request_id_required", "request_id", "修改请求必须提供请求标识")
	}
	if len(meta.RequestID) > 128 {
		return domain.NewValidation("request_id_too_long", "request_id", "请求标识不能超过 128 个字符")
	}
	return nil
}

func translateRepositoryError(err error) error {
	if errors.Is(err, persistence.ErrNotFound) {
		return &domain.BusinessError{Kind: domain.ErrorNotFound, Code: "application_not_found", Message: "申请不存在"}
	}
	var conflict *persistence.ConflictError
	if errors.As(err, &conflict) {
		return domain.RevisionConflict(conflict.Expected, conflict.Actual)
	}
	var duplicate *persistence.ActiveDuplicateError
	if errors.As(err, &duplicate) {
		return domain.NewDetailedConflict("active_application_duplicate", "同一古树已有未归档迁移申请", map[string]any{"application_id": duplicate.ApplicationID, "status": duplicate.Status, "revision": duplicate.Revision})
	}
	return err
}
