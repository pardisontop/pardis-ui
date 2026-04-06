package controller

import (
	"github.com/gin-gonic/gin"
)

type PardisController struct {
	BaseController

	inboundController     *InboundController
	settingController     *SettingController
	xraySettingController *XraySettingController
}

func NewPardisController(g *gin.RouterGroup) *PardisController {
	a := &PardisController{}
	a.initRouter(g)
	return a
}

func (a *PardisController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/pardis")
	g.Use(a.checkLogin)

	g.GET("/", a.index)
	g.GET("/inbounds", a.inbounds)
	g.GET("/settings", a.settings)
	g.GET("/xray", a.xraySettings)

	a.inboundController = NewInboundController(g)
	a.settingController = NewSettingController(g)
	a.xraySettingController = NewXraySettingController(g)
}

func (a *PardisController) index(c *gin.Context) {
	html(c, "index.html", "pages.index.title", nil)
}

func (a *PardisController) inbounds(c *gin.Context) {
	html(c, "inbounds.html", "pages.inbounds.title", nil)
}

func (a *PardisController) settings(c *gin.Context) {
	html(c, "settings.html", "pages.settings.title", nil)
}

func (a *PardisController) xraySettings(c *gin.Context) {
	html(c, "xray.html", "pages.xray.title", nil)
}
