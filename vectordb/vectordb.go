package vectordb

import "context"

type Document struct {
	ID      string
	Content string
	Meta    map[string]any
}

type DocumentWithScore struct {
	Document
	Score string
}

type DocumentSearch struct {
	Query    string
	Metadata map[string]any
	TopK     int
}

type IndexConfig struct {
	Dimensions     int
	DistanceMetric string
}

type Client interface {
	CreateIndex(ctx context.Context, config IndexConfig) error
	StoreDocument(ctx context.Context, doc Document) error
	StoreDocumentsBatch(ctx context.Context, docs []Document) error
	UpdateDocument(ctx context.Context, doc Document) error
	DeleteDocument(ctx context.Context, id string) error
	SearchDocuments(ctx context.Context, search DocumentSearch) ([]DocumentWithScore, error)
}
