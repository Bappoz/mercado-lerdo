package sandbox

import (
	"strings"
	"testing"
)

func okContainer() Container {
	return Container{
		Component: "app", Replica: 1, Image: "alpine@sha256:abc",
		Networks:  []string{"play"},
		Resources: Resources{CPUs: 0.5, Memory: 64 << 20, Pids: 64},
		Mounts: []Mount{
			{Type: VolumeMount, Source: "bin", Target: "/app", ReadOnly: true},
			{Type: BindMount, Source: "/home/u/repo/deploy/nginx.conf", Target: "/etc/nginx/conf.d/default.conf", ReadOnly: true},
			{Type: TmpfsMount, Target: "/tmp"},
		},
	}
}

func spec(c Container) Spec {
	return Spec{Networks: []Network{{Name: "play", Internal: true}}, Volumes: []Volume{{Name: "bin"}}, Containers: []Container{c}}
}

func TestValidateAcceptsHardenedContainer(t *testing.T) {
	if err := spec(okContainer()).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Container)
		wantErr string
	}{
		{"unpinned image", func(c *Container) { c.Image = "postgres:latest" }, "pinned by digest"},
		{"missing memory limit", func(c *Container) { c.Resources.Memory = 0 }, "limits are all required"},
		{"dangerous capability", func(c *Container) { c.CapAdd = []string{"SYS_ADMIN"} }, "SYS_ADMIN is not allowed"},
		{"docker socket", func(c *Container) {
			c.Mounts = []Mount{{Type: BindMount, Source: "/var/run/docker.sock", Target: "/var/run/docker.sock", ReadOnly: true}}
		}, "not allowed"},
		{"socket via another path", func(c *Container) {
			c.Mounts = []Mount{{Type: BindMount, Source: "/home/u/docker.sock", Target: "/s", ReadOnly: true}}
		}, "not allowed"},
		{"writable bind", func(c *Container) {
			c.Mounts = []Mount{{Type: BindMount, Source: "/home/u/repo", Target: "/src"}}
		}, "must be read-only"},
		{"host root", func(c *Container) {
			c.Mounts = []Mount{{Type: BindMount, Source: "/", Target: "/host", ReadOnly: true}}
		}, "off-limits"},
		{"host etc via traversal", func(c *Container) {
			c.Mounts = []Mount{{Type: BindMount, Source: "/home/../etc/shadow", Target: "/x", ReadOnly: true}}
		}, "off-limits"},
		{"unknown network", func(c *Container) { c.Networks = []string{"host"} }, `unknown network "host"`},
		{"unknown volume", func(c *Container) {
			c.Mounts = []Mount{{Type: VolumeMount, Source: "nope", Target: "/x"}}
		}, `unknown volume "nope"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := okContainer()
			tt.mutate(&c)
			err := spec(c).Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestNewRunID(t *testing.T) {
	a, b := NewRunID(), NewRunID()
	if a == b || !strings.HasPrefix(string(a), "r-") || len(a) != 12 {
		t.Errorf("run ids %q %q", a, b)
	}
}
