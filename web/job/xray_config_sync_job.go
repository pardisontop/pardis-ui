package job

import (
	"github.com/pardisontop/pardis-ui/logger"
	"github.com/pardisontop/pardis-ui/web/service"
)

// XrayConfigSyncJob detects inbound setting changes in the shared DB and
// restarts Xray when needed. This allows nodes to pick up config changes
// made via the master panel without manual intervention.
type XrayConfigSyncJob struct {
	xrayService service.XrayService
}

func NewXrayConfigSyncJob() *XrayConfigSyncJob {
	return new(XrayConfigSyncJob)
}

func (j *XrayConfigSyncJob) Run() {
	// RestartXray(false) reads the current config from DB and restarts Xray
	// only if the config has actually changed — a no-op when nothing changed.
	if err := j.xrayService.RestartXray(false); err != nil {
		logger.Warning("xray config sync failed:", err)
	}
}
