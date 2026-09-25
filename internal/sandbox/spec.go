package sandbox

import (
	"crypto/rand"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// RunID identifies one sandbox run; every Docker resource of the run carries it
// in the dev.sesim.run label.
type RunID string

// NewRunID returns a short random run id ("r-" + 10 hex chars).
func NewRunID() RunID {
	b := make([]byte, 5)
	_, _ = rand.Read(b) // crypto/rand.Read never fails on supported platforms
	return RunID(fmt.Sprintf("r-%x", b))
}

// Spec describes a whole sandbox. It deliberately has no way to express a
// privileged container, the host network, the host PID namespace or a
// writable bind mount (threat model T6/T13); Validate enforces the rest.
type Spec struct {
	Networks   []Network
	Volumes    []Volume
	Containers []Container
}

// Network is a bridge network of the run.
type Network struct {
	Name     string
	Internal bool // no route out of the network (threat model T14)
}

// Volume is a named volume. Cache volumes outlive runs (e.g. the Go build cache).
type Volume struct {
	Name  string
	Cache bool
}

// Container is one container of the sandbox.
type Container struct {
	Component  string // DNS alias on every network, e.g. "postgres"
	Replica    int    // 1-based
	Image      string
	Entrypoint []string
	Cmd        []string
	Env        map[string]string
	User       string
	Networks   []string // names from Spec.Networks; none = no network at all
	Mounts     []Mount
	ReadOnly   bool     // read-only root filesystem
	CapAdd     []string // on top of cap_drop: ALL; restricted by capAllowed
	Resources  Resources
	ShmBytes   int64
	Health     *Health
}

// MountType is the kind of a mount.
type MountType string

// Mount types.
const (
	VolumeMount MountType = "volume"
	BindMount   MountType = "bind" // always read-only
	TmpfsMount  MountType = "tmpfs"
)

// Mount attaches storage to a container.
type Mount struct {
	Type     MountType
	Source   string // volume name or absolute host path; empty for tmpfs
	Target   string
	ReadOnly bool
	Bytes    int64 // tmpfs size (0 = Docker default)
}

// Resources are the hard limits of a container; all are required.
type Resources struct {
	CPUs   float64
	Memory int64 // bytes
	Pids   int64
}

// Health is a container health check (exec form).
type Health struct {
	Cmd      []string
	Interval time.Duration
	Timeout  time.Duration
	Retries  int
}

// capAllowed reports whether a capability may be added back. The list covers
// images whose entrypoint starts as root and drops privileges (postgres).
func capAllowed(c string) bool {
	switch c {
	case "CHOWN", "DAC_OVERRIDE", "FOWNER", "SETGID", "SETUID":
		return true
	}
	return false
}

// forbiddenBindRoots are host paths that must never be mounted into a sandbox.
func forbiddenBindRoots() []string {
	return []string{"/", "/proc", "/sys", "/dev", "/etc", "/run", "/var/run", "/var/lib/docker", "/boot", "/root"}
}

// Validate checks the hardening invariants of docs/architecture/sandbox.md §2
// and returns every violation.
func (s Spec) Validate() error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	nets := map[string]bool{}
	for _, n := range s.Networks {
		nets[n.Name] = true
	}
	vols := map[string]bool{}
	for _, v := range s.Volumes {
		vols[v.Name] = true
	}
	seen := map[string]bool{}
	for _, c := range s.Containers {
		id := fmt.Sprintf("%s#%d", c.Component, c.Replica)
		if c.Component == "" || c.Replica < 1 || seen[id] {
			fail("container %q: needs a component, a replica >= 1, and must be unique", id)
		}
		seen[id] = true
		if !strings.Contains(c.Image, "@sha256:") && !strings.HasPrefix(c.Image, "sesim/") {
			fail("%s: image %q must be pinned by digest (or be a locally built sesim/* image)", id, c.Image)
		}
		if r := c.Resources; r.CPUs <= 0 || r.Memory <= 0 || r.Pids <= 0 {
			fail("%s: CPU, memory and pids limits are all required", id)
		}
		for _, cp := range c.CapAdd {
			if !capAllowed(cp) {
				fail("%s: capability %s is not allowed", id, cp)
			}
		}
		for _, n := range c.Networks {
			if !nets[n] {
				fail("%s: unknown network %q", id, n)
			}
		}
		for _, m := range c.Mounts {
			if err := validateMount(m, vols); err != nil {
				fail("%s: %v", id, err)
			}
		}
	}
	return errors.Join(errs...)
}

func validateMount(m Mount, vols map[string]bool) error {
	if !filepath.IsAbs(m.Target) {
		return fmt.Errorf("mount target %q must be absolute", m.Target)
	}
	switch m.Type {
	case VolumeMount:
		if !vols[m.Source] {
			return fmt.Errorf("unknown volume %q", m.Source)
		}
	case TmpfsMount:
		if m.Source != "" {
			return errors.New("tmpfs mounts have no source")
		}
	case BindMount:
		if !m.ReadOnly {
			return fmt.Errorf("bind mount %s must be read-only", m.Source)
		}
		src := filepath.Clean(m.Source)
		if !filepath.IsAbs(src) || strings.Contains(src, "docker.sock") {
			return fmt.Errorf("bind mount %s is not allowed", m.Source)
		}
		for _, root := range forbiddenBindRoots() {
			if src == root || (root != "/" && strings.HasPrefix(src, root+"/")) {
				return fmt.Errorf("bind mount %s is under %s, which is off-limits", m.Source, root)
			}
		}
	default:
		return fmt.Errorf("unknown mount type %q", m.Type)
	}
	return nil
}

// Find returns the container of component/replica.
func (s Spec) Find(component string, replica int) (Container, bool) {
	i := slices.IndexFunc(s.Containers, func(c Container) bool { return c.Component == component && c.Replica == replica })
	if i < 0 {
		return Container{}, false
	}
	return s.Containers[i], true
}
