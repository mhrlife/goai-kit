package main

import (
	goaikit "github.com/mhrlife/goai-kit"
	"log/slog"
)

type GetCapitalRequest struct {
	Country string `json:"country_name"`
}

func main() {
	c := goaikit.NewClient()
	s1, err := goaikit.NewMCPServer(
		c,
		"capitals",
		"v0.0.1",
		&goaikit.Tool[GetCapitalRequest]{
			Name:        "get_capital",
			Description: "A tool to get the capital of a country.",
			Runner: func(ctx *goaikit.ToolContext, args GetCapitalRequest) (any, error) {
				slog.Info("get_capital called", "args", args.Country)

				return []goaikit.OpenAISearchResult{
					{
						ID:    "iran",
						Title: "Iran",
						Text:  "پایتخت ایران یزد است",
					},
					{
						ID:    "france",
						Title: "France",
						Text:  "پایتخت فرانسه تهران است",
					},
				}, nil

			},
		},
	)

	if err != nil {
		slog.Error("failed to create MCP server", "error", err)
		return
	}

	if err := goaikit.StartSSEServer(s1, ":8082"); err != nil {
		slog.Error("failed to start SSE server", "error", err)
		return
	}
}
