package context

import (
	"github.com/sentriz/gonic/subsonic"

	"github.com/labstack/echo"
)

type Subsonic struct {
	echo.Context
}

func (c *Subsonic) Respond(code int, r *subsonic.Response) error {
	format := c.QueryParams().Get("f")
	switch format {
	case "json":
		return c.JSON(code, r)
	case "jsonp":
		callback := c.QueryParams().Get("callback")
		return c.JSONP(code, callback, r)
	default:
		return c.XML(code, r)
	}
}
