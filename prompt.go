package goaikit

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"text/template"
)

type Render[Context any] struct {
	Context Context `json:"ctx"`
	Data    any     `json:"data"`
}

type Template[Context any] interface {
	Load(fs embed.FS) error
	Execute(name string, data Render[Context]) (string, error)
}

type manager[Context any] struct {
	templates map[string]*template.Template
}

func NewTemplate[Context any]() Template[Context] {
	return &manager[Context]{templates: make(map[string]*template.Template)}
}

func (m *manager[Context]) Load(fileSystem embed.FS) error {
	return fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".tpl" && ext != ".tmpl" && ext != ".gotmpl" {
			return nil
		}

		data, err := fileSystem.ReadFile(path)
		if err != nil {
			return err
		}

		name := strings.TrimSuffix(filepath.Base(path), ext)
		tmpl, err := template.New(name).Parse(string(data))
		if err != nil {
			return err
		}

		m.templates[name] = tmpl

		return nil
	})
}

func (m *manager[Context]) Execute(name string, args Render[Context]) (string, error) {
	tmpl, ok := m.templates[name]
	if !ok {
		return "", fmt.Errorf("template %q not found", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, args); err != nil {
		return "", err
	}

	return buf.String(), nil
}
