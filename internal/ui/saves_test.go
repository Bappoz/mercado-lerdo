package ui

import (
	"path/filepath"
	"testing"
)

func TestSaveSlots(t *testing.T) {
	got, err := SaveSlots("workspace")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != MaxSaves || filepath.Base(got[0]) != "workspace" || filepath.Base(got[2]) != "workspace-3" || !filepath.IsAbs(got[1]) {
		t.Errorf("slots = %v", got)
	}
}
