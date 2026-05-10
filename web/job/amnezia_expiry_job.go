package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/service"
)

// AmneziaExpiryJob periodically checks for expired AmneziaWG peers
// and pauses them, removing them from the active server config.
type AmneziaExpiryJob struct {
	amneziaService *service.AmneziaService
	interval       time.Duration
	stopChan       chan struct{}
}

// NewAmneziaExpiryJob creates a new expiry checker job.
// Default interval is 1 minute.
func NewAmneziaExpiryJob() *AmneziaExpiryJob {
	return &AmneziaExpiryJob{
		amneziaService: service.NewAmneziaService(),
		interval:       time.Minute,
		stopChan:       make(chan struct{}),
	}
}

// Run starts the periodic expiry check.
func (j *AmneziaExpiryJob) Run() {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := j.checkExpiredPeers(); err != nil {
				logger.Warning("AmneziaExpiryJob: failed to pause expired peers:", err)
			}
		case <-j.stopChan:
			return
		}
	}
}

// Stop stops the periodic expiry check.
func (j *AmneziaExpiryJob) Stop() {
	close(j.stopChan)
}

func (j *AmneziaExpiryJob) checkExpiredPeers() error {
	return j.amneziaService.PauseExpiredPeers()
}