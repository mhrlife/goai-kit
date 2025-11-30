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
		"products_index", embeddingModel, redis.NewClient(
			&redis.Options{Addr: "localhost:6379", Protocol: 2}),
	)

	// Create index with filterable fields: category (tag) and price (numeric)
	if err := vectorDB.CreateIndex(
		context.Background(),
		vectordb.IndexConfig{
			Dimensions:     1536,
			DistanceMetric: "COSINE",
			FilterableFields: []vectordb.FilterableField{
				{Name: "category", Type: vectordb.FilterFieldTypeTag},
				{Name: "price", Type: vectordb.FilterFieldTypeNumeric},
			},
		},
	); err != nil {
		panic(err)
	}

	// Store products with category and price metadata
	err := vectorDB.StoreDocumentsBatch(context.Background(), []vectordb.Document{
		{
			ID:      "macbook-pro",
			Content: "MacBook Pro 16 inch laptop with M3 chip, great for developers",
			Meta:    map[string]any{"category": "laptop", "price": 2499},
		},
		{
			ID:      "thinkpad",
			Content: "Lenovo ThinkPad X1 Carbon, lightweight business laptop",
			Meta:    map[string]any{"category": "laptop", "price": 1299},
		},
		{
			ID:      "iphone",
			Content: "iPhone 15 Pro smartphone with A17 chip",
			Meta:    map[string]any{"category": "phone", "price": 999},
		},
		{
			ID:      "pixel",
			Content: "Google Pixel 8 smartphone with great camera",
			Meta:    map[string]any{"category": "phone", "price": 699},
		},
		{
			ID:      "airpods",
			Content: "AirPods Pro wireless earbuds with noise cancellation",
			Meta:    map[string]any{"category": "audio", "price": 249},
		},
		{
			ID:      "sony-headphones",
			Content: "Sony WH-1000XM5 over-ear headphones with best-in-class ANC",
			Meta:    map[string]any{"category": "audio", "price": 349},
		},
	})
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	// 1. Search all products (no filter)
	fmt.Println("=== All products matching 'portable device' ===")
	printResults(vectorDB, ctx, vectordb.DocumentSearch{
		Query: "portable device for work",
		TopK:  4,
	})

	// 2. Filter by category (tag filter)
	fmt.Println("\n=== Only laptops ===")
	printResults(vectorDB, ctx, vectordb.DocumentSearch{
		Query: "portable device for work",
		TopK:  4,
		Filters: []vectordb.Filter{
			{Field: "category", Operator: vectordb.FilterOpEq, Value: "laptop"},
		},
	})

	// 3. Filter by price range (numeric range filter)
	fmt.Println("\n=== Products between $200 and $500 ===")
	printResults(vectorDB, ctx, vectordb.DocumentSearch{
		Query: "good audio quality",
		TopK:  4,
		Filters: []vectordb.Filter{
			{Field: "price", Operator: vectordb.FilterOpRange, Value: vectordb.NumericRange{Min: 200, Max: 500}},
		},
	})

	// 4. Filter by price >= (gte filter)
	fmt.Println("\n=== Products $1000 or more ===")
	printResults(vectorDB, ctx, vectordb.DocumentSearch{
		Query: "powerful device",
		TopK:  4,
		Filters: []vectordb.Filter{
			{Field: "price", Operator: vectordb.FilterOpGte, Value: 1000},
		},
	})

	// 5. Filter by price <= (lte filter)
	fmt.Println("\n=== Budget products under $800 ===")
	printResults(vectorDB, ctx, vectordb.DocumentSearch{
		Query: "best value",
		TopK:  4,
		Filters: []vectordb.Filter{
			{Field: "price", Operator: vectordb.FilterOpLte, Value: 800},
		},
	})

	// 6. Combined filters: category AND price range
	fmt.Println("\n=== Phones under $800 ===")
	printResults(vectorDB, ctx, vectordb.DocumentSearch{
		Query: "smartphone",
		TopK:  4,
		Filters: []vectordb.Filter{
			{Field: "category", Operator: vectordb.FilterOpEq, Value: "phone"},
			{Field: "price", Operator: vectordb.FilterOpLte, Value: 800},
		},
	})
}

func printResults(db *vectordb.RedisVectorDB, ctx context.Context, search vectordb.DocumentSearch) {
	docs, err := db.SearchDocuments(ctx, search)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	if len(docs) == 0 {
		fmt.Println("No results found")
		return
	}
	for _, doc := range docs {
		fmt.Printf("  %s (score: %s, price: $%v, category: %v)\n",
			doc.ID, doc.Score, doc.Meta["price"], doc.Meta["category"])
	}
}
