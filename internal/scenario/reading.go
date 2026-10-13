package scenario

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// Reading is one piece of documentation Slotthy recommends for a mission
// (ADR-0026): where it is, how long it takes and why it is worth the time, in
// her words. It names a subject to understand, never the diagnosis: that is
// the hint ladder's.
type Reading struct {
	ID      string `yaml:"id"`
	Title   string `yaml:"title"`
	Source  string `yaml:"source"`  // who publishes it
	URL     string `yaml:"url"`     // https only
	Lang    string `yaml:"lang"`    // see ReadingLangs
	Minutes int    `yaml:"minutes"` // how long it takes to read
	Why     string `yaml:"why"`
}

// ReadingLangs are the languages a reading may be in.
var ReadingLangs = []string{"pt", "en"}

// ValidateReading checks the reading list of one pack.
func ValidateReading(list []Reading) error {
	var errs []error
	seen := map[string]bool{}
	for i, r := range list {
		fail := func(format string, a ...any) {
			errs = append(errs, fmt.Errorf("reading[%d] %s: %s", i, r.ID, fmt.Sprintf(format, a...)))
		}
		if r.ID == "" || seen[r.ID] {
			fail("needs a unique id")
		}
		seen[r.ID] = true
		if strings.TrimSpace(r.Title) == "" || strings.TrimSpace(r.Source) == "" || strings.TrimSpace(r.Why) == "" {
			fail("needs a title, a source and why it is worth reading")
		}
		if u, err := url.Parse(r.URL); err != nil || u.Scheme != "https" || u.Host == "" {
			fail("url %q is not an https link", r.URL)
		}
		if !slices.Contains(ReadingLangs, r.Lang) {
			fail("lang %q is not one of %v", r.Lang, ReadingLangs)
		}
		if r.Minutes < 1 {
			fail("minutes is how long it takes to read: at least 1")
		}
	}
	return errors.Join(errs...)
}
