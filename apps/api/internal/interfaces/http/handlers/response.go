package handlers

import (
	"errors"
	"net/http"

	"fluxa-api/internal/shared"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{Code: 0, Message: "success", Data: data})
}

func Error(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := 500
	msg := "internal_error"
	switch {
	case errors.Is(err, shared.ErrInvalidInput), errors.Is(err, shared.ErrInvalidTransition):
		status, code, msg = http.StatusBadRequest, 400, err.Error()
	case errors.Is(err, shared.ErrNotFound):
		status, code, msg = http.StatusNotFound, 404, err.Error()
	case errors.Is(err, shared.ErrConflict):
		status, code, msg = http.StatusConflict, 409, err.Error()
	case errors.Is(err, shared.ErrUnauthorized):
		status, code, msg = http.StatusUnauthorized, 401, err.Error()
	case errors.Is(err, shared.ErrForbidden):
		status, code, msg = http.StatusForbidden, 403, err.Error()
	default:
		msg = "internal_error"
	}
	if err != nil {
		_ = c.Error(err)
	}
	c.JSON(status, Response{Code: code, Message: msg})
}
