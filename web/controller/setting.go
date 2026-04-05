package controller

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/alireza0/pardis-ui/config"
	"github.com/alireza0/pardis-ui/web/entity"
	"github.com/alireza0/pardis-ui/web/service"
	"github.com/alireza0/pardis-ui/web/session"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type updateUserForm struct {
	OldUsername string `json:"oldUsername" form:"oldUsername"`
	OldPassword string `json:"oldPassword" form:"oldPassword"`
	NewUsername string `json:"newUsername" form:"newUsername"`
	NewPassword string `json:"newPassword" form:"newPassword"`
}

type SettingController struct {
	settingService service.SettingService
	userService    service.UserService
	panelService   service.PanelService
}

func NewSettingController(g *gin.RouterGroup) *SettingController {
	a := &SettingController{}
	a.initRouter(g)
	return a
}

func (a *SettingController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/setting")

	g.POST("/all", a.getAllSetting)
	g.POST("/defaultSettings", a.getDefaultSettings)
	g.POST("/update", a.updateSetting)
	g.POST("/updateUser", a.updateUser)
	g.POST("/uploadCert", a.uploadCert)
	g.GET("/dbFolder", a.getDbFolder)
	g.POST("/updateDbFolder", a.updateDbFolder)
	g.POST("/restartPanel", a.restartPanel)
	g.GET("/getDefaultJsonConfig", a.getDefaultXrayConfig)
}

func (a *SettingController) getAllSetting(c *gin.Context) {
	allSetting, err := a.settingService.GetAllSetting()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, allSetting, nil)
}

func (a *SettingController) getDefaultSettings(c *gin.Context) {
	result, err := a.settingService.GetDefaultSettings(c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, result, nil)
}

func (a *SettingController) updateSetting(c *gin.Context) {
	allSetting := &entity.AllSetting{}
	err := c.ShouldBind(allSetting)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	err = a.settingService.UpdateAllSetting(allSetting)
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
}

func (a *SettingController) updateUser(c *gin.Context) {
	form := &updateUserForm{}
	err := c.ShouldBind(form)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	user := session.GetLoginUser(c)
	if user.Username != form.OldUsername || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(form.OldPassword)) != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUser"), errors.New(I18nWeb(c, "pages.settings.toasts.originalUserPassIncorrect")))
		return
	}
	if form.NewUsername == "" || form.NewPassword == "" {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUser"), errors.New(I18nWeb(c, "pages.settings.toasts.userPassMustBeNotEmpty")))
		return
	}
	err = a.userService.UpdateUser(user.Id, form.NewUsername, form.NewPassword)
	if err == nil {
		updatedUser, fetchErr := a.userService.GetFirstUser()
		if fetchErr == nil {
			session.SetLoginUser(c, updatedUser)
		}
	}
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifyUser"), err)
}

func (a *SettingController) uploadCert(c *gin.Context) {
	certType := c.PostForm("certType")
	validTypes := map[string]bool{
		"webCert": true, "webKey": true, "subCert": true, "subKey": true,
	}
	if !validTypes[certType] {
		jsonMsg(c, "upload certificate", errors.New("invalid certType"))
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		jsonMsg(c, "upload certificate", err)
		return
	}
	defer file.Close()

	certsDir := filepath.Join(config.GetDBFolderPath(), "certs")
	if err := os.MkdirAll(certsDir, 0750); err != nil {
		jsonMsg(c, "upload certificate", err)
		return
	}

	destPath := filepath.Join(certsDir, certType+".pem")
	dest, err := os.Create(destPath)
	if err != nil {
		jsonMsg(c, "upload certificate", err)
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		jsonMsg(c, "upload certificate", err)
		return
	}

	switch certType {
	case "webCert":
		err = a.settingService.SetCertFile(destPath)
	case "webKey":
		err = a.settingService.SetKeyFile(destPath)
	case "subCert":
		err = a.settingService.SetSubCertFile(destPath)
	case "subKey":
		err = a.settingService.SetSubKeyFile(destPath)
	}
	if err != nil {
		jsonMsg(c, "upload certificate", err)
		return
	}
	jsonObj(c, destPath, nil)
}

func (a *SettingController) getDbFolder(c *gin.Context) {
	jsonObj(c, config.GetDBFolderPath(), nil)
}

func (a *SettingController) updateDbFolder(c *gin.Context) {
	newFolder := c.PostForm("dbFolder")
	if newFolder == "" {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), errors.New("db folder path cannot be empty"))
		return
	}

	newFolder = filepath.Clean(newFolder)
	if err := os.MkdirAll(newFolder, 0750); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}

	currentDBPath := config.GetDBPath()
	newDBPath := filepath.Join(newFolder, filepath.Base(currentDBPath))

	if filepath.Clean(currentDBPath) != newDBPath {
		if err := copyFile(currentDBPath, newDBPath); err != nil {
			jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
			return
		}
	}

	if err := config.SetDBFolderPath(newFolder); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}

	// Reload via SIGHUP (which now also re-inits the DB)
	if err := a.panelService.RestartPanel(time.Second * 3); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}

	jsonObj(c, newFolder, nil)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func (a *SettingController) restartPanel(c *gin.Context) {
	err := a.panelService.RestartPanel(time.Second * 3)
	jsonMsg(c, I18nWeb(c, "pages.settings.restartPanel"), err)
}

func (a *SettingController) getDefaultXrayConfig(c *gin.Context) {
	defaultJsonConfig, err := a.settingService.GetDefaultXrayConfig()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, defaultJsonConfig, nil)
}
