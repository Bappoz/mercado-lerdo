package stack

import (
	"slices"
	"testing"

	"github.com/Bappoz/software-engineering-simulator/internal/sandbox"
)

func testOptions() Options {
	return Options{Root: "/home/u/repo", Workspace: "/home/u/repo/workspace", PGCPUs: 0.5, Secrets: NewSecrets()}
}

func TestSpecIsValid(t *testing.T) {
	o := testOptions()
	s := Spec(o)
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []sandbox.Container{builder(o), seeder(o, "/home/u/repo/nexora/migrations", "nexora")} {
		one := sandbox.Spec{Networks: s.Networks, Volumes: s.Volumes, Containers: []sandbox.Container{c}}
		if err := one.Validate(); err != nil {
			t.Errorf("%s: %v", c.Component, err)
		}
	}
}

// ADR-0014 / threat model T12: nothing the player controls may share a
// network with the metrics store.
func TestTelemetryNetworkIsolation(t *testing.T) {
	allowed := []string{"prometheus", "agent"}
	for _, c := range Spec(testOptions()).Containers {
		if slices.Contains(c.Networks, netTelemetry) && !slices.Contains(allowed, c.Component) {
			t.Errorf("%s is on the telemetry network", c.Component)
		}
		if c.Component == "prometheus" && !slices.Equal(c.Networks, []string{netTelemetry}) {
			t.Errorf("prometheus must be on the telemetry network only, got %v", c.Networks)
		}
	}
}

func TestBuilderHasNoNetwork(t *testing.T) {
	if n := builder(testOptions()).Networks; len(n) != 0 {
		t.Errorf("builder networks = %v, want none (offline build)", n)
	}
}

func TestNetworksAreInternal(t *testing.T) {
	for _, n := range Spec(testOptions()).Networks {
		if !n.Internal {
			t.Errorf("network %s is not internal", n.Name)
		}
	}
}

func TestSecretsAreRandomAndDistinct(t *testing.T) {
	a, b := NewSecrets(), NewSecrets()
	if a == b || a.Admin == a.App || len(a.Admin) != 32 {
		t.Errorf("secrets %+v %+v", a, b)
	}
}
