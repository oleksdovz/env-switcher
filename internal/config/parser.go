package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func Load(path string) (*Settings, error) {
	if err := EnsurePrivate(filepath.Dir(path), true); err != nil {
		return nil, fmt.Errorf("settings directory security: %w", err)
	}
	if err := EnsurePrivate(path, false); err != nil {
		return nil, fmt.Errorf("settings file security: %w", err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open settings: %w", err)
	}
	defer f.Close()
	return Parse(io.LimitReader(f, MaxSettingsSize+1))
}

func Parse(r io.Reader) (*Settings, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	if len(b) > MaxSettingsSize {
		return nil, fmt.Errorf("settings file exceeds %d bytes", MaxSettingsSize)
	}
	if !bytes.Equal(bytes.ToValidUTF8(b, nil), b) {
		return nil, fmt.Errorf("settings file is not valid UTF-8")
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	var root yaml.Node
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}
	if len(root.Content) == 0 {
		return nil, fmt.Errorf("settings document is empty")
	}
	if err := inspectNode(root.Content[0], "settings"); err != nil {
		return nil, err
	}
	if err := checkExpansionBudget(root.Content[0]); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple YAML documents are not supported")
		}
		return nil, fmt.Errorf("parse trailing settings data: %w", err)
	}
	var out Settings
	strict := yaml.NewDecoder(bytes.NewReader(b))
	strict.KnownFields(true)
	if err := strict.Decode(&out); err != nil {
		return nil, fmt.Errorf("decode settings: %w", err)
	}
	if err := Validate(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// inspectNode walks the raw parse tree before any Go-struct decoding, rejecting constructs the
// strict decode step wouldn't catch on its own (duplicate keys, non-string keys, custom tags,
// and the two anchor-safety rules below). Aliases and merge keys (`<<`) are otherwise accepted;
// gopkg.in/yaml.v3 resolves merge-key overriding (explicit keys win over merged ones) during the
// later strict Decode, so no extra handling is needed here for that part.
func inspectNode(n *yaml.Node, path string) error {
	if n.Anchor != "" && hasAlias(n) {
		return fmt.Errorf("%s at line %d: an anchored value cannot itself reference another anchor", path, n.Line)
	}
	if n.Kind == yaml.AliasNode || n.Alias != nil {
		// Resolved and validated at the anchor's own definition; nothing further to check
		// at this occurrence.
		return nil
	}
	if strings.HasPrefix(n.Tag, "!") && !strings.HasPrefix(n.Tag, "!!") {
		return fmt.Errorf("%s at line %d: custom tags are not supported", path, n.Line)
	}
	if n.Kind == yaml.MappingNode {
		seen := make(map[string]struct{}, len(n.Content)/2)
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			// The merge key's own key node is tagged !!merge (a reserved YAML 1.1 type), not
			// !!str, even though its literal text is the ordinary string "<<".
			if key.Kind != yaml.ScalarNode || (key.Tag != "!!str" && key.Tag != "!!merge") {
				return fmt.Errorf("%s at line %d: mapping keys must be strings", path, key.Line)
			}
			if _, ok := seen[key.Value]; ok {
				return fmt.Errorf("%s.%s at line %d: duplicate key", path, key.Value, key.Line)
			}
			seen[key.Value] = struct{}{}
			value := n.Content[i+1]
			// env-vars, shell-functions, and shell-cmd are all plain-string fields with no
			// further structure, so an implicit (unquoted) scalar there must still be
			// rejected explicitly — Go's yaml decode would otherwise happily stringify e.g.
			// an unquoted `123` or `true` into the target string field.
			requiresString := key.Value == "project" || key.Value == "shell-cmd" ||
				strings.HasSuffix(path, ".env-vars") || strings.HasSuffix(path, ".shell-functions")
			// A merge key's value is always a mapping (or a sequence of them) supplied via
			// alias, never a scalar, so the explicit-string rule above doesn't apply to it.
			if key.Value != "<<" && requiresString && value.Tag != "!!str" {
				return fmt.Errorf("%s.%s at line %d: value must be an explicit YAML string", path, key.Value, value.Line)
			}
			if err := inspectNode(value, path+"."+key.Value); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range n.Content {
		if err := inspectNode(child, path); err != nil {
			return err
		}
	}
	return nil
}

// hasAlias reports whether n or any descendant is an alias reference. Used to forbid an
// anchor's own definition from referencing another anchor, which is what keeps alias expansion
// linear (one hop) instead of exponential (chained).
func hasAlias(n *yaml.Node) bool {
	if n.Kind == yaml.AliasNode || n.Alias != nil {
		return true
	}
	for _, child := range n.Content {
		if hasAlias(child) {
			return true
		}
	}
	return false
}

// checkExpansionBudget bounds the total size a document may grow to once every alias is
// resolved to its target. Because inspectNode already forbids an anchor's own content from
// containing further aliases, each target's size is cheap to compute and safe to memoize by
// node identity, so this whole pass is linear in document size regardless of how many times
// a given anchor is referenced.
func checkExpansionBudget(root *yaml.Node) error {
	sizes := make(map[*yaml.Node]int)
	var nodeSize func(n *yaml.Node) int
	nodeSize = func(n *yaml.Node) int {
		if size, ok := sizes[n]; ok {
			return size
		}
		size := len(n.Value)
		for _, child := range n.Content {
			size += nodeSize(child)
		}
		sizes[n] = size
		return size
	}
	total := 0
	var walk func(n *yaml.Node) error
	walk = func(n *yaml.Node) error {
		if n.Alias != nil {
			total += nodeSize(n.Alias)
			if total > MaxExpandedSettingsSize {
				return fmt.Errorf("settings document expands to more than %d bytes after resolving anchors", MaxExpandedSettingsSize)
			}
			return nil
		}
		for _, child := range n.Content {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root)
}
