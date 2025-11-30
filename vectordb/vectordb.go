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
	Query   string
	TopK    int
	Filters []Filter
}

// Filter represents a search filter condition
type Filter struct {
	Field    string      // Metadata field name to filter on
	Operator FilterOp   // Filter operator
	Value    interface{} // Value to compare against
}

// FilterOp represents the filter operation type
type FilterOp string

const (
	FilterOpEq       FilterOp = "eq"       // Equals (text/tag match)
	FilterOpIn       FilterOp = "in"       // In list of values (tag match)
	FilterOpRange    FilterOp = "range"    // Numeric range [min, max]
	FilterOpGte      FilterOp = "gte"      // Greater than or equal
	FilterOpLte      FilterOp = "lte"      // Less than or equal
	FilterOpContains FilterOp = "contains" // Text contains
)

// NumericRange represents a numeric range for filtering
type NumericRange struct {
	Min float64
	Max float64
}

type IndexConfig struct {
	Dimensions       int
	DistanceMetric   string
	FilterableFields []FilterableField // Metadata fields that can be filtered
}

// FilterableField defines a metadata field that can be filtered
type FilterableField struct {
	Name string          // Field name in metadata
	Type FilterFieldType // Field type for indexing
}

// FilterFieldType represents the type of a filterable field
type FilterFieldType string

const (
	FilterFieldTypeText    FilterFieldType = "text"    // Full-text searchable
	FilterFieldTypeTag     FilterFieldType = "tag"     // Exact match (like category)
	FilterFieldTypeNumeric FilterFieldType = "numeric" // Numeric range queries
)

type Client interface {
	CreateIndex(ctx context.Context, config IndexConfig) error
	StoreDocument(ctx context.Context, doc Document) error
	StoreDocumentsBatch(ctx context.Context, docs []Document) error
	UpdateDocument(ctx context.Context, doc Document) error
	DeleteDocument(ctx context.Context, id string) error
	SearchDocuments(ctx context.Context, search DocumentSearch) ([]DocumentWithScore, error)
}
