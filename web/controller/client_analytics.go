package controller

import (
	"net/http"

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
	g.POST("/export", a.export)
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

func (a *ClientAnalyticsController) export(c *gin.Context) {
	req := service.ClientAnalyticsRequest{}
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "client analytics", err)
		return
	}
	filename, content, err := a.clientAnalyticsService.ExportClientReportCSV(req)
	if err != nil {
		jsonMsg(c, "client analytics", err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", content)
}
