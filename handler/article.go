package handler

import (
	"net/http"

	"github.com/sentriz/gonic/context"
	"github.com/sentriz/gonic/subsonic"

	"github.com/labstack/echo"
)

// GetTest doesn't do anything
func (h *Handler) GetTest(c echo.Context) error {
	cc := c.(*context.Subsonic)
	resp := subsonic.NewResponse()
	return cc.Respond(http.StatusOK, resp)
}
