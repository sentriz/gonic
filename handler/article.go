package handler

import (
	"net/http"

	"github.com/labstack/echo"
)

// GetTest doesn't do anything
func (h *Handler) GetTest(c echo.Context) error {
	return c.JSON(http.StatusOK, "hello")
}
