package health

import (
	"github.com/agent-pilot/agent-pilot-be/model"
	"github.com/gin-gonic/gin"
)

type ControllerInterface interface {
	GetHealth(c *gin.Context) (model.Response, error)
}
type Controller struct {
}

func NewHealthController() *Controller {
	return &Controller{}
}

// GetHealth 健康检查
// @Summary      健康检查
// @Tags         system
// @Produce      json
// @Success      200 {object} model.Response
// @Router       /health [get]
func (hc *Controller) GetHealth(ctx *gin.Context) (model.Response, error) {
	return model.Response{Code: 200, Message: "healthy"}, nil
}
