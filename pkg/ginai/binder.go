package ginai

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type routeBinder struct {
	registry *Registry
}

func NewBinder(registry *Registry) *routeBinder {
	return &routeBinder{registry: registry}
}

func (b *routeBinder) Bind(tool Tool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.FullPath() == "" {
			c.Next()
			return
		}
		registered := tool
		if existing, exists := b.registry.Get(tool.Name); exists {
			registered = *existing
		}
		registered.Method = c.Request.Method
		registered.Path = c.FullPath()
		if err := b.registry.Upsert(&registered); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Next()
	}
}
