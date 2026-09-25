package events

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "events.jsonl")
	at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	l := Log{Path: path, Scenario: "stage1/c1", now: func() time.Time { return at }}
	if err := l.Append("hint_requested", map[string]any{"level": 2}); err != nil {
		t.Fatal(err)
	}
	if err := l.Append("scenario_completed", nil); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var got []Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		got = append(got, e)
	}
	if len(got) != 2 || got[0].Event != "hint_requested" || got[0].Props["level"] != float64(2) ||
		!got[0].TS.Equal(at) || got[1].Scenario != "stage1/c1" {
		t.Errorf("events = %+v", got)
	}
}
