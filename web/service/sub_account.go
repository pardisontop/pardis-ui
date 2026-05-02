package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pardisontop/pardis-ui/database"
	"github.com/pardisontop/pardis-ui/database/model"
	"github.com/pardisontop/pardis-ui/logger"
	"github.com/pardisontop/pardis-ui/util/common"
	"github.com/pardisontop/pardis-ui/util/random"
	"github.com/pardisontop/pardis-ui/xray"

	"gorm.io/gorm"
)

type SubAccountService struct{}

func (s *SubAccountService) SubIds(tx *gorm.DB) ([]string, error) {
	var subIds []string
	err := tx.Model(model.SubAccount{}).Select("sub_id").Find(&subIds).Error
	return subIds, err
}

func (s *SubAccountService) Exists(tx *gorm.DB, subId string) (bool, error) {
	if subId == "" {
		return false, nil
	}
	var count int64
	err := tx.Model(model.SubAccount{}).Where("sub_id = ?", subId).Count(&count).Error
	return count > 0, err
}

type SubAccountForm struct {
	Id         int    `json:"id" form:"id"`
	Remark     string `json:"remark" form:"remark"`
	SubId      string `json:"subId" form:"subId"`
	Total      int64  `json:"total" form:"total"`
	Duration   int64  `json:"duration" form:"duration"`
	Enable     bool   `json:"enable" form:"enable"`
	InboundIds []int  `json:"inboundIds" form:"inboundIds"`
}

type SubAccountView struct {
	Id         int    `json:"id"`
	Remark     string `json:"remark"`
	SubId      string `json:"subId"`
	Total      int64  `json:"total"`
	Duration   int64  `json:"duration"`
	StartTime  int64  `json:"startTime"`
	ExpiryTime int64  `json:"expiryTime"`
	Enable     bool   `json:"enable"`
	Depleted   bool   `json:"depleted"`
	InboundIds []int  `json:"inboundIds"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
}

type subClientRef struct {
	Protocol string
	Tag      string
	Email    string
	Client   map[string]interface{}
}

func parseSubInboundIds(raw string) []int {
	ids := make([]int, 0)
	if raw == "" {
		return ids
	}
	_ = json.Unmarshal([]byte(raw), &ids)
	return ids
}

func encodeSubInboundIds(ids []int) string {
	data, _ := json.Marshal(ids)
	return string(data)
}

func subInboundIdMap(ids []int) map[int]bool {
	result := make(map[int]bool, len(ids))
	for _, id := range ids {
		if id > 0 {
			result[id] = true
		}
	}
	return result
}

func normalizedSubInboundIds(ids []int) []int {
	seen := subInboundIdMap(ids)
	result := make([]int, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	return result
}

func subAccountExpiryTime(account *model.SubAccount) int64 {
	if account.Duration <= 0 || account.StartTime <= 0 {
		return 0
	}
	return account.StartTime + account.Duration
}

func subAccountLimitReached(account *model.SubAccount, up int64, down int64, now int64) bool {
	if account.Total > 0 && up+down >= account.Total {
		return true
	}
	expiryTime := subAccountExpiryTime(account)
	return expiryTime > 0 && expiryTime <= now
}

func subAccountClientEmail(subId string, inboundId int) string {
	return fmt.Sprintf("sub-%s-%d", subId, inboundId)
}

func isSubAccountProtocol(protocol model.Protocol) bool {
	switch protocol {
	case model.VMess, model.VLESS, model.Trojan, model.Shadowsocks:
		return true
	default:
		return false
	}
}

func newSubAccountClient(protocol model.Protocol, subId string, inboundId int, enable bool) model.Client {
	client := model.Client{
		Email:      subAccountClientEmail(subId, inboundId),
		TotalGB:    0,
		ExpiryTime: 0,
		Enable:     enable,
		SubID:      subId,
		Reset:      0,
	}
	switch protocol {
	case model.VMess:
		client.ID = uuid.NewString()
		client.Security = "auto"
	case model.VLESS:
		client.ID = uuid.NewString()
	case model.Trojan, model.Shadowsocks:
		client.Password = random.Seq(16)
	}
	return client
}

func subAccountClientToMap(client model.Client) (map[string]interface{}, error) {
	data, err := json.Marshal(client)
	if err != nil {
		return nil, err
	}
	clientMap := map[string]interface{}{}
	err = json.Unmarshal(data, &clientMap)
	return clientMap, err
}

func normalizeSubAccountClient(c map[string]interface{}, subId string, enable bool) {
	c["subId"] = subId
	c["totalGB"] = float64(0)
	c["expiryTime"] = float64(0)
	c["reset"] = float64(0)
	c["enable"] = enable
}

func getSubAccountClientMap(client interface{}) (map[string]interface{}, bool) {
	c, ok := client.(map[string]interface{})
	return c, ok
}

func getSubAccountClientString(c map[string]interface{}, key string) string {
	value, _ := c[key].(string)
	return value
}

func getSubAccountClientBool(c map[string]interface{}, key string) bool {
	value, ok := c[key].(bool)
	return ok && value
}

func getSubAccountSettingsClients(settingsRaw string) (map[string]interface{}, []interface{}, error) {
	settings := map[string]interface{}{}
	if err := json.Unmarshal([]byte(settingsRaw), &settings); err != nil {
		return nil, nil, err
	}
	clients, ok := settings["clients"].([]interface{})
	if !ok {
		return nil, nil, common.NewError("inbound does not support clients")
	}
	return settings, clients, nil
}

func getSubAccountTraffic(tx *gorm.DB, subId string) (int64, int64, error) {
	var stats struct {
		Up   int64
		Down int64
	}
	err := tx.Model(xray.ClientTraffic{}).
		Select("COALESCE(SUM(up), 0) as up, COALESCE(SUM(down), 0) as down").
		Where("sub_id = ?", subId).
		Scan(&stats).Error
	return stats.Up, stats.Down, err
}

func (s *SubAccountService) accountView(tx *gorm.DB, account *model.SubAccount) (*SubAccountView, error) {
	up, down, err := getSubAccountTraffic(tx, account.SubId)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix() * 1000
	expiryTime := subAccountExpiryTime(account)
	depleted := subAccountLimitReached(account, up, down, now)
	return &SubAccountView{
		Id:         account.Id,
		Remark:     account.Remark,
		SubId:      account.SubId,
		Total:      account.Total,
		Duration:   account.Duration,
		StartTime:  account.StartTime,
		ExpiryTime: expiryTime,
		Enable:     account.Enable,
		Depleted:   depleted,
		InboundIds: parseSubInboundIds(account.InboundIds),
		Up:         up,
		Down:       down,
	}, nil
}

func (s *SubAccountService) List() ([]*SubAccountView, error) {
	db := database.GetDB()
	accounts := make([]*model.SubAccount, 0)
	if err := db.Model(model.SubAccount{}).Order("id desc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	views := make([]*SubAccountView, 0, len(accounts))
	for _, account := range accounts {
		view, err := s.accountView(db, account)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *SubAccountService) Save(form *SubAccountForm) (*SubAccountView, bool, error) {
	form.SubId = strings.TrimSpace(form.SubId)
	form.Remark = strings.TrimSpace(form.Remark)
	form.InboundIds = normalizedSubInboundIds(form.InboundIds)
	if form.SubId == "" {
		form.SubId = strings.ToLower(random.Seq(16))
	}
	if form.Remark == "" {
		form.Remark = form.SubId
	}
	if len(form.InboundIds) == 0 {
		return nil, false, common.NewError("select at least one inbound")
	}

	db := database.GetDB()
	var existing model.SubAccount
	if err := db.Model(model.SubAccount{}).Where("sub_id = ? AND id != ?", form.SubId, form.Id).First(&existing).Error; err == nil {
		return nil, false, common.NewError("duplicate sub id:", form.SubId)
	} else if err != nil && !database.IsNotFound(err) {
		return nil, false, err
	}

	var oldAccount *model.SubAccount
	if form.Id > 0 {
		oldAccount = &model.SubAccount{}
		if err := db.Model(model.SubAccount{}).First(oldAccount, form.Id).Error; err != nil {
			return nil, false, err
		}
	}

	account := &model.SubAccount{
		Id:         form.Id,
		Remark:     form.Remark,
		SubId:      form.SubId,
		Total:      form.Total,
		Duration:   form.Duration,
		Enable:     form.Enable,
		InboundIds: encodeSubInboundIds(form.InboundIds),
	}
	if oldAccount != nil {
		account.StartTime = oldAccount.StartTime
	}

	tx := db.Begin()
	var err error
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	err = s.validateInbounds(tx, form.InboundIds)
	if err != nil {
		return nil, false, err
	}

	err = tx.Save(account).Error
	if err != nil {
		return nil, false, err
	}

	oldSubId := account.SubId
	if oldAccount != nil {
		oldSubId = oldAccount.SubId
	}
	err = s.syncSubAccountClients(tx, oldSubId, account, form.InboundIds)
	if err != nil {
		return nil, false, err
	}

	view, err := s.accountView(tx, account)
	if err != nil {
		return nil, false, err
	}
	return view, true, nil
}

func (s *SubAccountService) validateInbounds(tx *gorm.DB, inboundIds []int) error {
	inbounds := make([]*model.Inbound, 0, len(inboundIds))
	if err := tx.Model(model.Inbound{}).Where("id IN ?", inboundIds).Find(&inbounds).Error; err != nil {
		return err
	}
	if len(inbounds) != len(inboundIds) {
		return common.NewError("some inbounds were not found")
	}
	for _, inbound := range inbounds {
		if !isSubAccountProtocol(inbound.Protocol) {
			return common.NewError("unsupported inbound protocol:", inbound.Protocol)
		}
		if _, _, err := getSubAccountSettingsClients(inbound.Settings); err != nil {
			return common.NewErrorf("inbound %d does not support multi-user clients", inbound.Id)
		}
	}
	return nil
}

func (s *SubAccountService) syncSubAccountClients(tx *gorm.DB, oldSubId string, account *model.SubAccount, inboundIds []int) error {
	selected := subInboundIdMap(inboundIds)
	up, down, err := getSubAccountTraffic(tx, account.SubId)
	if err != nil {
		return err
	}
	clientEnable := account.Enable && !subAccountLimitReached(account, up, down, time.Now().Unix()*1000)

	inbounds := make([]*model.Inbound, 0)
	if err := tx.Model(model.Inbound{}).
		Where("protocol IN ?", []string{string(model.VMess), string(model.VLESS), string(model.Trojan), string(model.Shadowsocks)}).
		Find(&inbounds).Error; err != nil {
		return err
	}

	for _, inbound := range inbounds {
		settings, clients, err := getSubAccountSettingsClients(inbound.Settings)
		if err != nil {
			continue
		}

		shouldHave := selected[inbound.Id]
		found := false
		changed := false
		newClients := make([]interface{}, 0, len(clients)+1)
		clientEmail := ""

		for _, client := range clients {
			c, ok := getSubAccountClientMap(client)
			if !ok {
				newClients = append(newClients, client)
				continue
			}
			clientSubId := getSubAccountClientString(c, "subId")
			if oldSubId != "" && oldSubId != account.SubId && clientSubId == oldSubId {
				changed = true
				continue
			}
			if clientSubId == account.SubId {
				if shouldHave && !found {
					clientEmail = getSubAccountClientString(c, "email")
					if clientEmail == "" {
						clientEmail = subAccountClientEmail(account.SubId, inbound.Id)
						c["email"] = clientEmail
					}
					normalizeSubAccountClient(c, account.SubId, clientEnable)
					newClients = append(newClients, c)
					found = true
					changed = true
				} else {
					changed = true
				}
				continue
			}
			newClients = append(newClients, client)
		}

		if shouldHave && !found {
			client := newSubAccountClient(inbound.Protocol, account.SubId, inbound.Id, clientEnable)
			clientEmail = client.Email
			clientMap, err := subAccountClientToMap(client)
			if err != nil {
				return err
			}
			newClients = append(newClients, clientMap)
			changed = true
		}

		if changed {
			settings["clients"] = newClients
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return err
			}
			if err := tx.Model(model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", string(modifiedSettings)).Error; err != nil {
				return err
			}
		}

		if shouldHave && clientEmail != "" {
			if err := s.upsertSubAccountClientTraffic(tx, inbound.Id, account.SubId, clientEmail, clientEnable); err != nil {
				return err
			}
		}
	}

	if oldSubId != "" && oldSubId != account.SubId {
		if err := tx.Where("sub_id = ?", oldSubId).Delete(xray.ClientTraffic{}).Error; err != nil {
			return err
		}
	}
	if len(inboundIds) > 0 {
		return tx.Where("sub_id = ? AND inbound_id NOT IN ?", account.SubId, inboundIds).Delete(xray.ClientTraffic{}).Error
	}
	return nil
}

func (s *SubAccountService) upsertSubAccountClientTraffic(tx *gorm.DB, inboundId int, subId string, email string, enable bool) error {
	traffic := &xray.ClientTraffic{}
	err := tx.Model(xray.ClientTraffic{}).Where("email = ?", email).First(traffic).Error
	if database.IsNotFound(err) {
		return tx.Create(&xray.ClientTraffic{
			InboundId:   inboundId,
			SubId:       subId,
			Enable:      enable,
			Email:       email,
			Up:          0,
			Down:        0,
			Total:       0,
			ExpiryTime:  0,
			Reset:       0,
		}).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(xray.ClientTraffic{}).Where("id = ?", traffic.Id).Updates(map[string]interface{}{
		"inbound_id":  inboundId,
		"sub_id":      subId,
		"enable":      enable,
		"total":       0,
		"expiry_time": 0,
		"reset":       0,
	}).Error
}

func (s *SubAccountService) Delete(id int) (bool, error) {
	db := database.GetDB()
	account := &model.SubAccount{}
	if err := db.Model(model.SubAccount{}).First(account, id).Error; err != nil {
		return false, err
	}

	tx := db.Begin()
	var err error
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	_, err = s.removeSubAccountClients(tx, account.SubId)
	if err != nil {
		return false, err
	}
	err = tx.Where("sub_id = ?", account.SubId).Delete(xray.ClientTraffic{}).Error
	if err != nil {
		return false, err
	}
	err = tx.Delete(account).Error
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *SubAccountService) Reset(id int) (bool, error) {
	db := database.GetDB()
	account := &model.SubAccount{}
	if err := db.Model(model.SubAccount{}).First(account, id).Error; err != nil {
		return false, err
	}

	tx := db.Begin()
	var err error
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	err = tx.Model(model.SubAccount{}).Where("id = ?", account.Id).Updates(map[string]interface{}{
		"start_time": int64(0),
		"enable":     true,
	}).Error
	if err != nil {
		return false, err
	}
	err = tx.Model(xray.ClientTraffic{}).Where("sub_id = ?", account.SubId).Updates(map[string]interface{}{
		"up":     int64(0),
		"down":   int64(0),
		"enable": true,
	}).Error
	if err != nil {
		return false, err
	}
	_, err = s.setSubAccountClientsEnable(tx, account.SubId, true)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *SubAccountService) removeSubAccountClients(tx *gorm.DB, subId string) ([]subClientRef, error) {
	inbounds := make([]*model.Inbound, 0)
	if err := tx.Model(model.Inbound{}).Where("protocol IN ?", []string{string(model.VMess), string(model.VLESS), string(model.Trojan), string(model.Shadowsocks)}).Find(&inbounds).Error; err != nil {
		return nil, err
	}
	refs := make([]subClientRef, 0)
	for _, inbound := range inbounds {
		settings, clients, err := getSubAccountSettingsClients(inbound.Settings)
		if err != nil {
			continue
		}
		changed := false
		newClients := make([]interface{}, 0, len(clients))
		for _, client := range clients {
			c, ok := getSubAccountClientMap(client)
			if !ok || getSubAccountClientString(c, "subId") != subId {
				newClients = append(newClients, client)
				continue
			}
			changed = true
			if getSubAccountClientBool(c, "enable") {
				refs = append(refs, subClientRef{Protocol: string(inbound.Protocol), Tag: inbound.Tag, Email: getSubAccountClientString(c, "email"), Client: c})
			}
		}
		if changed {
			settings["clients"] = newClients
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return nil, err
			}
			if err := tx.Model(model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", string(modifiedSettings)).Error; err != nil {
				return nil, err
			}
		}
	}
	return refs, nil
}

func (s *SubAccountService) setSubAccountClientsEnable(tx *gorm.DB, subId string, enable bool) ([]subClientRef, error) {
	inbounds := make([]*model.Inbound, 0)
	if err := tx.Model(model.Inbound{}).Where("protocol IN ?", []string{string(model.VMess), string(model.VLESS), string(model.Trojan), string(model.Shadowsocks)}).Find(&inbounds).Error; err != nil {
		return nil, err
	}
	refs := make([]subClientRef, 0)
	for _, inbound := range inbounds {
		settings, clients, err := getSubAccountSettingsClients(inbound.Settings)
		if err != nil {
			continue
		}
		changed := false
		for index, client := range clients {
			c, ok := getSubAccountClientMap(client)
			if !ok || getSubAccountClientString(c, "subId") != subId {
				continue
			}
			oldEnable := getSubAccountClientBool(c, "enable")
			if oldEnable != enable {
				c["enable"] = enable
				clients[index] = c
				changed = true
			}
			refs = append(refs, subClientRef{Protocol: string(inbound.Protocol), Tag: inbound.Tag, Email: getSubAccountClientString(c, "email"), Client: c})
		}
		if changed {
			settings["clients"] = clients
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return nil, err
			}
			if err := tx.Model(model.Inbound{}).Where("id = ?", inbound.Id).Update("settings", string(modifiedSettings)).Error; err != nil {
				return nil, err
			}
		}
	}
	return refs, nil
}

func (s *SubAccountService) StartByClientTraffics(tx *gorm.DB, traffics []*xray.ClientTraffic) (int64, error) {
	emails := make([]string, 0)
	for _, traffic := range traffics {
		if traffic.Up+traffic.Down > 0 && traffic.Email != "" {
			emails = append(emails, traffic.Email)
		}
	}
	if len(emails) == 0 {
		return 0, nil
	}

	var rows []struct{ SubId string }
	if err := tx.Model(xray.ClientTraffic{}).
		Select("DISTINCT sub_id").
		Where("email IN ? AND sub_id <> ?", emails, "").
		Find(&rows).Error; err != nil {
		return 0, err
	}
	subIds := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.SubId != "" {
			subIds = append(subIds, row.SubId)
		}
	}
	if len(subIds) == 0 {
		return 0, nil
	}

	result := tx.Model(model.SubAccount{}).
		Where("sub_id IN ? AND start_time = ? AND enable = ?", subIds, 0, true).
		Update("start_time", time.Now().Unix()*1000)
	return result.RowsAffected, result.Error
}

func (s *SubAccountService) DisableInvalid(tx *gorm.DB) (bool, int64, error) {
	accounts := make([]*model.SubAccount, 0)
	if err := tx.Model(model.SubAccount{}).Where("enable = ?", true).Find(&accounts).Error; err != nil {
		return false, 0, err
	}
	now := time.Now().Unix() * 1000
	disabledIds := make([]int, 0)
	disabledSubIds := make([]string, 0)
	refs := make([]subClientRef, 0)
	for _, account := range accounts {
		up, down, err := getSubAccountTraffic(tx, account.SubId)
		if err != nil {
			return false, 0, err
		}
		if !subAccountLimitReached(account, up, down, now) {
			continue
		}
		disabledIds = append(disabledIds, account.Id)
		disabledSubIds = append(disabledSubIds, account.SubId)
		accountRefs, err := s.setSubAccountClientsEnable(tx, account.SubId, false)
		if err != nil {
			return false, 0, err
		}
		refs = append(refs, accountRefs...)
	}
	if len(disabledIds) == 0 {
		return false, 0, nil
	}
	if err := tx.Model(model.SubAccount{}).Where("id IN ?", disabledIds).Update("enable", false).Error; err != nil {
		return false, 0, err
	}
	if err := tx.Model(xray.ClientTraffic{}).Where("sub_id IN ?", disabledSubIds).Update("enable", false).Error; err != nil {
		return false, 0, err
	}

	needRestart := false
	if p != nil && len(refs) > 0 {
		var xrayAPI xray.XrayAPI
		if err := xrayAPI.Init(p.GetAPIPort()); err != nil {
			return true, int64(len(disabledIds)), nil
		}
		for _, ref := range refs {
			if ref.Email == "" {
				needRestart = true
				continue
			}
			err := xrayAPI.RemoveUser(ref.Tag, ref.Email)
			if err == nil {
				logger.Debug("Sub account client disabled by api:", ref.Email)
			} else if strings.Contains(err.Error(), fmt.Sprintf("User %s not found.", ref.Email)) {
				logger.Debug("Sub account client is already disabled:", ref.Email)
			} else {
				logger.Debug("Error in disabling sub account client by api:", err)
				needRestart = true
			}
		}
		xrayAPI.Close()
	}

	return needRestart, int64(len(disabledIds)), nil
}

func (s *SubAccountService) GetSubscriptionTraffic(subId string) (*xray.ClientTraffic, bool, error) {
	db := database.GetDB()
	account := &model.SubAccount{}
	err := db.Model(model.SubAccount{}).Where("sub_id = ?", subId).First(account).Error
	if database.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	up, down, err := getSubAccountTraffic(db, account.SubId)
	if err != nil {
		return nil, true, err
	}
	traffic := &xray.ClientTraffic{
		SubId:      account.SubId,
		Enable:     account.Enable,
		Email:      account.SubId,
		Up:         up,
		Down:       down,
		Total:      account.Total,
		ExpiryTime: subAccountExpiryTime(account),
	}
	if subAccountLimitReached(account, up, down, time.Now().Unix()*1000) {
		traffic.Enable = false
	}
	return traffic, true, nil
}
