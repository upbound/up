package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"sigs.k8s.io/yaml"
)

type docsCmd struct {
	OutputDir string `name:"output-dir" help:"Path of the output directory for docs"`
}

func (d *docsCmd) Run(ctx *kong.Context) error {
	root := ctx.Model.Node
	return traverseChildren(root, d.docsForNode)
}

func (d *docsCmd) docsForNode(n *kong.Node) error {
	fname := n.FullPath()
	fname = strings.ReplaceAll(fname, " ", "_")
	fname += ".yaml"
	fname = filepath.Join(d.OutputDir, fname)

	y, err := yaml.Marshal((*aliasNode)(n))
	if err != nil {
		return err
	}

	return os.WriteFile(fname, y, 0600)
}

func traverseChildren(root *kong.Node, fn func(*kong.Node) error) error {
	if root.Hidden {
		return nil
	}
	err := fn(root)
	if err != nil {
		return err
	}
	for _, node := range root.Children {
		err := traverseChildren(node, fn)
		if err != nil {
			return err
		}
	}
	return nil
}

type aliasNode kong.Node

func (n *aliasNode) MarshalJSON() ([]byte, error) {
	return marshalNodeJSON((*kong.Node)(n))
}

type docsNode struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Options     []docsOption `json:"options,omitempty"`
}

type docsOption struct {
	Name         string `json:"name"`
	Shorthand    string `json:"shorthand,omitempty"`
	DefaultValue string `json:"default_value,omitempty"`
	Description  string `json:"description"`
}

func marshalNodeJSON(n *kong.Node) ([]byte, error) {
	dn := &docsNode{
		Name:        n.FullPath(),
		Description: n.Help,
	}
	for _, f := range n.Flags {
		opt := docsOption{
			Name:         f.Name,
			DefaultValue: f.Default,
			Description:  f.Help,
		}
		if f.Short != 0 {
			opt.Shorthand = string(f.Short)
		}
		dn.Options = append(dn.Options, opt)
	}

	return json.Marshal(dn)
}
