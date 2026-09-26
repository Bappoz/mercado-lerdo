package topology

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ErrInstalled means the component is already in the topology.
var ErrInstalled = errors.New("topology: already installed")

// Install adds the optional component k to src (the text of topology.yaml)
// with the catalog's defaults, keeping the player's comments and order.
// It changes only the file: the running system changes on the next deploy.
func Install(src []byte, k Kind) ([]byte, error) {
	e, ok := Catalog[k]
	if !ok || !e.Optional || e.Snippet == "" {
		return nil, fmt.Errorf("topology: %s cannot be installed", k)
	}
	t, err := Parse(src)
	if err != nil {
		return nil, err
	}
	if t.Installed(k) {
		return nil, ErrInstalled
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("topology: %w", err)
	}
	comps := mapValue(doc.Content[0], "components")
	if comps == nil || comps.Kind != yaml.MappingNode {
		return nil, errors.New("topology: components is not a mapping")
	}
	var add yaml.Node
	if err := yaml.Unmarshal([]byte(e.Snippet), &add); err != nil {
		return nil, fmt.Errorf("topology: catalog snippet: %w", err)
	}
	key, val := add.Content[0].Content[0], add.Content[0].Content[1]
	key.HeadComment = fmt.Sprintf("%s — instalado pelo app Docs. Aplique com um deploy.", e.Name)
	comps.Content = append(comps.Content, key, val)

	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("topology: %w", err)
	}
	if _, err := Parse(out.Bytes()); err != nil {
		return nil, fmt.Errorf("topology: install produced an invalid file: %w", err)
	}
	return out.Bytes(), nil
}

// InstallFile applies Install to <workspace>/topology.yaml, creating the file
// from Default when the workspace predates it.
func InstallFile(workspace string, k Kind) error {
	path := filepath.Join(workspace, File)
	src, err := os.ReadFile(path) //nolint:gosec // G304: the player's own workspace
	if errors.Is(err, os.ErrNotExist) {
		src, err = yaml.Marshal(Default())
	}
	if err != nil {
		return fmt.Errorf("topology: %w", err)
	}
	out, err := Install(src, k)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644) //nolint:gosec // G306: a source file of the player's repository
}

func mapValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// Set changes component fields of src ("app.replicas": 3), keeping the
// player's comments, order and style; a missing field is added. The result
// must still be a valid topology.
func Set(src []byte, fields map[string]int) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("topology: %w", err)
	}
	comps := mapValue(doc.Content[0], "components")
	for path, v := range fields {
		comp, field, ok := strings.Cut(path, ".")
		m := mapValue(comps, comp)
		if !ok || m == nil || m.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("topology: no component for %q", path)
		}
		val := strconv.Itoa(v)
		if n := mapValue(m, field); n != nil {
			n.Value, n.Tag = val, "!!int"
			continue
		}
		m.Content = append(m.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: field},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: val})
	}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("topology: %w", err)
	}
	if _, err := Parse(out.Bytes()); err != nil {
		return nil, fmt.Errorf("topology: the change produced an invalid file: %w", err)
	}
	return out.Bytes(), nil
}

// SetFile applies Set to <workspace>/topology.yaml (from Default when missing).
func SetFile(workspace string, fields map[string]int) error {
	path := filepath.Join(workspace, File)
	src, err := os.ReadFile(path) //nolint:gosec // G304: the player's own workspace
	if errors.Is(err, os.ErrNotExist) {
		src, err = yaml.Marshal(Default())
	}
	if err != nil {
		return fmt.Errorf("topology: %w", err)
	}
	out, err := Set(src, fields)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644) //nolint:gosec // G306: a source file of the player's repository
}
