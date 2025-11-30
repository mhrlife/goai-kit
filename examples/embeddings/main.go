package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mhrlife/goai-kit/embedding"
	"github.com/mhrlife/goai-kit/kit"
	"github.com/mhrlife/goai-kit/vectordb"
	"github.com/redis/go-redis/v9"
)

func main() {
	client := kit.NewClient(
		kit.WithAPIKey(os.Getenv("LLM_COURSE_OPENROUTER_API_KEY")),
		kit.WithBaseURL("https://openrouter.ai/api/v1"),
		kit.WithDefaultModel("openai/gpt-4o-mini"),
	)

	embeddingModel := embedding.NewOpenAIEmbeddings(client, "text-embedding-3-small")

	vectorDB := vectordb.NewRedisVectorDB(
		"my_redis_index", embeddingModel, redis.NewClient(
			&redis.Options{Addr: "localhost:6379", Protocol: 2}),
	)

	if err := vectorDB.CreateIndex(
		context.Background(),
		vectordb.IndexConfig{
			Dimensions:     1536,
			DistanceMetric: "COSINE",
			FilterableFields: []vectordb.FilterableField{
				{Name: "category", Type: vectordb.FilterFieldTypeTag},
			},
		},
	); err != nil {
		panic(err)
	}

	err := vectorDB.StoreDocumentsBatch(context.Background(), []vectordb.Document{
		{
			ID:      "php",
			Content: "php is a backend language",
			Meta:    map[string]any{"category": "backend"},
		},
		{
			ID:      "go",
			Content: "go is a backend language",
			Meta:    map[string]any{"category": "backend"},
		},
		{
			ID:      "javascript",
			Content: "javascript is a frontend language",
			Meta:    map[string]any{"category": "frontend"},
		},
		{
			ID:      "typescript",
			Content: "typescript is a frontend language",
			Meta:    map[string]any{"category": "frontend"},
		},
		{
			ID:      "python",
			Content: "python is a dynamically typed language for backend development",
			Meta:    map[string]any{"category": "backend"},
		},
	})
	if err != nil {
		panic(err)
	}

	searchForQuery := func(query string) {
		documents, err := vectorDB.SearchDocuments(context.Background(), vectordb.DocumentSearch{
			Query: query,
			TopK:  2,
		})
		if err != nil {
			panic(err)
		}

		fmt.Println("> Results for query:", query)
		for _, doc := range documents {
			fmt.Printf("ID: %s, Score: %s, Content: %s, Meta: %v\n", doc.ID, doc.Score, doc.Content, doc.Meta)
		}
	}

	searchForQuery("a backend language")
	//ID: php, Score: 0.338148355484, Content: php is a backend language, Meta: map[category:backend]
	//ID: go, Score: 0.403613626957, Content: go is a backend language, Meta: map[category:backend]
	fmt.Println("-----")
	searchForQuery("a frontend language")
	//ID: javascript, Score: 0.268988072872, Content: javascript is a frontend language, Meta: map[category:frontend]
	//ID: typescript, Score: 0.354185700417, Content: typescript is a frontend language, Meta: map[category:frontend]
	fmt.Println("-----")
	searchForQuery("a dynamically typed language")
	//ID: typescript, Score: 0.43790769577, Content: typescript is a frontend language, Meta: map[category:frontend]
	//ID: python, Score: 0.490692019463, Content: python is a dynamically typed language for backend development, Meta: map[category:backend]
	fmt.Println("-----")
	searchForQuery("I'm looking for a language that is fast and efficient")
	//ID: go, Score: 0.626137971878, Content: go is a backend language, Meta: map[category:backend]
	//ID: javascript, Score: 0.597403407097, Content: javascript is a frontend language, Meta: map[category:frontend]

	// Search with filters - only backend languages
	fmt.Println("-----")
	fmt.Println("> Filtered search: backend languages only")
	docs, err := vectorDB.SearchDocuments(context.Background(), vectordb.DocumentSearch{
		Query: "a fast language",
		TopK:  3,
		Filters: []vectordb.Filter{
			{Field: "category", Operator: vectordb.FilterOpEq, Value: "backend"},
		},
	})
	if err != nil {
		panic(err)
	}
	for _, doc := range docs {
		fmt.Printf("ID: %s, Score: %s, Content: %s, Meta: %v\n", doc.ID, doc.Score, doc.Content, doc.Meta)
	}

	// Search with filters - only frontend languages
	fmt.Println("-----")
	fmt.Println("> Filtered search: frontend languages only")
	docs, err = vectorDB.SearchDocuments(context.Background(), vectordb.DocumentSearch{
		Query: "a fast language",
		TopK:  3,
		Filters: []vectordb.Filter{
			{Field: "category", Operator: vectordb.FilterOpEq, Value: "frontend"},
		},
	})
	if err != nil {
		panic(err)
	}
	for _, doc := range docs {
		fmt.Printf("ID: %s, Score: %s, Content: %s, Meta: %v\n", doc.ID, doc.Score, doc.Content, doc.Meta)
	}
}
