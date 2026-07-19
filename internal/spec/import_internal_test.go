package spec

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDocumentMapping(t *testing.T) {
	t.Parallel()
	mapping := &yaml.Node{Kind: yaml.MappingNode}
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Value: "x"}
	tests := []struct {
		name string
		in   *yaml.Node
		want *yaml.Node
	}{
		{name: "document wrapping mapping", in: &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mapping}}, want: mapping},
		{name: "bare mapping", in: mapping, want: mapping},
		{name: "document with two children", in: &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mapping, scalar}}, want: nil},
		{name: "scalar", in: scalar, want: nil},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := documentMapping(tt.in); got != tt.want {
				t.Errorf("documentMapping(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestMapValue(t *testing.T) {
	t.Parallel()
	value := &yaml.Node{Kind: yaml.ScalarNode, Value: "v"}
	mapping := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "k"}, value,
	}}
	tests := []struct {
		name string
		m    *yaml.Node
		key  string
		want *yaml.Node
	}{
		{name: "nil node", m: nil, key: "k", want: nil},
		{name: "not a mapping", m: value, key: "k", want: nil},
		{name: "key present", m: mapping, key: "k", want: value},
		{name: "key absent", m: mapping, key: "other", want: nil},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := mapValue(tt.m, tt.key); got != tt.want {
				t.Errorf("mapValue(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestAbsolutizeScalar(t *testing.T) {
	t.Parallel()
	base := "/base"
	tests := []struct {
		name string
		in   *yaml.Node
		want string
	}{
		{name: "nil", in: nil, want: ""},
		{name: "not a scalar", in: &yaml.Node{Kind: yaml.MappingNode}, want: ""},
		{name: "empty", in: &yaml.Node{Kind: yaml.ScalarNode, Value: ""}, want: ""},
		{name: "already absolute", in: &yaml.Node{Kind: yaml.ScalarNode, Value: "/x/y"}, want: "/x/y"},
		{name: "home based", in: &yaml.Node{Kind: yaml.ScalarNode, Value: "~/z"}, want: "~/z"},
		{name: "relative", in: &yaml.Node{Kind: yaml.ScalarNode, Value: "./a/b"}, want: "/base/a/b"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			absolutizeScalar(tt.in, base)
			got := ""
			if tt.in != nil {
				got = tt.in.Value
			}
			if got != tt.want {
				t.Errorf("absolutizeScalar(%s).Value = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
