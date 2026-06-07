package xray

type ClientTraffic struct {
	Id         int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	InboundId  int    `json:"inboundId" form:"inboundId"`
	SubId          string `json:"subId" form:"subId" gorm:"size:191;index"`
	Enable         bool   `json:"enable" form:"enable"`
	TrackAnalytics bool   `json:"trackAnalytics" form:"trackAnalytics" gorm:"default:false;index"`
	Email          string `json:"email" form:"email" gorm:"size:191;unique"`
	Up             int64  `json:"up" form:"up"`
	Down           int64  `json:"down" form:"down"`
	ExpiryTime     int64  `json:"expiryTime" form:"expiryTime"`
	Total          int64  `json:"total" form:"total"`
	Reset          int    `json:"reset" form:"reset" gorm:"default:0"`
}
