package ginai

import "github.com/gin-gonic/gin"

// RegisterDebugRoutes exposes registered tools for local inspection.
func RegisterDebugRoutes(router gin.IRouter, registry *Registry) {
	router.GET("/debug/ginai/tools", func(c *gin.Context) {
		c.JSON(200, gin.H{"tools": registry.List()})
	})
}
