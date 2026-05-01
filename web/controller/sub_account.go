package controller

import (
	"strconv"

	"github.com/pardisontop/pardis-ui/web/service"

	"github.com/gin-gonic/gin"
)

type SubAccountController struct {
	subAccountService service.SubAccountService
	xrayService       service.XrayService
}

func NewSubAccountController(g *gin.RouterGroup) *SubAccountController {
	a := &SubAccountController{}
	a.initRouter(g)
	return a
}

func (a *SubAccountController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/subAccount")

	g.POST("/list", a.list)
	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
	g.POST("/reset/:id", a.reset)
}

func (a *SubAccountController) list(c *gin.Context) {
	accounts, err := a.subAccountService.List()
	jsonObj(c, accounts, err)
}

func (a *SubAccountController) add(c *gin.Context) {
	form := &service.SubAccountForm{}
	if err := c.ShouldBind(form); err != nil {
		jsonMsg(c, "sub account", err)
		return
	}
	account, needRestart, err := a.subAccountService.Save(form)
	jsonMsgObj(c, "sub account", account, err)
	if err == nil && needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *SubAccountController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "sub account", err)
		return
	}
	form := &service.SubAccountForm{Id: id}
	if err := c.ShouldBind(form); err != nil {
		jsonMsg(c, "sub account", err)
		return
	}
	form.Id = id
	account, needRestart, err := a.subAccountService.Save(form)
	jsonMsgObj(c, "sub account", account, err)
	if err == nil && needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *SubAccountController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "sub account", err)
		return
	}
	needRestart, err := a.subAccountService.Delete(id)
	jsonMsg(c, "sub account", err)
	if err == nil && needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *SubAccountController) reset(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "sub account", err)
		return
	}
	needRestart, err := a.subAccountService.Reset(id)
	jsonMsg(c, "sub account", err)
	if err == nil && needRestart {
		a.xrayService.SetToNeedRestart()
	}
}
