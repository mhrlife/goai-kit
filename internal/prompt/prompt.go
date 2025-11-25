package prompt

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
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
	templateSet *template.Template
}

func NewTemplate[Context any]() Template[Context] {
	return &manager[Context]{}
}

func (m *manager[Context]) Load(fileSystem embed.FS) error {
	var templateFiles []string

	err := fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".tpl" || ext == ".tmpl" || ext == ".gotmpl" {
				templateFiles = append(templateFiles, path)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	if len(templateFiles) == 0 {
		return fmt.Errorf("no template files found")
	}

	slog.Debug("Loading templates", "files", templateFiles)

	tmplSet, err := template.New("").Funcs(funcMap).ParseFS(fileSystem, templateFiles...)
	if err != nil {
		return err
	}

	m.templateSet = tmplSet
	return nil
}

func (m *manager[Context]) Execute(name string, args Render[Context]) (string, error) {
	if m.templateSet == nil {
		return "", fmt.Errorf("templates not loaded")
	}

	// Try to find the template by name
	var tmpl *template.Template

	// First try the exact name
	tmpl = m.templateSet.Lookup(name)

	// If not found, try with .tpl extension
	if tmpl == nil {
		tmpl = m.templateSet.Lookup(name + ".tpl")
	}

	// If still not found, try other extensions
	if tmpl == nil {
		for _, ext := range []string{".tmpl", ".gotmpl"} {
			tmpl = m.templateSet.Lookup(name + ext)
			if tmpl != nil {
				break
			}
		}
	}

	if tmpl == nil {
		return "", fmt.Errorf("template %q not found", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, args); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func toJSONwSchema(v interface{}) string {
	jsonBytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "Error converting to JSON: " + err.Error()
	}

	jsonschema := MarshalToSchema(v)
	jsonSchemaBytes, err := json.MarshalIndent(jsonschema, "", "  ")
	if err != nil {
		return "Error converting schema to JSON: " + err.Error()
	}

	return fmt.Sprintf(`
**JSON Values:**
`+"```"+`
%s
`+"```"+`
**JSON Schema Definition:**
`+"```"+`
%s
`+"```", string(jsonBytes), string(jsonSchemaBytes))
}

func toJSON(v interface{}) string {
	jsonBytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "Error converting to JSON: " + err.Error()
	}

	return fmt.Sprintf(`
**JSON Values:**
`+"```"+`
%s
`+"```", string(jsonBytes))
}

var funcMap = template.FuncMap{
	"toJSON":        toJSON,
	"toJSONwSchema": toJSONwSchema,
}
