package service

import (
	"errors"
	"sort"
	"time"

	"github.com/pardisontop/pardis-ui/database"
	"github.com/pardisontop/pardis-ui/database/model"
	"github.com/pardisontop/pardis-ui/util/common"
	"github.com/pardisontop/pardis-ui/xray"

	"gorm.io/gorm"
)

const (
	clientSessionIdleTimeout = 2 * time.Minute
	defaultAnalyticsWindow   = 24 * time.Hour
	maxAnalyticsSessions     = 50
	maxAnalyticsPoints       = 160
)

var trackedAppNames = []string{"telegram", "whatsapp", "instagram", "youtube", "x"}

type ClientAnalyticsRequest struct {
	Email     string `json:"email" form:"email"`
	InboundId int    `json:"inboundId" form:"inboundId"`
	SubId     string `json:"subId" form:"subId"`
	Since     int64  `json:"since" form:"since"`
}

type ClientUsagePoint struct {
	RecordedAt int64 `json:"recordedAt"`
	Up         int64 `json:"up"`
	Down       int64 `json:"down"`
}

type ClientAppUsageSummary struct {
	App  string `json:"app"`
	Up   int64  `json:"up"`
	Down int64  `json:"down"`
}

type ClientAnalyticsReport struct {
	Email                string                            `json:"email"`
	InboundId            int                               `json:"inboundId"`
	SubId                string                            `json:"subId"`
	Since                int64                             `json:"since"`
	Now                  int64                             `json:"now"`
	TotalUp              int64                             `json:"totalUp"`
	TotalDown            int64                             `json:"totalDown"`
	Sessions             []model.ClientConnectionSession   `json:"sessions"`
	Samples              []ClientUsagePoint                `json:"samples"`
	Apps                 []ClientAppUsageSummary           `json:"apps"`
	AppTrackingAvailable bool                              `json:"appTrackingAvailable"`
	AppTrackingNote      string                            `json:"appTrackingNote"`
}

type ClientAnalyticsService struct{}

func (s *ClientAnalyticsService) RecordTraffic(tx *gorm.DB, traffics []*xray.ClientTraffic) error {
	nowMs := time.Now().UnixMilli()
	if err := s.closeIdleSessions(tx, nowMs-int64(clientSessionIdleTimeout/time.Millisecond)); err != nil {
		return err
	}

	if len(traffics) == 0 {
		return nil
	}

	deltaByEmail := make(map[string]*xray.ClientTraffic, len(traffics))
	emails := make([]string, 0, len(traffics))
	for _, traffic := range traffics {
		if traffic == nil || traffic.Email == "" || traffic.Up+traffic.Down <= 0 {
			continue
		}
		if _, ok := deltaByEmail[traffic.Email]; !ok {
			emails = append(emails, traffic.Email)
			deltaByEmail[traffic.Email] = &xray.ClientTraffic{Email: traffic.Email}
		}
		deltaByEmail[traffic.Email].Up += traffic.Up
		deltaByEmail[traffic.Email].Down += traffic.Down
	}
	if len(emails) == 0 {
		return nil
	}

	dbClientTraffics := make([]*xray.ClientTraffic, 0, len(emails))
	for i := 0; i < len(emails); i += safeBatchSize {
		end := i + safeBatchSize
		if end > len(emails) {
			end = len(emails)
		}

		batchClientTraffics := make([]*xray.ClientTraffic, 0, end-i)
		if err := tx.Model(xray.ClientTraffic{}).Where("email IN ?", emails[i:end]).Find(&batchClientTraffics).Error; err != nil {
			return err
		}
		dbClientTraffics = append(dbClientTraffics, batchClientTraffics...)
	}

	samples := make([]model.ClientUsageSample, 0, len(dbClientTraffics))
	for _, dbTraffic := range dbClientTraffics {
		delta, ok := deltaByEmail[dbTraffic.Email]
		if !ok {
			continue
		}
		up, down := normalizedDelta(delta.Up, delta.Down)
		if up+down <= 0 {
			continue
		}

		samples = append(samples, model.ClientUsageSample{
			InboundId:  dbTraffic.InboundId,
			SubId:      dbTraffic.SubId,
			Email:      dbTraffic.Email,
			RecordedAt: nowMs,
			Up:         up,
			Down:       down,
		})
		if err := s.addTrafficToSession(tx, dbTraffic, up, down, nowMs); err != nil {
			return err
		}
	}

	if len(samples) == 0 {
		return nil
	}
	return tx.Create(&samples).Error
}

func (s *ClientAnalyticsService) GetClientReport(req ClientAnalyticsRequest) (*ClientAnalyticsReport, error) {
	if req.Email == "" && req.SubId == "" {
		return nil, common.NewError("client email or sub id is required")
	}

	nowMs := time.Now().UnixMilli()
	if req.Since <= 0 || req.Since > nowMs {
		req.Since = nowMs - int64(defaultAnalyticsWindow/time.Millisecond)
	}

	db := database.GetDB()
	report := &ClientAnalyticsReport{
		Email:     req.Email,
		InboundId: req.InboundId,
		SubId:     req.SubId,
		Since:     req.Since,
		Now:       nowMs,
	}

	totals, err := s.getUsageTotals(db, req)
	if err != nil {
		return nil, err
	}
	report.TotalUp = totals.Up
	report.TotalDown = totals.Down

	sessions, err := s.getSessions(db, req)
	if err != nil {
		return nil, err
	}
	report.Sessions = sessions

	samples, err := s.getUsagePoints(db, req, nowMs)
	if err != nil {
		return nil, err
	}
	report.Samples = samples

	apps, appTrackingAvailable, err := s.getAppUsage(db, req)
	if err != nil {
		return nil, err
	}
	report.Apps = apps
	report.AppTrackingAvailable = appTrackingAvailable
	if !appTrackingAvailable {
		report.AppTrackingNote = "Per-app usage needs per-user app counters from Xray routing stats."
	}

	return report, nil
}

func (s *ClientAnalyticsService) closeIdleSessions(tx *gorm.DB, cutoffMs int64) error {
	return tx.Model(&model.ClientConnectionSession{}).
		Where("active = ? AND last_seen_at < ?", true, cutoffMs).
		Updates(map[string]interface{}{
			"active":   false,
			"end_time": gorm.Expr("last_seen_at"),
		}).Error
}

func (s *ClientAnalyticsService) addTrafficToSession(tx *gorm.DB, dbTraffic *xray.ClientTraffic, up int64, down int64, nowMs int64) error {
	var session model.ClientConnectionSession
	err := tx.Model(&model.ClientConnectionSession{}).
		Where("email = ? AND inbound_id = ? AND active = ?", dbTraffic.Email, dbTraffic.InboundId, true).
		Order("last_seen_at DESC").
		First(&session).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		session = model.ClientConnectionSession{
			InboundId:  dbTraffic.InboundId,
			SubId:      dbTraffic.SubId,
			Email:      dbTraffic.Email,
			StartTime:  nowMs,
			EndTime:    nowMs,
			LastSeenAt: nowMs,
			Up:         up,
			Down:       down,
			Active:     true,
		}
		return tx.Create(&session).Error
	}

	return tx.Model(&session).Updates(map[string]interface{}{
		"sub_id":       dbTraffic.SubId,
		"end_time":     nowMs,
		"last_seen_at": nowMs,
		"up":           gorm.Expr("up + ?", up),
		"down":         gorm.Expr("down + ?", down),
	}).Error
}

func (s *ClientAnalyticsService) getUsageTotals(db *gorm.DB, req ClientAnalyticsRequest) (*ClientUsagePoint, error) {
	total := &ClientUsagePoint{}
	err := applyClientAnalyticsFilters(db.Model(&model.ClientUsageSample{}), req).
		Where("recorded_at >= ?", req.Since).
		Select("COALESCE(SUM(up), 0) as up, COALESCE(SUM(down), 0) as down").
		Scan(total).Error
	return total, err
}

func (s *ClientAnalyticsService) getSessions(db *gorm.DB, req ClientAnalyticsRequest) ([]model.ClientConnectionSession, error) {
	sessions := make([]model.ClientConnectionSession, 0)
	err := applyClientAnalyticsFilters(db.Model(&model.ClientConnectionSession{}), req).
		Where("(end_time >= ? OR active = ?)", req.Since, true).
		Order("start_time DESC").
		Limit(maxAnalyticsSessions).
		Find(&sessions).Error
	if err != nil {
		return nil, err
	}
	reverseSessions(sessions)
	return sessions, nil
}

func (s *ClientAnalyticsService) getUsagePoints(db *gorm.DB, req ClientAnalyticsRequest, nowMs int64) ([]ClientUsagePoint, error) {
	rawSamples := make([]model.ClientUsageSample, 0)
	err := applyClientAnalyticsFilters(db.Model(&model.ClientUsageSample{}), req).
		Where("recorded_at >= ?", req.Since).
		Order("recorded_at ASC").
		Find(&rawSamples).Error
	if err != nil {
		return nil, err
	}
	if len(rawSamples) == 0 {
		return []ClientUsagePoint{}, nil
	}

	windowMs := nowMs - req.Since
	if windowMs <= 0 || len(rawSamples) <= maxAnalyticsPoints {
		points := make([]ClientUsagePoint, 0, len(rawSamples))
		for _, sample := range rawSamples {
			points = append(points, ClientUsagePoint{RecordedAt: sample.RecordedAt, Up: sample.Up, Down: sample.Down})
		}
		return points, nil
	}

	bucketMs := windowMs / maxAnalyticsPoints
	if bucketMs < 1 {
		bucketMs = 1
	}
	pointsByBucket := make(map[int64]*ClientUsagePoint)
	for _, sample := range rawSamples {
		bucket := req.Since + ((sample.RecordedAt - req.Since) / bucketMs * bucketMs)
		point, ok := pointsByBucket[bucket]
		if !ok {
			point = &ClientUsagePoint{RecordedAt: bucket}
			pointsByBucket[bucket] = point
		}
		point.Up += sample.Up
		point.Down += sample.Down
	}

	points := make([]ClientUsagePoint, 0, len(pointsByBucket))
	for _, point := range pointsByBucket {
		points = append(points, *point)
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].RecordedAt < points[j].RecordedAt
	})
	return points, nil
}

func (s *ClientAnalyticsService) getAppUsage(db *gorm.DB, req ClientAnalyticsRequest) ([]ClientAppUsageSummary, bool, error) {
	rows := make([]ClientAppUsageSummary, 0)
	err := applyClientAnalyticsFilters(db.Model(&model.ClientAppUsage{}), req).
		Where("recorded_at >= ?", req.Since).
		Select("app, COALESCE(SUM(up), 0) as up, COALESCE(SUM(down), 0) as down").
		Group("app").
		Scan(&rows).Error
	if err != nil {
		return nil, false, err
	}

	rowsByApp := make(map[string]ClientAppUsageSummary, len(rows))
	trackingAvailable := false
	for _, row := range rows {
		rowsByApp[row.App] = row
		if row.Up+row.Down > 0 {
			trackingAvailable = true
		}
	}

	apps := make([]ClientAppUsageSummary, 0, len(trackedAppNames))
	for _, name := range trackedAppNames {
		row, ok := rowsByApp[name]
		if !ok {
			row = ClientAppUsageSummary{App: name}
		}
		apps = append(apps, row)
	}
	return apps, trackingAvailable, nil
}

func applyClientAnalyticsFilters(db *gorm.DB, req ClientAnalyticsRequest) *gorm.DB {
	if req.SubId != "" {
		db = db.Where("sub_id = ?", req.SubId)
	} else {
		db = db.Where("email = ?", req.Email)
	}
	if req.InboundId > 0 {
		db = db.Where("inbound_id = ?", req.InboundId)
	}
	return db
}

func normalizedDelta(up int64, down int64) (int64, int64) {
	if up < 0 {
		up = 0
	}
	if down < 0 {
		down = 0
	}
	return up, down
}

func reverseSessions(sessions []model.ClientConnectionSession) {
	for i, j := 0, len(sessions)-1; i < j; i, j = i+1, j-1 {
		sessions[i], sessions[j] = sessions[j], sessions[i]
	}
}
