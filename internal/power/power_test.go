package power

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSavingAt(t *testing.T) {
	write := func(root, p, v string) {
		t.Helper()
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(v+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"no sysfs (macOS, Windows)", nil, ""},
		{"plugged in, balanced", map[string]string{
			"sys/class/power_supply/ADP0/type": "Mains", "sys/class/power_supply/ADP0/online": "1",
			"sys/firmware/acpi/platform_profile": "balanced",
		}, ""},
		{"on battery", map[string]string{
			"sys/class/power_supply/ADP0/type": "Mains", "sys/class/power_supply/ADP0/online": "0",
			"sys/class/power_supply/BAT0/type": "Battery",
		}, "o computador está na bateria"},
		{"power-saver on AC", map[string]string{
			"sys/class/power_supply/ADP0/type": "Mains", "sys/class/power_supply/ADP0/online": "1",
			"sys/devices/system/cpu/cpu0/cpufreq/energy_performance_preference": "power",
		}, "o perfil de energia é de economia"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for p, v := range tc.files {
				write(root, p, v)
			}
			if got := savingAt(root); got != tc.want {
				t.Errorf("savingAt = %q, want %q", got, tc.want)
			}
		})
	}
}
