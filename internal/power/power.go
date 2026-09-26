// Package power tells when the host is saving energy, which slows the
// sandbox and so the verdicts on speed (ADR-0021). It reads Linux sysfs;
// elsewhere it finds nothing and says nothing.
package power

import (
	"os"
	"path/filepath"
	"strings"
)

// Saving describes why the host is running slow to save energy, or "".
func Saving() string { return savingAt("/") }

func savingAt(root string) string {
	read := func(p string) string {
		b, err := os.ReadFile(filepath.Join(root, p)) //nolint:gosec // G304: fixed sysfs paths
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	supplies, _ := filepath.Glob(filepath.Join(root, "sys/class/power_supply/*"))
	for _, s := range supplies {
		rel, _ := filepath.Rel(root, s)
		if read(filepath.Join(rel, "type")) == "Mains" && read(filepath.Join(rel, "online")) == "0" {
			return "o computador está na bateria"
		}
	}
	if read("sys/firmware/acpi/platform_profile") == "low-power" ||
		read("sys/devices/system/cpu/cpu0/cpufreq/energy_performance_preference") == "power" {
		return "o perfil de energia é de economia"
	}
	return ""
}
