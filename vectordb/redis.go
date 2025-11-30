package vectordb

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/mhrlife/goai-kit/embedding"
	"github.com/redis/go-redis/v9"
)

type RedisVectorDB struct {
	index       string
	embedClient embedding.Client
	client      *redis.Client
	indexConfig *IndexConfig
}

func NewRedisVectorDB(index string, embeddingClient embedding.Client, redisClient *redis.Client) *RedisVectorDB {
	return &RedisVectorDB{
		index:       index,
		embedClient: embeddingClient,
		client:      redisClient,
		indexConfig: nil,
	}
}

func (r *RedisVectorDB) CreateIndex(ctx context.Context, config IndexConfig) error {
	if config.Dimensions <= 0 {
		return fmt.Errorf("dimensions must be positive, got %d", config.Dimensions)
	}

	distanceMetric := config.DistanceMetric
	if distanceMetric == "" {
		distanceMetric = "COSINE"
	}

	dataType := "FLOAT32"

	validMetrics := map[string]bool{"L2": true, "COSINE": true, "IP": true}
	if !validMetrics[distanceMetric] {
		return fmt.Errorf("invalid distance metric: %s (must be L2, COSINE, or IP)", distanceMetric)
	}

	// Build field schemas
	fields := []*redis.FieldSchema{
		{
			FieldName: "content",
			FieldType: redis.SearchFieldTypeText,
		},
		{
			FieldName: "embedding",
			FieldType: redis.SearchFieldTypeVector,
			VectorArgs: &redis.FTVectorArgs{
				HNSWOptions: &redis.FTHNSWOptions{
					Dim:            config.Dimensions,
					DistanceMetric: distanceMetric,
					Type:           dataType,
				},
			},
		},
	}

	// Add filterable fields to schema
	for _, f := range config.FilterableFields {
		fieldName := "meta_" + f.Name
		var schema *redis.FieldSchema
		switch f.Type {
		case FilterFieldTypeText:
			schema = &redis.FieldSchema{
				FieldName: fieldName,
				FieldType: redis.SearchFieldTypeText,
			}
		case FilterFieldTypeTag:
			schema = &redis.FieldSchema{
				FieldName: fieldName,
				FieldType: redis.SearchFieldTypeTag,
			}
		case FilterFieldTypeNumeric:
			schema = &redis.FieldSchema{
				FieldName: fieldName,
				FieldType: redis.SearchFieldTypeNumeric,
			}
		default:
			return fmt.Errorf("unsupported filter field type: %s", f.Type)
		}
		fields = append(fields, schema)
	}

	err := r.client.FTCreate(
		ctx,
		r.index,
		&redis.FTCreateOptions{
			OnHash: true,
			Prefix: []interface{}{r.index + ":"},
		},
		fields...,
	).Err()

	if err != nil && !strings.Contains(err.Error(), "Index already exists") {
		return fmt.Errorf("failed to create index: %w", err)
	}

	r.indexConfig = &config
	return nil
}

func (r *RedisVectorDB) StoreDocument(ctx context.Context, doc Document) error {
	if r.indexConfig == nil {
		return fmt.Errorf("index not created: call CreateIndex first")
	}

	embeddings, err := r.embedClient.EmbedTexts(ctx, []string{fmt.Sprintf("%s:%s", doc.ID, doc.Content)})
	if err != nil {
		return fmt.Errorf("failed to embed document: %w", err)
	}

	vec := embeddings[0]

	if len(vec) != r.indexConfig.Dimensions {
		return fmt.Errorf("embedding dimension mismatch: got %d, expected %d",
			len(vec), r.indexConfig.Dimensions)
	}

	embedding32 := make([]float32, len(vec))
	for i, v := range vec {
		embedding32[i] = float32(v)
	}
	b, _ := json.Marshal(doc.Meta)

	docData := map[string]interface{}{
		"id":        doc.ID,
		"content":   doc.Content,
		"metadata":  string(b),
		"embedding": encodeFloat32Vector(embedding32),
	}

	// Add filterable metadata fields with meta_ prefix
	for _, f := range r.indexConfig.FilterableFields {
		if val, ok := doc.Meta[f.Name]; ok {
			docData["meta_"+f.Name] = val
		}
	}

	key := fmt.Sprintf("%s:%s", r.index, doc.ID)
	err = r.client.HSet(ctx, key, docData).Err()
	if err != nil {
		return fmt.Errorf("failed to store document: %w", err)
	}

	return nil
}

func (r *RedisVectorDB) StoreDocumentsBatch(ctx context.Context, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}

	if r.indexConfig == nil {
		return fmt.Errorf("index not created: call CreateIndex first")
	}

	contents := make([]string, len(docs))
	for i, doc := range docs {
		contents[i] = fmt.Sprintf("#%s\n%s", doc.ID, doc.Content)
	}

	embeddings, err := r.embedClient.EmbedTexts(ctx, contents)
	if err != nil {
		return fmt.Errorf("failed to embed documents: %w", err)
	}

	pipe := r.client.Pipeline()

	for i, doc := range docs {
		vec := embeddings[i]

		if len(vec) != r.indexConfig.Dimensions {
			return fmt.Errorf("document %s: embedding dimension mismatch: got %d, expected %d",
				doc.ID, len(vec), r.indexConfig.Dimensions)
		}

		embedding32 := make([]float32, len(vec))
		for j, v := range vec {
			embedding32[j] = float32(v)
		}

		b, _ := json.Marshal(doc.Meta)

		docData := map[string]interface{}{
			"id":        doc.ID,
			"content":   doc.Content,
			"metadata":  string(b),
			"embedding": encodeFloat32Vector(embedding32),
		}

		// Add filterable metadata fields with meta_ prefix
		for _, f := range r.indexConfig.FilterableFields {
			if val, ok := doc.Meta[f.Name]; ok {
				docData["meta_"+f.Name] = val
			}
		}

		key := fmt.Sprintf("%s:%s", r.index, doc.ID)
		pipe.HSet(ctx, key, docData)
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store batch: %w", err)
	}

	return nil
}

func (r *RedisVectorDB) UpdateDocument(ctx context.Context, doc Document) error {
	return r.StoreDocument(ctx, doc)
}

func (r *RedisVectorDB) DeleteDocument(ctx context.Context, id string) error {
	key := fmt.Sprintf("%s:%s", r.index, id)
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	return nil
}

func (r *RedisVectorDB) SearchDocuments(ctx context.Context, search DocumentSearch) ([]DocumentWithScore, error) {
	if r.indexConfig == nil {
		return []DocumentWithScore{}, fmt.Errorf("index not created: call CreateIndex first")
	}

	if search.TopK <= 0 {
		return []DocumentWithScore{}, fmt.Errorf("TopK must be positive, got %d", search.TopK)
	}

	if search.Query == "" {
		return []DocumentWithScore{}, fmt.Errorf("query cannot be empty")
	}

	embeddings, err := r.embedClient.EmbedTexts(ctx, []string{search.Query})
	if err != nil {
		return []DocumentWithScore{}, fmt.Errorf("failed to embed query: %w", err)
	}

	queryVec := embeddings[0]

	if len(queryVec) != r.indexConfig.Dimensions {
		return []DocumentWithScore{}, fmt.Errorf("query vector dimension mismatch: got %d, expected %d",
			len(queryVec), r.indexConfig.Dimensions)
	}

	queryVec32 := make([]float32, len(queryVec))
	for i, v := range queryVec {
		queryVec32[i] = float32(v)
	}

	// Build filter prefix
	filterPrefix := "*"
	if len(search.Filters) > 0 {
		filterPrefix = r.buildFilterQuery(search.Filters)
	}

	query := fmt.Sprintf("%s=>[KNN %d @embedding $vec AS score]", filterPrefix, search.TopK)

	result, err := r.client.FTSearchWithArgs(
		ctx,
		r.index,
		query,
		&redis.FTSearchOptions{
			DialectVersion: 2,
			Params: map[string]interface{}{
				"vec": encodeFloat32Vector(queryVec32),
			},
			Return: []redis.FTSearchReturn{
				{FieldName: "id"},
				{FieldName: "content"},
				{FieldName: "metadata"},
				{FieldName: "score"},
			},
		},
	).Result()

	if err != nil {
		return []DocumentWithScore{}, fmt.Errorf("failed to search: %w", err)
	}

	docs := make([]DocumentWithScore, 0, len(result.Docs))

	for _, doc := range result.Docs {
		var id, content string
		if v, ok := doc.Fields["id"]; ok {
			id = v
		}
		if v, ok := doc.Fields["content"]; ok {
			content = v
		}

		metadata := make(map[string]interface{})
		if v, ok := doc.Fields["metadata"]; ok && v != "" {
			err := json.Unmarshal([]byte(v), &metadata)
			if err != nil {
				return []DocumentWithScore{}, fmt.Errorf("failed to unmarshal metadata for doc %s: %w", id, err)
			}
		}

		docs = append(docs, DocumentWithScore{
			Document: Document{
				ID:      id,
				Content: content,
				Meta:    metadata,
			},
			Score: doc.Fields["score"],
		})
	}

	return docs, nil
}

func encodeFloat32Vector(fs []float32) []byte {
	buf := make([]byte, len(fs)*4)

	for i, f := range fs {
		u := math.Float32bits(f)
		binary.NativeEndian.PutUint32(buf[i*4:], u)
	}

	return buf
}

// buildFilterQuery constructs a Redis Search filter query from filters
func (r *RedisVectorDB) buildFilterQuery(filters []Filter) string {
	if len(filters) == 0 {
		return "*"
	}

	parts := make([]string, 0, len(filters))
	for _, f := range filters {
		fieldName := "meta_" + f.Field
		var part string

		switch f.Operator {
		case FilterOpEq:
			// Tag exact match: @field:{value}
			part = fmt.Sprintf("@%s:{%v}", fieldName, escapeTagValue(f.Value))
		case FilterOpIn:
			// Tag in list: @field:{val1|val2|val3}
			if vals, ok := f.Value.([]string); ok {
				escaped := make([]string, len(vals))
				for i, v := range vals {
					escaped[i] = escapeTagValue(v)
				}
				part = fmt.Sprintf("@%s:{%s}", fieldName, strings.Join(escaped, "|"))
			}
		case FilterOpContains:
			// Text contains: @field:value
			part = fmt.Sprintf("@%s:%v", fieldName, f.Value)
		case FilterOpRange:
			// Numeric range: @field:[min max]
			if rng, ok := f.Value.(NumericRange); ok {
				part = fmt.Sprintf("@%s:[%v %v]", fieldName, rng.Min, rng.Max)
			}
		case FilterOpGte:
			// Numeric >=: @field:[value +inf]
			part = fmt.Sprintf("@%s:[%v +inf]", fieldName, f.Value)
		case FilterOpLte:
			// Numeric <=: @field:[-inf value]
			part = fmt.Sprintf("@%s:[-inf %v]", fieldName, f.Value)
		}

		if part != "" {
			parts = append(parts, part)
		}
	}

	if len(parts) == 0 {
		return "*"
	}

	// Combine with AND (space separated in Redis Search)
	return "(" + strings.Join(parts, " ") + ")"
}

// escapeTagValue escapes special characters in tag values for Redis Search
func escapeTagValue(v interface{}) string {
	s := fmt.Sprintf("%v", v)
	// Escape special characters in Redis Search tag syntax
	replacer := strings.NewReplacer(
		",", "\\,",
		".", "\\.",
		"<", "\\<",
		">", "\\>",
		"{", "\\{",
		"}", "\\}",
		"[", "\\[",
		"]", "\\]",
		"\"", "\\\"",
		"'", "\\'",
		":", "\\:",
		";", "\\;",
		"!", "\\!",
		"@", "\\@",
		"#", "\\#",
		"$", "\\$",
		"%", "\\%",
		"^", "\\^",
		"&", "\\&",
		"*", "\\*",
		"(", "\\(",
		")", "\\)",
		"-", "\\-",
		"+", "\\+",
		"=", "\\=",
		"~", "\\~",
		" ", "\\ ",
	)
	return replacer.Replace(s)
}
