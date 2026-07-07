package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/thethoughtcriminal/xray-master/internal/service"
)

func statusFromError(err error) int {
	switch {
	case errors.Is(err, service.ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, service.ErrConflict):
		return http.StatusConflict
	default:
		if err != nil && strings.Contains(err.Error(), "node ") {
			return http.StatusBadGateway
		}
		return http.StatusInternalServerError
	}
}
