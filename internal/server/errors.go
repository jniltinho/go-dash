package server

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/jniltinho/go-dash/internal/auth"
	"github.com/jniltinho/go-dash/internal/menu"
)

// errorResponse is the JSON body for every API error.
type errorResponse struct {
	Error string `json:"error"`
	Field string `json:"field,omitempty"`
}

// handleError maps domain and HTTP errors to JSON responses.
func (s *Server) handleError(c *echo.Context, err error) {
	if resp, _ := echo.UnwrapResponse(c.Response()); resp != nil && resp.Committed {
		return
	}

	status := http.StatusInternalServerError
	body := errorResponse{Error: "internal server error"}

	var (
		validation *menu.ValidationError
		password   *auth.PasswordError
		httpErr    *echo.HTTPError
	)
	switch {
	case errors.As(err, &validation):
		status = http.StatusBadRequest
		body = errorResponse{Error: validation.Message, Field: validation.Field}
	case errors.As(err, &password):
		status = http.StatusBadRequest
		body = errorResponse{Error: password.Message, Field: "password"}
	case errors.Is(err, menu.ErrNotFound):
		status = http.StatusNotFound
		body.Error = "item not found"
	case errors.Is(err, menu.ErrDashboardNotFound), errors.Is(err, auth.ErrUnknownDashboard):
		status = http.StatusNotFound
		body.Error = "dashboard not found"
	case errors.Is(err, menu.ErrDashboardNotEmpty):
		status = http.StatusConflict
		body.Error = menu.ErrDashboardNotEmpty.Error()
	case errors.As(err, &httpErr):
		status = httpErr.Code
		body.Error = httpErr.Message
		if body.Error == "" {
			body.Error = http.StatusText(status)
		}
	default:
		if code := echo.StatusCode(err); code != 0 {
			status = code
			body.Error = http.StatusText(code)
		}
	}

	if status >= http.StatusInternalServerError {
		s.deps.Logger.Error("request failed", "path", c.Request().URL.Path, "error", err)
	}

	var writeErr error
	if c.Request().Method == http.MethodHead {
		writeErr = c.NoContent(status)
	} else {
		writeErr = c.JSON(status, body)
	}
	if writeErr != nil {
		s.deps.Logger.Error("writing error response failed", "error", writeErr)
	}
}
