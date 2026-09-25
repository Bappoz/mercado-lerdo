// Package events records product events as local JSONL
// (docs/architecture/data-model.md §3). Nothing is sent over the network, and
// events never carry code, SQL, free text, paths or identity.
package events

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Event is one line of the log.
type Event struct {
	V        int            `json:"v"`
	TS       time.Time      `json:"ts"`
	Event    string         `json:"event"`
	Scenario string         `json:"scenario,omitempty"`
	Props    map[string]any `json:"props,omitempty"`
}

// Log appends events to a JSONL file.
type Log struct {
	Path     string
	Scenario string
	now      func() time.Time
}

// Append writes one event. Failing to record an event must never break the
// game, so callers usually ignore the error.
func (l Log) Append(name string, props map[string]any) error {
	now := time.Now
	if l.now != nil {
		now = l.now
	}
	b, err := json.Marshal(Event{V: 1, TS: now().UTC(), Event: name, Scenario: l.Scenario, Props: props})
	if err != nil {
		return fmt.Errorf("events: encode: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o750); err != nil {
		return fmt.Errorf("events: %w", err)
	}
	f, err := os.OpenFile(l.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("events: %w", err)
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}
