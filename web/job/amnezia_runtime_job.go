package job

import (
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/websocket"
)

// AmneziaRuntimeJob collects live AmneziaWG transfer counters, handshakes,
// online state, and enforces peer expiry/traffic limits.
type AmneziaRuntimeJob struct {
	amneziaService *service.AmneziaService
}

func NewAmneziaRuntimeJob() *AmneziaRuntimeJob {
	return &AmneziaRuntimeJob{
		amneziaService: service.NewAmneziaService(),
	}
}

func (j *AmneziaRuntimeJob) Run() {
	snapshot, err := j.amneziaService.CollectRuntimeStats()
	if err != nil {
		logger.Warning("AmneziaRuntimeJob: failed to collect runtime stats:", err)
		return
	}
	if websocket.HasClients() {
		websocket.BroadcastAmnezia(snapshot)
	}
}
