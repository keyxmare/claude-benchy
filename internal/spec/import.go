package spec

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Import reads the bench.yaml at path and returns it with every relative path
// (app, output, promptFile, auth.configDir, each config's bundle and
// promptFile) rewritten to an absolute path against the file's own directory,
// so the imported bench points at the same folders wherever it is later saved
// or run from. Everything else — comments, field order, values — is preserved:
// it neither applies defaults nor validates, so the result stays a faithful,
// editable starting point for a new bench.
func Import(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bench: %w", err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse bench: %w", err)
	}
	doc := documentMapping(&root)
	if doc == nil {
		return nil, fmt.Errorf("bench %q: mapping YAML attendu à la racine", path)
	}

	base := filepath.Dir(path)
	absolutizeScalar(mapValue(doc, "app"), base)
	absolutizeScalar(mapValue(doc, "output"), base)
	absolutizeScalar(mapValue(doc, "promptFile"), base)
	absolutizeScalar(mapValue(mapValue(doc, "auth"), "configDir"), base)
	if configs := mapValue(doc, "configs"); configs != nil && configs.Kind == yaml.SequenceNode {
		for _, c := range configs.Content {
			absolutizeScalar(mapValue(c, "bundle"), base)
			absolutizeScalar(mapValue(c, "promptFile"), base)
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		// Re-encoding a node we just decoded cannot fail in practice.
		return nil, fmt.Errorf("encode bench: %w", err)
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}

// documentMapping returns the top-level mapping of a parsed YAML document,
// unwrapping the document node, or nil when the root is not a mapping.
func documentMapping(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		n = n.Content[0]
	}
	if n.Kind == yaml.MappingNode {
		return n
	}
	return nil
}

// mapValue returns the value node for key in a mapping node, or nil when m is
// not a mapping or the key is absent.
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

// absolutizeScalar rewrites a relative path scalar to absolute against base,
// leaving empty, already-absolute and ~-based paths untouched.
func absolutizeScalar(n *yaml.Node, base string) {
	if n == nil || n.Kind != yaml.ScalarNode {
		return
	}
	if n.Value == "" || filepath.IsAbs(n.Value) || strings.HasPrefix(n.Value, "~") {
		return
	}
	n.Value = filepath.Clean(filepath.Join(base, n.Value))
}
