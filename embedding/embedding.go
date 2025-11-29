package embedding

import "context"

type Client interface {
	EmbedTexts(ctx context.Context, texts []string) ([][]float64, error)
}
