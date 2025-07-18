package goaikit

import (
	"embed"
	"fmt"
	"github.com/stretchr/testify/require"
	"log/slog"
	"testing"
)

//go:embed fixture/template/**.tpl
var tplFS embed.FS

func TestRender(t *testing.T) {
	type Context struct {
		Ready bool
	}

	tpl := NewTemplate[Context]()
	err := tpl.Load(tplFS)
	require.NoError(t, err)

	rendered, err := tpl.Execute("hello", Render[Context]{
		Data: map[string]any{
			"Name": "World",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "Hello World", rendered)

	rendered, err = tpl.Execute("hello", Render[Context]{
		Context: Context{
			Ready: true,
		},
		Data: map[string]any{
			"Name": "Amir",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "Ready: Hello Amir", rendered)
}

func TestWithNested(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	type Context struct {
		Nested bool
		Ready  bool
	}

	tpl := NewTemplate[Context]()
	err := tpl.Load(tplFS)
	require.NoError(t, err)

	rendered, err := tpl.Execute("hello", Render[Context]{
		Context: Context{
			Nested: true,
		},
		Data: map[string]any{
			"Name": "Reza",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "Hello Reza\nAlso supports nested", rendered)
}

func TestToJson(t *testing.T) {
	type Context struct {
		Name string `json:"name" jsonschema_description:"The name of the user"`
		Age  int    `json:"age"`
	}

	tpl := NewTemplate[Context]()
	err := tpl.Load(tplFS)
	require.NoError(t, err)

	render, err := tpl.Execute("json", Render[Context]{
		Context: Context{
			Name: "Ali",
			Age:  20,
		},
	})
	require.NoError(t, err)

	fmt.Println(render)
}
