// Package docker implements the sandbox runtime on the Docker Engine API.
// It is the only package allowed to import github.com/moby/moby (ADR-0005).
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
)

// Label keys used to name components. The sesim label wins; the compose
// labels cover the dev stack.
const (
	labelComponent        = "dev.sesim.component"
	labelComposeService   = "com.docker.compose.service"
	labelComposeContainer = "com.docker.compose.container-number"
)

// Client talks to the local Docker daemon.
type Client struct {
	api *client.Client
}

// New connects using the standard DOCKER_* environment variables.
func New() (*Client, error) {
	api, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("docker: client: %w", err)
	}
	return &Client{api: api}, nil
}

// Close releases the client's connections.
func (c *Client) Close() error { return c.api.Close() }

// Stats samples every running container carrying label (key=value).
// Containers are sampled concurrently; one failing container fails the call.
func (c *Client) Stats(ctx context.Context, label string) ([]sandbox.ResourceStats, error) {
	list, err := c.api.ContainerList(ctx, client.ContainerListOptions{
		Filters: make(client.Filters).Add("label", label).Add("status", "running"),
	})
	if err != nil {
		return nil, fmt.Errorf("docker: list containers: %w", err)
	}
	out := make([]sandbox.ResourceStats, len(list.Items))
	errs := make([]error, len(list.Items))
	var wg sync.WaitGroup
	for i, s := range list.Items {
		wg.Go(func() { out[i], errs[i] = c.sample(ctx, s) })
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (c *Client) sample(ctx context.Context, s container.Summary) (sandbox.ResourceStats, error) {
	res, err := c.api.ContainerStats(ctx, s.ID, client.ContainerStatsOptions{})
	if err != nil {
		return sandbox.ResourceStats{}, fmt.Errorf("docker: stats %s: %w", s.ID[:12], err)
	}
	defer res.Body.Close()
	var st container.StatsResponse
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		return sandbox.ResourceStats{}, fmt.Errorf("docker: decode stats %s: %w", s.ID[:12], err)
	}
	// The quota can change at runtime (docker update), so inspect every sample.
	ins, err := c.api.ContainerInspect(ctx, s.ID, client.ContainerInspectOptions{})
	if err != nil {
		return sandbox.ResourceStats{}, fmt.Errorf("docker: inspect %s: %w", s.ID[:12], err)
	}
	return toResourceStats(s.Labels, st, ins.Container.HostConfig), nil
}

func toResourceStats(labels map[string]string, st container.StatsResponse, hc *container.HostConfig) sandbox.ResourceStats {
	r := sandbox.ResourceStats{
		Component:        componentName(labels),
		Replica:          1,
		At:               st.Read,
		CPUUsageSeconds:  float64(st.CPUStats.CPUUsage.TotalUsage) / 1e9,
		CPUPeriods:       st.CPUStats.ThrottlingData.Periods,
		ThrottledPeriods: st.CPUStats.ThrottlingData.ThrottledPeriods,
		ThrottledSeconds: float64(st.CPUStats.ThrottlingData.ThrottledTime) / 1e9,
		MemoryLimitBytes: st.MemoryStats.Limit,
		Pids:             st.PidsStats.Current,
	}
	if n, err := strconv.Atoi(labels[labelComposeContainer]); err == nil && n > 0 {
		r.Replica = n
	}
	// Same working-set definition as `docker stats`: exclude reclaimable page cache.
	r.MemoryBytes = st.MemoryStats.Usage
	if inactive, ok := st.MemoryStats.Stats["inactive_file"]; ok && inactive < r.MemoryBytes {
		r.MemoryBytes -= inactive
	}
	for _, n := range st.Networks {
		r.NetRxBytes += n.RxBytes
		r.NetTxBytes += n.TxBytes
	}
	if hc != nil {
		switch {
		case hc.NanoCPUs > 0:
			r.CPUQuotaCores = float64(hc.NanoCPUs) / 1e9
		case hc.CPUQuota > 0 && hc.CPUPeriod > 0:
			r.CPUQuotaCores = float64(hc.CPUQuota) / float64(hc.CPUPeriod)
		}
	}
	return r
}

func componentName(labels map[string]string) string {
	for _, k := range []string{labelComponent, labelComposeService} {
		if v := labels[k]; v != "" {
			return v
		}
	}
	return "unknown"
}
