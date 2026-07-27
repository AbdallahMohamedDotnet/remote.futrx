package serverinfo

import (
	"context"
	"time"
)

type Collector interface {
	Collect(ctx context.Context, now time.Time) Snapshot
}

type Service struct {
	collector     Collector
	appVersion    string
	dataPath      string
	workspacePath string
	startedAt     time.Time
}

func New(collector Collector, appVersion, dataPath, workspacePath string) *Service {
	return &Service{
		collector:     collector,
		appVersion:    appVersion,
		dataPath:      dataPath,
		workspacePath: workspacePath,
		startedAt:     time.Now(),
	}
}

func (s *Service) Collect(ctx context.Context) Info {
	now := time.Now()
	snapshot := s.collector.Collect(ctx, now)
	snapshot.Host.ServiceUptimeSec = int64(now.Sub(s.startedAt).Seconds())
	snapshot.Host.AppVersion = s.appVersion
	snapshot.Host.DataPath = s.dataPath
	snapshot.Host.WorkspacePath = s.workspacePath
	return Info{
		CollectedAt: now.Unix(),
		Host:        snapshot.Host,
		CPU:         snapshot.CPU,
		Memory:      snapshot.Memory,
		Storage:     snapshot.Storage,
		Network:     snapshot.Network,
		Process:     snapshot.Process,
	}
}
