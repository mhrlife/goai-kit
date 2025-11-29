package embedding

import (
	"context"

	"github.com/mhrlife/goai-kit/kit"
	"github.com/openai/openai-go"
)

type OpenAIEmbeddings struct {
	client openai.Client
	model  string
}

// NewOpenAIEmbeddings creates a new OpenAI embeddings client.
// If model is empty, defaults to "text-embedding-3-small".
func NewOpenAIEmbeddings(client *kit.Client, model string) *OpenAIEmbeddings {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAIEmbeddings{
		client: client.GetOpenAI(),
		model:  model,
	}
}

func (o *OpenAIEmbeddings) EmbedTexts(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return [][]float64{}, nil
	}

	resp, err := o.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: texts,
		},
		Model: o.model,
	})
	if err != nil {
		return nil, err
	}

	// Extract embeddings from response
	embeddings := make([][]float64, len(resp.Data))
	for i, data := range resp.Data {
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}
