package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mhrlife/goai-kit/embedding"
	"github.com/mhrlife/goai-kit/kit"
)

func main() {
	client := kit.NewClient(
		kit.WithAPIKey(os.Getenv("LLM_COURSE_OPENROUTER_API_KEY")),
		kit.WithBaseURL("https://openrouter.ai/api/v1"),
		kit.WithDefaultModel("openai/gpt-4o-mini"),
	)

	embeddingModel := embedding.NewOpenAIEmbeddings(client, "text-embedding-3-small")

	embeddings, err := embeddingModel.EmbedTexts(context.Background(), []string{"Hello world", "Go is awesome!"})
	if err != nil {
		panic(err)
	}

	fmt.Println("Generated ", len(embeddings), " embeddings")
	fmt.Println("Each embedding has dimension ", len(embeddings[0]))

}
