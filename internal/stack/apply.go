package stack

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
)

// Action is what a deploy does to one component.
type Action string

// Actions.
const (
	Create   Action = "create"
	Recreate Action = "recreate"
	Remove   Action = "remove"
)

// Change is one component a deploy touches.
type Change struct {
	Component string `json:"component"`
	Action    Action `json:"action"`
}

// playerOrder is dependency order: a component starts after what it uses.
var playerOrder = []string{"postgres", "pgbouncer", "app", "nginx"}

// Topology is what the sandbox runs now.
func (s *Stack) Topology() topology.Topology { return s.opts.Topology }

// Plan lists what Apply(t) would change, in the order it would.
func (s *Stack) Plan(t topology.Topology) ([]Change, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	next := s.opts
	next.Topology = t
	was, will := byComponent(playerComponents(s.opts)), byComponent(playerComponents(next))
	confChanged, err := s.nginxConfChanged()
	if err != nil {
		return nil, err
	}
	var out []Change
	appChanged := false
	for _, c := range playerOrder {
		old, now := was[c], will[c]
		switch {
		case len(old) == 0 && len(now) == 0:
			continue
		case len(old) == 0:
			out = append(out, Change{c, Create})
		case len(now) == 0:
			out = append(out, Change{c, Remove})
		case c == "nginx" && (confChanged || appChanged || !reflect.DeepEqual(withoutConfHash(old), withoutConfHash(now))):
			// Recreated app replicas get new addresses; nginx resolves its
			// upstream once, at start.
			out = append(out, Change{c, Recreate})
		case c != "nginx" && !reflect.DeepEqual(old, now):
			out = append(out, Change{c, Recreate})
			appChanged = appChanged || c == "app"
		}
	}
	return out, nil
}

// Apply makes the sandbox run t and the workspace's nginx configuration. It
// tests a new nginx configuration before touching anything, then recreates
// only what changed. Replaced app replicas drop their in-flight requests:
// there is no rolling update.
func (s *Stack) Apply(ctx context.Context, t topology.Topology, say func(string)) ([]Change, error) {
	plan, err := s.Preflight(ctx, t)
	if err != nil {
		return nil, err
	}
	next := s.opts
	next.Topology = t
	will := byComponent(playerComponents(next))
	nextSpec := Spec(next)
	for _, ch := range plan {
		say(fmt.Sprintf("%s: %s", ch.Component, map[Action]string{Create: "criando", Recreate: "recriando", Remove: "removendo"}[ch.Action]))
		if err := s.d.RemoveComponent(ctx, s.Run, ch.Component); err != nil {
			return nil, err
		}
		if ch.Action == Remove {
			continue
		}
		for _, ct := range will[ch.Component] {
			if err := s.d.Launch(ctx, s.Run, nextSpec, ct); err != nil {
				return nil, err
			}
		}
		if err := s.d.WaitHealthy(ctx, s.Run, ch.Component, healthyTimeout); err != nil {
			return nil, err
		}
		if ch.Component == "nginx" {
			s.appURL = ""
		}
	}
	s.opts, s.spec = next, nextSpec
	return plan, s.recordDeployed()
}

// Preflight checks that t can be applied without touching the sandbox: the
// topology is valid and a new nginx configuration passes `nginx -t`.
func (s *Stack) Preflight(ctx context.Context, t topology.Topology) ([]Change, error) {
	plan, err := s.Plan(t)
	if err != nil {
		return nil, err
	}
	next := s.opts
	next.Topology = t
	for _, ch := range plan {
		if ch.Component == "nginx" && ch.Action != Remove {
			if err := s.testNginx(ctx, Spec(next), byComponent(playerComponents(next))["nginx"][0]); err != nil {
				return nil, err
			}
		}
	}
	return plan, nil
}

// testNginx runs `nginx -t` on the new configuration, on the player's network
// so the upstream names resolve, without touching the running nginx.
func (s *Stack) testNginx(ctx context.Context, spec sandbox.Spec, ng sandbox.Container) error {
	ng.Component = "nginx-test"
	ng.Networks = []string{netPlay}
	// Straight to nginx: the image's entrypoint scripts only add noise here.
	ng.Entrypoint, ng.Cmd = []string{"nginx"}, []string{"-t", "-q"}
	ng.Health = nil
	res, err := s.d.RunOnce(ctx, s.Run, spec, ng)
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("a configuração do nginx (nginx/default.conf) é inválida; nada foi alterado:\n%s",
			strings.TrimSpace(res.Stdout+res.Stderr))
	}
	return nil
}

func (s *Stack) nginxConfChanged() (bool, error) {
	if s.reg == nil {
		return false, nil
	}
	rec, err := s.reg.Get(s.Run)
	if err != nil || rec == nil {
		return false, err
	}
	// Records from before this field existed: the running config is unknown.
	return rec.NginxConfHash != fileHash(NginxConf(s.opts)), nil
}

func (s *Stack) recordDeployed() error {
	if s.reg == nil {
		return nil
	}
	rec, err := s.reg.Get(s.Run)
	if err != nil || rec == nil {
		return err
	}
	t := s.opts.Topology
	rec.Topology, rec.NginxConfHash = &t, fileHash(NginxConf(s.opts))
	return s.reg.Put(*rec)
}

func byComponent(cs []sandbox.Container) map[string][]sandbox.Container {
	out := map[string][]sandbox.Container{}
	for _, c := range cs {
		out[c.Component] = append(out[c.Component], c)
	}
	return out
}

// withoutConfHash compares nginx containers apart from the config hash,
// which the registry tracks for the running one.
func withoutConfHash(cs []sandbox.Container) []sandbox.Container {
	out := make([]sandbox.Container, len(cs))
	for i, c := range cs {
		env := map[string]string{}
		for k, v := range c.Env {
			if k != "SESIM_CONF_SHA256" {
				env[k] = v
			}
		}
		c.Env = env
		out[i] = c
	}
	return out
}
