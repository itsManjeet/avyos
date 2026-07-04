package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	node         *yaml.Node
	searchPath   []string
	virtualFiles map[string][]byte
}

func NewConfig() Config {
	return Config{node: &yaml.Node{Kind: yaml.MappingNode}}
}

func (c *Config) UpdateFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file %q", path)
	}
	return c.UpdateFrom(data, path)
}

func (c *Config) UpdateFrom(data []byte, path string) error {
	next := &yaml.Node{}
	if err := yaml.Unmarshal(data, next); err != nil {
		return err
	}
	if next.Kind == yaml.DocumentNode && len(next.Content) > 0 {
		next = next.Content[0]
	}
	merged, err := mergeYAML(c.node, next)
	if err != nil {
		return err
	}
	c.node = merged

	mergeNode := mapValue(next, "merge")
	if mergeNode == nil {
		return nil
	}
	for _, item := range mergeNode.Content {
		name := scalarString(item)
		candidate := filepath.Join(filepath.Dir(path), name)
		if _, err := os.Stat(candidate); err == nil {
			if err := c.UpdateFromFile(candidate); err != nil {
				return fmt.Errorf("failed to load %s because %w to merge", path, err)
			}
			continue
		}
		found := false
		for _, base := range c.searchPath {
			candidate = filepath.Join(base, name)
			if _, err := os.Stat(candidate); err == nil {
				found = true
				if err := c.UpdateFromFile(candidate); err != nil {
					return fmt.Errorf("failed to load %s because %w to merge", path, err)
				}
				break
			}
		}
		if !found {
			if data, ok := c.virtualFiles[name]; ok {
				if err := c.UpdateFrom(data, name); err != nil {
					return fmt.Errorf("failed to load virtual merge %s for %s because %w", name, path, err)
				}
				continue
			}
			return fmt.Errorf("failed to load %s because missing required file to merge %q to merge", path, name)
		}
	}
	return nil
}

func mergeYAML(a, b *yaml.Node) (*yaml.Node, error) {
	if a == nil || a.Kind == 0 {
		return cloneYAML(b), nil
	}
	if b == nil || b.Kind == 0 {
		return cloneYAML(a), nil
	}
	if a.Kind == yaml.MappingNode && b.Kind == yaml.MappingNode {
		out := cloneYAML(a)
		for i := 0; i+1 < len(b.Content); i += 2 {
			key := b.Content[i].Value
			dst := mapValue(out, key)
			if dst != nil {
				merged, err := mergeYAML(dst, b.Content[i+1])
				if err != nil {
					return nil, err
				}
				setMapValue(out, key, merged)
			} else {
				out.Content = append(out.Content, cloneYAML(b.Content[i]), cloneYAML(b.Content[i+1]))
			}
		}
		return out, nil
	}
	if a.Kind == yaml.SequenceNode && b.Kind == yaml.SequenceNode {
		out := cloneYAML(a)
		for _, item := range b.Content {
			out.Content = append(out.Content, cloneYAML(item))
		}
		return out, nil
	}
	if a.Kind == yaml.ScalarNode && b.Kind == yaml.ScalarNode {
		return cloneYAML(a), nil
	}
	var buf bytes.Buffer
	_ = yaml.NewEncoder(&buf).Encode(a)
	return nil, fmt.Errorf("Can't handle other type: %s", buf.String())
}

func cloneYAML(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	out := *n
	out.Content = make([]*yaml.Node, len(n.Content))
	for i, child := range n.Content {
		out.Content[i] = cloneYAML(child)
	}
	return &out
}

func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func setMapValue(n *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			n.Content[i+1] = value
			return
		}
	}
	n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

func scalarString(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	var out string
	_ = n.Decode(&out)
	return out
}

func (c Config) Has(key string) bool {
	return mapValue(c.node, key) != nil
}

func (c Config) Node(key string) *yaml.Node {
	return mapValue(c.node, key)
}

func (c Config) String(key string, fallback ...string) (string, error) {
	n := mapValue(c.node, key)
	if n == nil {
		if len(fallback) > 0 {
			return fallback[0], nil
		}
		return "", fmt.Errorf("missing required key %q", key)
	}
	return scalarString(n), nil
}

func (c Config) Bool(key string, fallback bool) bool {
	n := mapValue(c.node, key)
	if n == nil {
		return fallback
	}
	var out bool
	if err := n.Decode(&out); err != nil {
		return fallback
	}
	return out
}

func (c Config) StringSlice(key string) []string {
	n := mapValue(c.node, key)
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]string, 0, len(n.Content))
	for _, item := range n.Content {
		out = append(out, scalarString(item))
	}
	return out
}

func (c Config) ScalarMap(key string) map[string]string {
	n := mapValue(c.node, key)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	out := map[string]string{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		out[n.Content[i].Value] = scalarString(n.Content[i+1])
	}
	return out
}

func (c *Config) SetString(key, value string) {
	setMapValue(c.node, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}
