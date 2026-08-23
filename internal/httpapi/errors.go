package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func ErrorCode(err error) string {
	switch {
	case errors.Is(err, model.ErrInvalid):
		return "invalid_request"
	case errors.Is(err, model.ErrForbidden):
		return "forbidden"
	case errors.Is(err, model.ErrNotFound):
		return "not_found"
	case errors.Is(err, model.ErrConflict):
		return "conflict"
	default:
		return "internal_error"
	}
}

func ErrorStatus(err error) int {
	switch ErrorCode(err) {
	case "invalid_request":
		return http.StatusBadRequest
	case "forbidden":
		return http.StatusForbidden
	case "not_found":
		return http.StatusNotFound
	case "conflict":
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func WriteStructuredError(w http.ResponseWriter, r *http.Request, err error) {
	message := "internal server error"
	if ErrorStatus(err) < 500 && err != nil {
		message = strings.TrimSpace(err.Error())
	}
	body := ErrorBody{Code: ErrorCode(err), Message: message, RequestID: RequestID(r.Context())}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ErrorStatus(err))
	_ = json.NewEncoder(w).Encode(body)
}
