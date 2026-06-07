package job

import (
	"sync"

	"github.com/pardisontop/pardis-ui/logger"
	"github.com/pardisontop/pardis-ui/web/service"
	"github.com/pardisontop/pardis-ui/xray"
)

type XrayTrafficJob struct {
	xrayService    service.XrayService
	inboundService service.InboundService

	// pending holds traffic that couldn't be written to DB on a previous tick.
	// It is merged into the next attempt so no traffic is lost during DB outages.
	mu                    sync.Mutex
	pendingTraffics       []*xray.Traffic
	pendingClientTraffics []*xray.ClientTraffic
}

func NewXrayTrafficJob() *XrayTrafficJob {
	return new(XrayTrafficJob)
}

func (j *XrayTrafficJob) Run() {
	if !j.xrayService.IsXrayRunning() {
		return
	}

	// Fetch AND reset Xray counters immediately so stats don't accumulate inside Xray.
	// If the DB write fails we keep the data ourselves in pendingTraffics.
	newTraffics, newClientTraffics, err := j.xrayService.GetXrayTraffic()
	if err != nil {
		logger.Warning("get xray traffic failed:", err)
		return
	}

	// Merge any previously buffered (unwritten) traffic with the fresh batch.
	j.mu.Lock()
	merged := mergeTraffics(j.pendingTraffics, newTraffics)
	mergedClients := mergeClientTraffics(j.pendingClientTraffics, newClientTraffics)
	j.mu.Unlock()

	err, needRestart := j.inboundService.AddTraffic(merged, mergedClients)
	if err != nil {
		// DB write failed — buffer the merged data so the next tick can retry.
		logger.Warning("add traffic failed, buffering for retry:", err)
		j.mu.Lock()
		j.pendingTraffics = merged
		j.pendingClientTraffics = mergedClients
		j.mu.Unlock()
		return
	}

	// Success — clear the pending buffer.
	j.mu.Lock()
	j.pendingTraffics = nil
	j.pendingClientTraffics = nil
	j.mu.Unlock()

	clientAnalyticsService := service.ClientAnalyticsService{}
	if err := clientAnalyticsService.RecordAccessLogDestinations(); err != nil {
		logger.Warning("record client destinations failed:", err)
	}
	if needRestart {
		j.xrayService.SetToNeedRestart()
	}
}

// mergeTraffics combines two inbound/outbound traffic slices by tag, summing bytes.
func mergeTraffics(a, b []*xray.Traffic) []*xray.Traffic {
	if len(a) == 0 {
		return b
	}
	index := make(map[string]*xray.Traffic, len(a))
	for _, t := range a {
		cp := *t
		index[t.Tag] = &cp
	}
	for _, t := range b {
		if existing, ok := index[t.Tag]; ok {
			existing.Up += t.Up
			existing.Down += t.Down
		} else {
			cp := *t
			index[t.Tag] = &cp
		}
	}
	result := make([]*xray.Traffic, 0, len(index))
	for _, t := range index {
		result = append(result, t)
	}
	return result
}

// mergeClientTraffics combines two per-user traffic slices by email, summing bytes.
func mergeClientTraffics(a, b []*xray.ClientTraffic) []*xray.ClientTraffic {
	if len(a) == 0 {
		return b
	}
	index := make(map[string]*xray.ClientTraffic, len(a))
	for _, t := range a {
		cp := *t
		index[t.Email] = &cp
	}
	for _, t := range b {
		if existing, ok := index[t.Email]; ok {
			existing.Up += t.Up
			existing.Down += t.Down
		} else {
			cp := *t
			index[t.Email] = &cp
		}
	}
	result := make([]*xray.ClientTraffic, 0, len(index))
	for _, t := range index {
		result = append(result, t)
	}
	return result
}
