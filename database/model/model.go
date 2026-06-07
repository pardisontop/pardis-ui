package model

import (
	"fmt"

	"github.com/pardisontop/pardis-ui/util/json_util"
	"github.com/pardisontop/pardis-ui/xray"
)

type Protocol string

const (
	VMess       Protocol = "vmess"
	VLESS       Protocol = "vless"
	Dokodemo    Protocol = "Dokodemo-door"
	Http        Protocol = "http"
	Trojan      Protocol = "trojan"
	Shadowsocks Protocol = "shadowsocks"
)

type User struct {
	Id       int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Inbound struct {
	Id          int                  `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	UserId      int                  `json:"-"`
	Up          int64                `json:"up" form:"up"`
	Down        int64                `json:"down" form:"down"`
	Total       int64                `json:"total" form:"total"`
	Remark      string               `json:"remark" form:"remark"`
	Enable      bool                 `json:"enable" form:"enable"`
	ExpiryTime  int64                `json:"expiryTime" form:"expiryTime"`
	ClientStats []xray.ClientTraffic `gorm:"foreignKey:InboundId;references:Id" json:"clientStats" form:"clientStats"`

	// config part
	Listen         string   `json:"listen" form:"listen"`
	Port           int      `json:"port" form:"port"`
	Protocol       Protocol `json:"protocol" form:"protocol"`
	Settings       string   `json:"settings" form:"settings"`
	StreamSettings string   `json:"streamSettings" form:"streamSettings"`
	Tag            string   `json:"tag" form:"tag" gorm:"size:191;unique"`
	Sniffing       string   `json:"sniffing" form:"sniffing"`
}

func (i *Inbound) GenXrayInboundConfig() *xray.InboundConfig {
	listen := i.Listen
	if listen != "" {
		listen = fmt.Sprintf("\"%v\"", listen)
	}
	return &xray.InboundConfig{
		Listen:         json_util.RawMessage(listen),
		Port:           i.Port,
		Protocol:       string(i.Protocol),
		Settings:       json_util.RawMessage(i.Settings),
		StreamSettings: json_util.RawMessage(i.StreamSettings),
		Tag:            i.Tag,
		Sniffing:       json_util.RawMessage(i.Sniffing),
	}
}

type Setting struct {
	Id    int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}

type SubAccount struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Remark     string `json:"remark" form:"remark"`
	SubId      string `json:"subId" form:"subId" gorm:"size:191;uniqueIndex"`
	Total      int64  `json:"total" form:"total"`
	Duration   int64  `json:"duration" form:"duration"`
	StartTime  int64  `json:"startTime" form:"startTime"`
	Enable     bool   `json:"enable" form:"enable"`
	InboundIds string `json:"inboundIds" form:"inboundIds"`
}

type ClientConnectionSession struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	InboundId  int    `json:"inboundId" form:"inboundId" gorm:"index:idx_client_session_lookup"`
	SubId      string `json:"subId" form:"subId" gorm:"size:191;index"`
	Email      string `json:"email" form:"email" gorm:"size:191;index:idx_client_session_lookup"`
	StartTime  int64  `json:"startTime" form:"startTime" gorm:"index"`
	EndTime    int64  `json:"endTime" form:"endTime"`
	LastSeenAt int64  `json:"lastSeenAt" form:"lastSeenAt" gorm:"index"`
	Up         int64  `json:"up" form:"up"`
	Down       int64  `json:"down" form:"down"`
	Active     bool   `json:"active" form:"active" gorm:"index"`
}

type ClientUsageSample struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	InboundId  int    `json:"inboundId" form:"inboundId" gorm:"index:idx_client_usage_lookup"`
	SubId      string `json:"subId" form:"subId" gorm:"size:191;index"`
	Email      string `json:"email" form:"email" gorm:"size:191;index:idx_client_usage_lookup"`
	RecordedAt int64  `json:"recordedAt" form:"recordedAt" gorm:"index:idx_client_usage_lookup"`
	Up         int64  `json:"up" form:"up"`
	Down       int64  `json:"down" form:"down"`
}

type ClientAppUsage struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	InboundId  int    `json:"inboundId" form:"inboundId" gorm:"index:idx_client_app_usage_lookup"`
	SubId      string `json:"subId" form:"subId" gorm:"size:191;index"`
	Email      string `json:"email" form:"email" gorm:"size:191;index:idx_client_app_usage_lookup"`
	App        string `json:"app" form:"app" gorm:"size:32;index:idx_client_app_usage_lookup"`
	RecordedAt int64  `json:"recordedAt" form:"recordedAt" gorm:"index:idx_client_app_usage_lookup"`
	Up         int64  `json:"up" form:"up"`
	Down       int64  `json:"down" form:"down"`
}

type ClientSessionDestination struct {
	Id          int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	SessionId   int    `json:"sessionId" form:"sessionId" gorm:"index:idx_client_destination_lookup"`
	InboundId   int    `json:"inboundId" form:"inboundId" gorm:"index"`
	SubId       string `json:"subId" form:"subId" gorm:"size:191;index"`
	Email       string `json:"email" form:"email" gorm:"size:191;index:idx_client_destination_lookup"`
	Network     string `json:"network" form:"network" gorm:"size:16"`
	Address     string `json:"address" form:"address" gorm:"size:191;index:idx_client_destination_lookup"`
	Port        int    `json:"port" form:"port"`
	Destination string `json:"destination" form:"destination" gorm:"size:255"`
	FirstSeenAt int64  `json:"firstSeenAt" form:"firstSeenAt" gorm:"index"`
	LastSeenAt  int64  `json:"lastSeenAt" form:"lastSeenAt" gorm:"index"`
	Count       int    `json:"count" form:"count"`
}

type Client struct {
	ID         string `json:"id"`
	Password   string `json:"password"`
	Security   string `json:"security,omitempty"`
	Method     string `json:"method,omitempty"`
	Flow       string `json:"flow"`
	Email      string `json:"email"`
	TotalGB    int64  `json:"totalGB" form:"totalGB"`
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"`
	Enable     bool   `json:"enable" form:"enable"`
	TgID           string `json:"tgId" form:"tgId"`
	SubID          string `json:"subId" form:"subId"`
	TrackAnalytics bool   `json:"trackAnalytics" form:"trackAnalytics"`
	Reset          int    `json:"reset" form:"reset"`
}

type VLESSSettings struct {
	Clients    []Client `json:"clients"`
	Decryption string   `json:"decryption"`
	Encryption string   `json:"encryption"`
	Fallbacks  []any    `json:"fallbacks"`
}
