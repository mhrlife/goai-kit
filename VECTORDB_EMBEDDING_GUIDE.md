# VectorDB & Embedding Packages - Architecture Guide

This guide explains the design and implementation of the `embedding` and `vectordb` packages for porting to PHP or other languages.

---

## 1. Embedding Package (`embedding/`)

### Overview
The embedding package is responsible for converting text into numerical vectors (embeddings) using an external embedding service (OpenAI).

### Interface Design

```go
type Client interface {
    EmbedTexts(ctx context.Context, texts []string) ([][]float64, error)
}
```

**Interface Requirements:**
- Accept multiple texts at once (batch operation)
- Return 2D array of float64 (vector per text)
- Error handling for API failures

### Implementation: OpenAI Embeddings

**Structure:**
```go
type OpenAIEmbeddings struct {
    client openai.Client  // OpenAI API client
    model  string         // Model name (e.g., "text-embedding-3-small")
}
```

**Constructor:**
```go
NewOpenAIEmbeddings(client *kit.Client, model string) *OpenAIEmbeddings
```
- Takes an existing kit.Client (handles auth/config)
- Model parameter (defaults to "text-embedding-3-small")

**Core Method: `EmbedTexts`**

```go
func (o *OpenAIEmbeddings) EmbedTexts(ctx context.Context, texts []string) ([][]float64, error)
```

**Logic:**
1. Accept batch of texts (can be empty, returns empty result)
2. Call OpenAI Embeddings API with texts and model
3. Extract embedding vectors from response
4. Return as 2D array: `[][]float64` (array of vectors)

**Example Output:**
```
texts = ["hello", "world"]
→ [
    [0.123, 0.456, 0.789, ...],  // 1536 dimensions for text-embedding-3-small
    [0.234, 0.567, 0.890, ...]
  ]
```

### Key Points for PHP Implementation

1. **Interface**: Create an abstract `EmbeddingClient` class with method `embedTexts(array $texts)`
2. **OpenAI Implementation**:
   - Use OpenAI's PHP SDK
   - Batch multiple texts in single API call for efficiency
   - Return 2D array of floats
3. **Error Handling**: Catch API errors and propagate with context
4. **Dimensions**: Different models have different dimensions (text-embedding-3-small = 1536)

---

## 2. VectorDB Package (`vectordb/`)

### Overview
The vectordb package stores embeddings in a database (Redis) and provides semantic search capabilities using vector similarity.

### Data Structures

**Document:**
```go
type Document struct {
    ID      string             // Unique identifier
    Content string             // Original text content
    Meta    map[string]any     // Custom metadata (key-value pairs)
}
```

**DocumentWithScore:**
```go
type DocumentWithScore struct {
    Document              // Embedded Document
    Score    string       // Similarity score from search
}
```

**DocumentSearch:**
```go
type DocumentSearch struct {
    Query string  // Search query (will be embedded)
    TopK  int     // Number of results to return
}
```

**IndexConfig:**
```go
type IndexConfig struct {
    Dimensions     int     // Vector dimensions (e.g., 1536)
    DistanceMetric string  // "COSINE", "L2", or "IP"
}
```

### Interface Design

```go
type Client interface {
    CreateIndex(ctx context.Context, config IndexConfig) error
    StoreDocument(ctx context.Context, doc Document) error
    StoreDocumentsBatch(ctx context.Context, docs []Document) error
    UpdateDocument(ctx context.Context, doc Document) error
    DeleteDocument(ctx context.Context, id string) error
    SearchDocuments(ctx context.Context, search DocumentSearch) ([]DocumentWithScore, error)
}
```

**Required Methods:**
1. **CreateIndex** - Initialize the vector database with dimensions and distance metric
2. **StoreDocument** - Save a single document with its embedding
3. **StoreDocumentsBatch** - Save multiple documents efficiently
4. **UpdateDocument** - Modify an existing document
5. **DeleteDocument** - Remove a document by ID
6. **SearchDocuments** - Find similar documents using semantic search

### Implementation: Redis VectorDB

**Structure:**
```go
type RedisVectorDB struct {
    index       string                    // Index name (namespace)
    embedClient embedding.Client          // Embedding service reference
    client      *redis.Client             // Redis connection
    indexConfig *IndexConfig              // Stored index configuration
}
```

**Constructor:**
```go
NewRedisVectorDB(index string, embeddingClient embedding.Client, redisClient *redis.Client)
```
- Takes index name (Redis namespace)
- Takes embedding client (dependency injection)
- Takes Redis client connection

### Key Implementation Details

#### 1. CreateIndex

**What it does:**
- Creates a Redis Search index using HNSW (Hierarchical Navigable Small World) algorithm
- Configures vector field with dimensions and distance metric
- Validates distance metric (COSINE, L2, or IP)
- Prefixes all keys with `{index}:` for namespace isolation

**Redis Schema:**
```
Field: "content"    → Text field (for future full-text search)
Field: "embedding"  → Vector field (HNSW, FLOAT32, K-NN search)
Field: "id"         → Document ID
Field: "metadata"   → JSON string
```

#### 2. StoreDocument / StoreDocumentsBatch

**Process:**
1. **Check Index**: Verify index was created
2. **Embed Content**: Call embedding service to get vectors
3. **Validate Dimensions**: Ensure embedding matches configured dimensions
4. **Convert to Float32**: OpenAI returns float64, convert to float32 for Redis
5. **Encode Vector**: Binary encode float32 values (4 bytes each)
6. **Store in Redis Hash**:
   ```
   key: "{index}:{docID}"
   fields: {id, content, metadata (JSON), embedding (binary)}
   ```
7. **Batch Optimization**: Use Redis pipeline for multiple documents

**Vector Encoding:**
```
float32 array → binary (uses NativeEndian for system byte order)
Example: [0.123, 0.456] → 8 bytes (4 bytes per float32)
```

#### 3. SearchDocuments

**Process:**
1. **Validate Input**: Check TopK > 0 and Query not empty
2. **Embed Query**: Convert search query to vector
3. **Validate Dimensions**: Ensure query vector matches index dimensions
4. **KNN Search**: Use Redis FT.SEARCH with KNN query
   ```
   Query: "*=>[KNN {TopK} @embedding $vec AS score]"
   ```
5. **Parse Results**: Extract documents with similarity scores
6. **Return**: Array of DocumentWithScore with original content and metadata

**Similarity Score:**
- Redis returns distance scores (not similarity directly)
- Lower scores = more similar (for COSINE metric)
- Scores are string format in Redis

#### 4. UpdateDocument

**Implementation:**
- Simply calls StoreDocument (overwrites existing document)
- Re-embeds the content and updates vector

#### 5. DeleteDocument

**Implementation:**
- Removes the Redis hash key `{index}:{docID}`
- Simple key deletion

### Data Flow Diagram

```
User Input
    ↓
StoreDocument(doc)
    ↓
EmbeddingClient.EmbedTexts(doc.Content)  ← Gets vector
    ↓
Validate vector dimensions
    ↓
Convert to Float32 & encode binary
    ↓
Store in Redis Hash with key "{index}:{id}"


SearchDocuments(query)
    ↓
EmbeddingClient.EmbedTexts(query)  ← Gets query vector
    ↓
Validate query vector dimensions
    ↓
Redis FT.SEARCH with KNN
    ↓
Parse Redis results
    ↓
Return DocumentWithScore[] with scores
```

---

## 3. Key Design Patterns

### Dependency Injection
- VectorDB depends on embedding.Client (interface)
- Allows swapping different embedding implementations
- In PHP: use interfaces/traits for same pattern

### Interface Segregation
- Embedding package: single responsibility (text → vector)
- VectorDB package: storage and search abstraction
- Allows multiple implementations (Redis, Pinecone, Milvus, etc.)

### Error Handling
- Validate state before operations (index created?)
- Wrap errors with context
- In PHP: use exceptions with descriptive messages

### Batch Operations
- StoreDocumentsBatch uses pipeline for efficiency
- Single API call for embeddings (batch)
- Reduces latency and cost

---

## 4. Integration Example

```go
// 1. Create embedding client
embeddingClient := embedding.NewOpenAIEmbeddings(kitClient, "text-embedding-3-small")

// 2. Create vector database
vectorDB := vectordb.NewRedisVectorDB("my_index", embeddingClient, redisClient)

// 3. Initialize index
vectorDB.CreateIndex(ctx, vectordb.IndexConfig{
    Dimensions: 1536,
    DistanceMetric: "COSINE",
})

// 4. Store documents (auto-embedded)
vectorDB.StoreDocumentsBatch(ctx, []vectordb.Document{
    {ID: "doc1", Content: "Go is fast", Meta: map[string]any{"lang": "go"}},
    {ID: "doc2", Content: "PHP is flexible", Meta: map[string]any{"lang": "php"}},
})

// 5. Search (auto-embeds query, finds similar)
results, _ := vectorDB.SearchDocuments(ctx, vectordb.DocumentSearch{
    Query: "fast programming language",
    TopK: 2,
})

// Results: [{ID: "doc1", Content: "Go is fast", Score: "0.338"}, ...]
```

---

## 5. PHP Implementation Checklist

### Embedding Package
- [ ] Create `EmbeddingClient` interface
- [ ] Implement `OpenAIEmbeddings` class
- [ ] Use OpenAI PHP SDK (or cURL)
- [ ] Return float array (2D)
- [ ] Handle batch texts efficiently

### VectorDB Package
- [ ] Create data models: `Document`, `DocumentWithScore`, `DocumentSearch`, `IndexConfig`
- [ ] Create `VectorDBClient` interface
- [ ] Implement `RedisVectorDB` class with all 6 methods
- [ ] Use Redis PHP library with vector search support
- [ ] Handle vector encoding (float32 binary)
- [ ] Implement KNN search query building
- [ ] Add proper error handling and validation

### Redis Setup
- [ ] Use Redis 7.0+ with Search module
- [ ] Index names with prefix format: `{indexName}:{docID}`
- [ ] HNSW algorithm configuration (dimensions, distance metric)
- [ ] Pipeline support for batch operations

### Testing Scenarios
- [ ] Store and retrieve single document
- [ ] Batch store documents
- [ ] Search with similarity scores
- [ ] Metadata preservation through search
- [ ] Different distance metrics (COSINE, L2, IP)
- [ ] Error cases (index not created, dimension mismatch)
