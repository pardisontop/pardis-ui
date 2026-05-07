package controller

import (
	"github.com/pardisontop/pardis-ui/web/service"

	"github.com/gin-gonic/gin"
)

type ClientAnalyticsController struct {
	clientAnalyticsService service.ClientAnalyticsService
}

func NewClientAnalyticsController(g *gin.RouterGroup) *ClientAnalyticsController {
	a := &ClientAnalyticsController{}
	a.initRouter(g)
	return a
}

func (a *ClientAnalyticsController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/clientAnalytics")

	g.POST("/report", a.report)
}

func (a *ClientAnalyticsController) report(c *gin.Context) {
	req := service.ClientAnalyticsRequest{}
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "client analytics", err)
		return
	}
	report, err := a.clientAnalyticsService.GetClientReport(req)
	jsonObj(c, report, err)
}
