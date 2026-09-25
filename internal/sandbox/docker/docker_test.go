package docker

import (
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
)

func TestToResourceStats(t *testing.T) {
	at := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	st := container.StatsResponse{
		Read: at,
		CPUStats: container.CPUStats{
			CPUUsage:       container.CPUUsage{TotalUsage: 2_500_000_000},
			ThrottlingData: container.ThrottlingData{Periods: 100, ThrottledPeriods: 40, ThrottledTime: 1_200_000_000},
		},
		MemoryStats: container.MemoryStats{Usage: 300, Limit: 1000, Stats: map[string]uint64{"inactive_file": 100}},
		PidsStats:   container.PidsStats{Current: 7},
		Networks: map[string]container.NetworkStats{
			"play":    {RxBytes: 10, TxBytes: 20},
			"ingress": {RxBytes: 1, TxBytes: 2},
		},
	}
	tests := []struct {
		name      string
		labels    map[string]string
		hc        *container.HostConfig
		wantComp  string
		wantRep   int
		wantQuota float64
	}{
		{"compose, nano cpus", map[string]string{labelComposeService: "postgres", labelComposeContainer: "2"},
			&container.HostConfig{Resources: container.Resources{NanoCPUs: 500_000_000}}, "postgres", 2, 0.5},
		{"sesim label wins, cfs quota", map[string]string{labelComponent: "db", labelComposeService: "postgres"},
			&container.HostConfig{Resources: container.Resources{CPUQuota: 25_000, CPUPeriod: 100_000}}, "db", 1, 0.25},
		{"no labels, unlimited", nil, &container.HostConfig{}, "unknown", 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := toResourceStats(tt.labels, st, tt.hc)
			if r.Component != tt.wantComp || r.Replica != tt.wantRep || r.CPUQuotaCores != tt.wantQuota {
				t.Errorf("component/replica/quota = %s/%d/%v, want %s/%d/%v",
					r.Component, r.Replica, r.CPUQuotaCores, tt.wantComp, tt.wantRep, tt.wantQuota)
			}
			if r.CPUUsageSeconds != 2.5 || r.ThrottledSeconds != 1.2 || r.ThrottledPeriods != 40 || r.CPUPeriods != 100 {
				t.Errorf("cpu = %+v", r)
			}
			if r.MemoryBytes != 200 || r.MemoryLimitBytes != 1000 || r.Pids != 7 {
				t.Errorf("memory/pids = %d/%d/%d", r.MemoryBytes, r.MemoryLimitBytes, r.Pids)
			}
			if r.NetRxBytes != 11 || r.NetTxBytes != 22 || !r.At.Equal(at) {
				t.Errorf("net/at = %d/%d/%s", r.NetRxBytes, r.NetTxBytes, r.At)
			}
		})
	}
}
