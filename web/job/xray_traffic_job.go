package job

import (
	"github.com/pardisontop/pardis-ui/logger"
	"github.com/pardisontop/pardis-ui/web/service"
)

type XrayTrafficJob struct {
	xrayService    service.XrayService
	inboundService service.InboundService
}

func NewXrayTrafficJob() *XrayTrafficJob {
	return new(XrayTrafficJob)
}

func (j *XrayTrafficJob) Run() {
	if !j.xrayService.IsXrayRunning() {
		return
	}

	traffics, clientTraffics, err := j.xrayService.GetXrayTraffic()
	if err != nil {
		logger.Warning("get xray traffic failed:", err)
		return
	}
	err, needRestart := j.inboundService.AddTraffic(traffics, clientTraffics)
	if err != nil {
		logger.Warning("add traffic failed:", err)
	}
	clientAnalyticsService := service.ClientAnalyticsService{}
	if err := clientAnalyticsService.RecordAccessLogDestinations(); err != nil {
		logger.Warning("record client destinations failed:", err)
	}
	if needRestart {
		j.xrayService.SetToNeedRestart()
	}
}
