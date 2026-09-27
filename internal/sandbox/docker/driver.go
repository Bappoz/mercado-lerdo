package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
)

// Labels carried by every resource the driver creates.
const (
	LabelManaged  = "dev.sesim.managed"
	LabelRun      = "dev.sesim.run"
	labelReplica  = "dev.sesim.replica"
	labelCache    = "dev.sesim.cache"
	labelOneShot  = "dev.sesim.oneshot"
	managedFilter = LabelManaged + "=true"
)

// resourceName is the Docker name of a run-scoped resource.
func resourceName(run sandbox.RunID, parts ...string) string {
	return "sesim-" + string(run) + "-" + strings.Join(parts, "-")
}

func volumeName(run sandbox.RunID, spec sandbox.Spec, name string) string {
	for _, v := range spec.Volumes {
		if v.Name == name && v.Cache {
			return "sesim-cache-" + name
		}
	}
	return resourceName(run, name)
}

func runLabels(run sandbox.RunID) map[string]string {
	return map[string]string{LabelManaged: "true", LabelRun: string(run)}
}

// Create creates the networks, volumes and containers of spec. Nothing is
// started. On error, whatever was created stays labelled for Destroy/GC.
func (c *Client) Create(ctx context.Context, run sandbox.RunID, spec sandbox.Spec) error {
	if err := spec.Validate(); err != nil {
		return fmt.Errorf("docker: invalid sandbox spec: %w", err)
	}
	if err := c.requireImages(ctx, spec); err != nil {
		return err
	}
	for _, n := range spec.Networks {
		_, err := c.api.NetworkCreate(ctx, resourceName(run, n.Name), client.NetworkCreateOptions{
			Driver: "bridge", Internal: n.Internal, Labels: runLabels(run),
		})
		if err != nil {
			return fmt.Errorf("docker: create network %s: %w", n.Name, err)
		}
	}
	for _, v := range spec.Volumes {
		labels := runLabels(run)
		if v.Cache {
			labels = map[string]string{LabelManaged: "true", labelCache: v.Name}
		}
		// Creating an existing volume is a no-op, which is what cache volumes need.
		if _, err := c.api.VolumeCreate(ctx, client.VolumeCreateOptions{Name: volumeName(run, spec, v.Name), Labels: labels}); err != nil {
			return fmt.Errorf("docker: create volume %s: %w", v.Name, err)
		}
	}
	for _, ct := range spec.Containers {
		if _, err := c.createContainer(ctx, run, spec, ct, resourceName(run, ct.Component, strconv.Itoa(ct.Replica)), nil); err != nil {
			return err
		}
	}
	return nil
}

// requireImages makes sure every image is present. Third-party images are
// pinned by digest (Spec.Validate), so pulling them is reproducible; locally
// built sesim/* images cannot be pulled and must come from `just images`.
func (c *Client) requireImages(ctx context.Context, spec sandbox.Spec) error {
	var missing []string
	for _, ct := range spec.Containers {
		if slices.Contains(missing, ct.Image) {
			continue
		}
		if _, err := c.api.ImageInspect(ctx, ct.Image); err == nil {
			continue
		}
		if strings.HasPrefix(ct.Image, "sesim/") {
			missing = append(missing, ct.Image)
			continue
		}
		pull, err := c.api.ImagePull(ctx, ct.Image, client.ImagePullOptions{})
		if err == nil {
			err = pull.Wait(ctx)
		}
		if err != nil {
			return fmt.Errorf("docker: pull %s: %w", ct.Image, err)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("docker: images not available locally: %s (run `just images`)", strings.Join(missing, ", "))
	}
	return nil
}

func (c *Client) createContainer(ctx context.Context, run sandbox.RunID, spec sandbox.Spec, ct sandbox.Container, name string, extraLabels map[string]string) (string, error) {
	labels := runLabels(run)
	labels[labelComponent] = ct.Component
	labels[labelReplica] = strconv.Itoa(ct.Replica)
	for k, v := range extraLabels {
		labels[k] = v
	}
	env := make([]string, 0, len(ct.Env))
	for k, v := range ct.Env {
		env = append(env, k+"="+v)
	}
	slices.Sort(env)

	cfg := &container.Config{
		Image: ct.Image, Entrypoint: ct.Entrypoint, Cmd: ct.Cmd, Env: env, User: ct.User,
		Labels: labels, Hostname: ct.Component,
	}
	if h := ct.Health; h != nil {
		cfg.Healthcheck = &container.HealthConfig{
			Test: append([]string{"CMD"}, h.Cmd...), Interval: h.Interval, Timeout: h.Timeout, Retries: h.Retries,
		}
	}
	pids := ct.Resources.Pids
	hc := &container.HostConfig{
		Resources:      container.Resources{NanoCPUs: int64(ct.Resources.CPUs * 1e9), Memory: ct.Resources.Memory, PidsLimit: &pids},
		CapDrop:        []string{"ALL"},
		CapAdd:         ct.CapAdd,
		SecurityOpt:    []string{"no-new-privileges:true"},
		ReadonlyRootfs: ct.ReadOnly,
		ShmSize:        ct.ShmBytes,
		NetworkMode:    "none",
	}
	if ct.RestartOnCrash {
		// on-failure, not always: a container the platform stops stays stopped.
		hc.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyOnFailure, MaximumRetryCount: 10}
	}
	for _, m := range ct.Mounts {
		dm := mount.Mount{Target: m.Target, ReadOnly: m.ReadOnly}
		switch m.Type {
		case sandbox.VolumeMount:
			dm.Type, dm.Source = mount.TypeVolume, volumeName(run, spec, m.Source)
		case sandbox.BindMount:
			dm.Type, dm.Source = mount.TypeBind, m.Source
		case sandbox.TmpfsMount:
			dm.Type = mount.TypeTmpfs
			dm.TmpfsOptions = &mount.TmpfsOptions{SizeBytes: m.Bytes}
		}
		hc.Mounts = append(hc.Mounts, dm)
	}
	var netCfg *network.NetworkingConfig
	if len(ct.Networks) > 0 {
		hc.NetworkMode = container.NetworkMode(resourceName(run, ct.Networks[0]))
		netCfg = &network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{}}
		for _, n := range ct.Networks {
			netCfg.EndpointsConfig[resourceName(run, n)] = &network.EndpointSettings{Aliases: []string{ct.Component}}
		}
	}
	res, err := c.api.ContainerCreate(ctx, client.ContainerCreateOptions{Name: name, Config: cfg, HostConfig: hc, NetworkingConfig: netCfg})
	if err != nil {
		return "", fmt.Errorf("docker: create container %s: %w", name, err)
	}
	return res.ID, nil
}

// containers lists the run's containers, optionally of one component.
func (c *Client) containers(ctx context.Context, run sandbox.RunID, component string) ([]container.Summary, error) {
	f := make(client.Filters).Add("label", LabelRun+"="+string(run))
	if component != "" {
		f = f.Add("label", labelComponent+"="+component)
	}
	list, err := c.api.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: f})
	if err != nil {
		return nil, fmt.Errorf("docker: list containers: %w", err)
	}
	return list.Items, nil
}

// Start starts every replica of component.
func (c *Client) Start(ctx context.Context, run sandbox.RunID, component string) error {
	list, err := c.containers(ctx, run, component)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return fmt.Errorf("docker: run %s has no %s container", run, component)
	}
	for _, s := range list {
		if _, err := c.api.ContainerStart(ctx, s.ID, client.ContainerStartOptions{}); err != nil {
			return fmt.Errorf("docker: start %s: %w", component, err)
		}
	}
	return nil
}

// WaitHealthy waits until every replica of component is healthy (or running,
// when it has no health check). A container that exits fails immediately.
func (c *Client) WaitHealthy(ctx context.Context, run sandbox.RunID, component string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		list, err := c.containers(ctx, run, component)
		if err != nil {
			return err
		}
		ready := len(list) > 0
		for _, s := range list {
			ins, err := c.api.ContainerInspect(ctx, s.ID, client.ContainerInspectOptions{})
			if err != nil {
				return fmt.Errorf("docker: inspect %s: %w", component, err)
			}
			st := ins.Container.State
			switch {
			case st == nil:
				ready = false
			case !st.Running && (st.Status == container.StateExited || st.Status == container.StateDead):
				return fmt.Errorf("docker: %s exited with code %d: %s", component, st.ExitCode, c.tail(ctx, s.ID, 20))
			case st.Health != nil && st.Health.Status != container.Healthy:
				ready = false
			case !st.Running:
				ready = false
			}
		}
		if ready {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("docker: %s not healthy after %s", component, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// RunOnce runs ct to completion as a throwaway container of the run and
// returns its exit code and full output (for builds, seeds, load tests).
func (c *Client) RunOnce(ctx context.Context, run sandbox.RunID, spec sandbox.Spec, ct sandbox.Container) (ExecResult, error) {
	id, name, err := c.launch(ctx, run, spec, ct, true)
	if err != nil {
		return ExecResult{}, err
	}
	defer func() {
		// Cleanup must happen even if ctx was canceled mid-run.
		_, _ = c.api.ContainerRemove(context.WithoutCancel(ctx), id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	}()
	wait := c.api.ContainerWait(ctx, id, client.ContainerWaitOptions{Condition: container.WaitConditionNotRunning})
	select {
	case res := <-wait.Result:
		stdout, stderr := c.output(ctx, id, "all")
		return ExecResult{ExitCode: int(res.StatusCode), Stdout: stdout, Stderr: stderr}, nil
	case err := <-wait.Error:
		return ExecResult{}, fmt.Errorf("docker: wait %s: %w", name, err)
	case <-ctx.Done():
		return ExecResult{}, ctx.Err()
	}
}

// Launch starts ct in the background as part of the run (e.g. long-running
// load). RemoveComponent stops it; Destroy removes it with the run.
func (c *Client) Launch(ctx context.Context, run sandbox.RunID, spec sandbox.Spec, ct sandbox.Container) error {
	_, _, err := c.launch(ctx, run, spec, ct, false)
	return err
}

func (c *Client) launch(ctx context.Context, run sandbox.RunID, spec sandbox.Spec, ct sandbox.Container, oneShot bool) (id, name string, err error) {
	if err := (sandbox.Spec{Networks: spec.Networks, Volumes: spec.Volumes, Containers: []sandbox.Container{ct}}).Validate(); err != nil {
		return "", "", fmt.Errorf("docker: invalid container: %w", err)
	}
	if err := c.requireImages(ctx, sandbox.Spec{Containers: []sandbox.Container{ct}}); err != nil {
		return "", "", err
	}
	name = resourceName(run, ct.Component, strconv.Itoa(ct.Replica))
	var extra map[string]string
	if oneShot {
		name = resourceName(run, ct.Component, "once", strconv.FormatInt(time.Now().UnixNano()%1e9, 36))
		extra = map[string]string{labelOneShot: "true"}
	}
	id, err = c.createContainer(ctx, run, spec, ct, name, extra)
	if err != nil {
		return "", "", err
	}
	if _, err := c.api.ContainerStart(ctx, id, client.ContainerStartOptions{}); err != nil {
		_, _ = c.api.ContainerRemove(context.WithoutCancel(ctx), id, client.ContainerRemoveOptions{Force: true})
		return "", "", fmt.Errorf("docker: start %s: %w", name, err)
	}
	return id, name, nil
}

// RemoveComponent force-removes every container of component (idempotent).
func (c *Client) RemoveComponent(ctx context.Context, run sandbox.RunID, component string) error {
	list, err := c.containers(ctx, run, component)
	if err != nil {
		return err
	}
	for _, s := range list {
		if _, err := c.api.ContainerRemove(ctx, s.ID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true}); err != nil && !isNotFound(err) {
			return fmt.Errorf("docker: remove %s: %w", component, err)
		}
	}
	return nil
}

// Stop stops every replica of component.
func (c *Client) Stop(ctx context.Context, run sandbox.RunID, component string) error {
	return c.each(ctx, run, component, func(id string) error {
		_, err := c.api.ContainerStop(ctx, id, client.ContainerStopOptions{})
		return err
	})
}

// Restart restarts every replica of component.
func (c *Client) Restart(ctx context.Context, run sandbox.RunID, component string) error {
	return c.each(ctx, run, component, func(id string) error {
		_, err := c.api.ContainerRestart(ctx, id, client.ContainerRestartOptions{})
		return err
	})
}

func (c *Client) each(ctx context.Context, run sandbox.RunID, component string, f func(id string) error) error {
	list, err := c.containers(ctx, run, component)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return fmt.Errorf("docker: run %s has no %s container", run, component)
	}
	for _, s := range list {
		if s.Labels[labelOneShot] != "" {
			continue
		}
		if err := f(s.ID); err != nil {
			return fmt.Errorf("docker: %s: %w", component, err)
		}
	}
	return nil
}

// ContainerName is the Docker name of a long-running container of the run.
func ContainerName(run sandbox.RunID, component string, replica int) string {
	return resourceName(run, component, strconv.Itoa(replica))
}

// output returns a container's stdout and stderr ("all" or the last n lines), best effort.
func (c *Client) output(ctx context.Context, id, tail string) (stdout, stderr string) {
	logs, err := c.api.ContainerLogs(ctx, id, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Tail: tail})
	if err != nil {
		return "", ""
	}
	defer logs.Close()
	var out, errb capped
	_, _ = stdcopy.StdCopy(&out, &errb, logs)
	return out.String(), errb.String()
}

// maxOutput bounds what the host keeps of one stream of a container or exec:
// the player's code writes it (a migration can print in a loop).
const maxOutput = 16 << 20

// capped keeps the first maxOutput bytes of a stream and discards the rest,
// without failing the copy.
type capped struct {
	b       bytes.Buffer
	dropped bool
}

func (c *capped) Write(p []byte) (int, error) {
	if room := maxOutput - c.b.Len(); room < len(p) {
		c.dropped = true
		c.b.Write(p[:max(room, 0)])
		return len(p), nil
	}
	return c.b.Write(p)
}

func (c *capped) String() string {
	if c.dropped {
		return c.b.String() + "\n… (output cut at 16 MiB)\n"
	}
	return c.b.String()
}

// tail returns the last n lines of a container's combined output (best effort).
func (c *Client) tail(ctx context.Context, id string, n int) string {
	out, errb := c.output(ctx, id, strconv.Itoa(n))
	return strings.TrimSpace(out + errb)
}

// ErrNotRunning means the container exists but its process is not running.
var ErrNotRunning = errors.New("docker: container is not running")

// ExecResult is the outcome of a non-interactive exec.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Exec runs cmd in replica of component and waits for it (no TTY).
func (c *Client) Exec(ctx context.Context, run sandbox.RunID, component string, replica int, cmd []string, stdin io.Reader) (ExecResult, error) {
	id, err := c.containerID(ctx, run, component, replica)
	if err != nil {
		return ExecResult{}, err
	}
	ex, err := c.api.ExecCreate(ctx, id, client.ExecCreateOptions{
		Cmd: cmd, AttachStdout: true, AttachStderr: true, AttachStdin: stdin != nil,
	})
	if cerrdefs.IsConflict(err) {
		return ExecResult{}, fmt.Errorf("docker: exec in %s: %w", component, ErrNotRunning)
	}
	if err != nil {
		return ExecResult{}, fmt.Errorf("docker: exec in %s: %w", component, err)
	}
	att, err := c.api.ExecAttach(ctx, ex.ID, client.ExecAttachOptions{})
	if err != nil {
		return ExecResult{}, fmt.Errorf("docker: attach exec in %s: %w", component, err)
	}
	defer att.Close()
	if stdin != nil {
		go func() {
			_, _ = io.Copy(att.Conn, stdin)
			_ = att.CloseWrite()
		}()
	}
	var stdout, stderr capped
	if _, err := stdcopy.StdCopy(&stdout, &stderr, att.Reader); err != nil {
		return ExecResult{}, fmt.Errorf("docker: read exec output: %w", err)
	}
	ins, err := c.api.ExecInspect(ctx, ex.ID, client.ExecInspectOptions{})
	if err != nil {
		return ExecResult{}, fmt.Errorf("docker: inspect exec: %w", err)
	}
	return ExecResult{ExitCode: ins.ExitCode, Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

// TTY is an interactive exec with a terminal: Read gets the terminal's
// output (stdout and stderr are one stream with a TTY), Write types into it.
type TTY struct {
	c    *Client
	id   string
	resp client.ExecAttachResult
}

func (t *TTY) Read(p []byte) (int, error)  { return t.resp.Reader.Read(p) }
func (t *TTY) Write(p []byte) (int, error) { return t.resp.Conn.Write(p) }

// Close ends the session; the process gets a hangup when its terminal goes away.
func (t *TTY) Close() error {
	t.resp.Close()
	return nil
}

// Resize changes the terminal size.
func (t *TTY) Resize(ctx context.Context, cols, rows uint) error {
	if _, err := t.c.api.ExecResize(ctx, t.id, client.ExecResizeOptions{Width: cols, Height: rows}); err != nil {
		return fmt.Errorf("docker: resize exec: %w", err)
	}
	return nil
}

// ExecTTY starts cmd in replica of component with a terminal of cols×rows.
func (c *Client) ExecTTY(ctx context.Context, run sandbox.RunID, component string, replica int, cmd []string, cols, rows uint) (*TTY, error) {
	id, err := c.containerID(ctx, run, component, replica)
	if err != nil {
		return nil, err
	}
	size := client.ConsoleSize{Width: cols, Height: rows}
	ex, err := c.api.ExecCreate(ctx, id, client.ExecCreateOptions{
		Cmd: cmd, TTY: true, ConsoleSize: size, AttachStdin: true, AttachStdout: true, AttachStderr: true,
		Env: []string{"TERM=xterm-256color"},
	})
	if err != nil {
		return nil, fmt.Errorf("docker: exec in %s: %w", component, err)
	}
	att, err := c.api.ExecAttach(ctx, ex.ID, client.ExecAttachOptions{TTY: true, ConsoleSize: size})
	if err != nil {
		return nil, fmt.Errorf("docker: attach exec in %s: %w", component, err)
	}
	return &TTY{c: c, id: ex.ID, resp: att}, nil
}

func (c *Client) containerID(ctx context.Context, run sandbox.RunID, component string, replica int) (string, error) {
	list, err := c.containers(ctx, run, component)
	if err != nil {
		return "", err
	}
	for _, s := range list {
		if s.Labels[labelReplica] == strconv.Itoa(replica) && s.Labels[labelOneShot] == "" {
			return s.ID, nil
		}
	}
	return "", fmt.Errorf("docker: run %s has no %s#%d", run, component, replica)
}

// Addr returns the IP of replica of component on network. On Linux the host
// reaches it directly, even on internal networks (Docker Desktop does not).
func (c *Client) Addr(ctx context.Context, run sandbox.RunID, component string, replica int, net string) (netip.Addr, error) {
	id, err := c.containerID(ctx, run, component, replica)
	if err != nil {
		return netip.Addr{}, err
	}
	ins, err := c.api.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return netip.Addr{}, fmt.Errorf("docker: inspect %s: %w", component, err)
	}
	if ns := ins.Container.NetworkSettings; ns != nil {
		if ep, ok := ns.Networks[resourceName(run, net)]; ok && ep.IPAddress.IsValid() {
			return ep.IPAddress, nil
		}
	}
	return netip.Addr{}, fmt.Errorf("docker: %s#%d has no address on %s", component, replica, net)
}

// Destroy removes every container, network and volume of the run. Cache
// volumes are shared across runs and kept. It is idempotent.
func (c *Client) Destroy(ctx context.Context, run sandbox.RunID) error {
	f := make(client.Filters).Add("label", LabelRun+"="+string(run))
	var errs []error
	list, err := c.api.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: f})
	if err != nil {
		return fmt.Errorf("docker: list containers: %w", err)
	}
	for _, s := range list.Items {
		if _, err := c.api.ContainerRemove(ctx, s.ID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true}); err != nil && !isNotFound(err) {
			errs = append(errs, fmt.Errorf("remove container %s: %w", strings.Join(s.Names, ","), err))
		}
	}
	nets, err := c.api.NetworkList(ctx, client.NetworkListOptions{Filters: f})
	if err != nil {
		return fmt.Errorf("docker: list networks: %w", err)
	}
	for _, n := range nets.Items {
		if _, err := c.api.NetworkRemove(ctx, n.ID, client.NetworkRemoveOptions{}); err != nil && !isNotFound(err) {
			errs = append(errs, fmt.Errorf("remove network %s: %w", n.Name, err))
		}
	}
	vols, err := c.api.VolumeList(ctx, client.VolumeListOptions{Filters: f})
	if err != nil {
		return fmt.Errorf("docker: list volumes: %w", err)
	}
	for _, v := range vols.Items {
		if _, err := c.api.VolumeRemove(ctx, v.Name, client.VolumeRemoveOptions{Force: true}); err != nil && !isNotFound(err) {
			errs = append(errs, fmt.Errorf("remove volume %s: %w", v.Name, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("docker: destroy %s: %w", run, err)
	}
	return nil
}

// Leftovers lists every resource still labelled with run. Empty after a
// successful Destroy: this is how cleanup is verified, not presumed.
func (c *Client) Leftovers(ctx context.Context, run sandbox.RunID) ([]string, error) {
	f := make(client.Filters).Add("label", LabelRun+"="+string(run))
	var out []string
	list, err := c.api.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}
	for _, s := range list.Items {
		out = append(out, "container "+strings.Join(s.Names, ","))
	}
	nets, err := c.api.NetworkList(ctx, client.NetworkListOptions{Filters: f})
	if err != nil {
		return nil, err
	}
	for _, n := range nets.Items {
		out = append(out, "network "+n.Name)
	}
	vols, err := c.api.VolumeList(ctx, client.VolumeListOptions{Filters: f})
	if err != nil {
		return nil, err
	}
	for _, v := range vols.Items {
		out = append(out, "volume "+v.Name)
	}
	return out, nil
}

// RunInfo summarizes one run found on the Docker host.
type RunInfo struct {
	Containers int
	Running    int
	Networks   int
	Volumes    int
	// Stopped lists the component of every container that is not running.
	Stopped []string
}

// Runs lists every run that still owns resources on the host.
func (c *Client) Runs(ctx context.Context) (map[sandbox.RunID]*RunInfo, error) {
	f := make(client.Filters).Add("label", managedFilter)
	out := map[sandbox.RunID]*RunInfo{}
	get := func(labels map[string]string) *RunInfo {
		r := sandbox.RunID(labels[LabelRun])
		if r == "" {
			return nil // cache volumes belong to no run
		}
		if out[r] == nil {
			out[r] = &RunInfo{}
		}
		return out[r]
	}
	list, err := c.api.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: f})
	if err != nil {
		return nil, fmt.Errorf("docker: list containers: %w", err)
	}
	for _, s := range list.Items {
		if ri := get(s.Labels); ri != nil {
			ri.Containers++
			if s.State == container.StateRunning {
				ri.Running++
			} else {
				ri.Stopped = append(ri.Stopped, s.Labels[labelComponent])
			}
		}
	}
	nets, err := c.api.NetworkList(ctx, client.NetworkListOptions{Filters: f})
	if err != nil {
		return nil, fmt.Errorf("docker: list networks: %w", err)
	}
	for _, n := range nets.Items {
		if ri := get(n.Labels); ri != nil {
			ri.Networks++
		}
	}
	vols, err := c.api.VolumeList(ctx, client.VolumeListOptions{Filters: f})
	if err != nil {
		return nil, fmt.Errorf("docker: list volumes: %w", err)
	}
	for _, v := range vols.Items {
		if ri := get(v.Labels); ri != nil {
			ri.Volumes++
		}
	}
	return out, nil
}

func isNotFound(err error) bool { return cerrdefs.IsNotFound(err) }
