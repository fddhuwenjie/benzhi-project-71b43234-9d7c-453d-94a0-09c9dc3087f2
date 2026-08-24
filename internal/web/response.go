package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
)

type apiError struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Field   string         `json:"field,omitempty"`
		Details map[string]any `json:"details,omitempty"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message, field string, details ...map[string]any) {
	var response apiError
	response.Error.Code, response.Error.Message, response.Error.Field = code, message, field
	if len(details) > 0 {
		response.Error.Details = details[0]
	}
	writeJSON(w, status, response)
}

func handleError(w http.ResponseWriter, err error) {
	var business *domain.BusinessError
	if errors.As(err, &business) {
		status := http.StatusUnprocessableEntity
		switch business.Kind {
		case domain.ErrorConflict:
			status = http.StatusConflict
		case domain.ErrorState:
			status = http.StatusConflict
		case domain.ErrorNotFound:
			status = http.StatusNotFound
		}
		writeError(w, status, business.Code, business.Message, business.Field, business.Details)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", "服务处理请求时发生内部错误", "")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return domain.NewValidation("body_required", "", "请求体不能为空")
		}
		return domain.NewValidation("invalid_json", "", "请求 JSON 无效或包含未知字段")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.NewValidation("multiple_json_values", "", "请求体只能包含一个 JSON 对象")
	}
	return nil
}
