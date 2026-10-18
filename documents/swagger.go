package documents

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed index.html
var swaggerUI []byte

//go:embed swagger.yaml
var swaggerSpec []byte

// RegisterRoutes serves the embedded Swagger UI and API contract.
func RegisterRoutes(router *gin.Engine) {
	redirect := func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/swagger/index.html")
	}
	router.GET("/swagger", redirect)
	router.GET("/swagger/", redirect)
	router.GET("/swagger/index.html", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", swaggerUI)
	})
	router.GET("/swagger/swagger.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", swaggerSpec)
	})
}
