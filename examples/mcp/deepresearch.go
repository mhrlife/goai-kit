package main

import (
	goaikit "github.com/mhrlife/goai-kit"
	"log/slog"
)

func main() {
	s1, err := goaikit.NewOpenAIDeepResearchMCPServer(
		"capitals",
		"v0.0.1",
		goaikit.OpenAISearch{
			Description: "search for countries",
			Exec: func(query string) ([]goaikit.OpenAISearchResult, error) {
				slog.Info("searching for countries",
					"query", query,
				)

				return []goaikit.OpenAISearchResult{
					{
						ID:    "iran",
						Title: "Iran",
					},
					{
						ID:    "france",
						Title: "France",
					},
				}, nil
			},
		},
		goaikit.OpenAIFetch{
			Description: "get the country info",
			Exec: func(id string) (*goaikit.OpenAISearchResult, error) {
				slog.Info("fetching country info",
					"id", id,
				)

				if id == "iran" {
					return &goaikit.OpenAISearchResult{
						ID:    "iran",
						Title: "Iran",
						Text:  "Iran is a country in Western Asia.",
						URL:   "https://en.wikipedia.org/wiki/Iran",
					}, nil
				} else if id == "france" {
					return &goaikit.OpenAISearchResult{
						ID:    "france",
						Title: "France",
						Text:  "Capital of france is Tehran in this world",
						URL:   "https://en.wikipedia.org/wiki/France",
					}, nil
				}

				return nil, nil
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
