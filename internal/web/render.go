package web

import (
	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

// Render is a helper to render Templ components through Echo
func Render(c echo.Context, status int, t templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(status)
	return t.Render(c.Request().Context(), c.Response().Writer)
}
