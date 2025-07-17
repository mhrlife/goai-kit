package goaikit

import (
	"embed"
	"github.com/stretchr/testify/require"
	"testing"
)

//go:embed fixture/template/*.tpl
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
