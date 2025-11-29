# PHP Implementation Skeleton

Quick reference for implementing embedding and vectordb in PHP.

---

## 1. Embedding Package Structure

```php
<?php
namespace YourApp\Embedding;

interface EmbeddingClient {
    /**
     * Convert texts to embeddings
     * @param array $texts List of text strings
     * @return array 2D array of floats ([[0.1, 0.2, ...], [0.3, 0.4, ...]])
     */
    public function embedTexts(array $texts): array;
}
```

### OpenAI Implementation

```php
<?php
namespace YourApp\Embedding;

use OpenAI\Client as OpenAIClient;

class OpenAIEmbeddings implements EmbeddingClient {
    private $client;
    private $model;

    public function __construct(OpenAIClient $client, string $model = "text-embedding-3-small") {
        $this->client = $client;
        $this->model = $model ?: "text-embedding-3-small";
    }

    public function embedTexts(array $texts): array {
        if (empty($texts)) {
            return [];
        }

        $response = $this->client->embeddings()->create([
            'model' => $this->model,
            'input' => $texts,
        ]);

        $embeddings = [];
        foreach ($response->data as $data) {
            $embeddings[] = $data['embedding']; // Returns array of floats
        }

        return $embeddings;
    }
}
```

---

## 2. VectorDB Package Structure

### Data Models

```php
<?php
namespace YourApp\VectorDB;

class Document {
    public string $id;
    public string $content;
    public array $meta; // Arbitrary metadata

    public function __construct(string $id, string $content, array $meta = []) {
        $this->id = $id;
        $this->content = $content;
        $this->meta = $meta;
    }
}

class DocumentWithScore extends Document {
    public string $score; // Similarity score from search

    public function __construct(Document $doc, string $score) {
        parent::__construct($doc->id, $doc->content, $doc->meta);
        $this->score = $score;
    }
}

class DocumentSearch {
    public string $query;
    public int $topK;

    public function __construct(string $query, int $topK) {
        $this->query = $query;
        $this->topK = $topK;
    }
}

class IndexConfig {
    public int $dimensions;       // e.g., 1536
    public string $distanceMetric; // COSINE, L2, or IP

    public function __construct(int $dimensions, string $distanceMetric = "COSINE") {
        $this->dimensions = $dimensions;
        $this->distanceMetric = $distanceMetric;
    }
}
```

### VectorDB Interface

```php
<?php
namespace YourApp\VectorDB;

use YourApp\Embedding\EmbeddingClient;

interface VectorDBClient {
    public function createIndex(IndexConfig $config): void;
    public function storeDocument(Document $doc): void;
    public function storeDocumentsBatch(array $docs): void; // array of Document
    public function updateDocument(Document $doc): void;
    public function deleteDocument(string $id): void;
    public function searchDocuments(DocumentSearch $search): array; // array of DocumentWithScore
}
```

### Redis Implementation (Key Methods)

```php
<?php
namespace YourApp\VectorDB;

use Redis;
use YourApp\Embedding\EmbeddingClient;

class RedisVectorDB implements VectorDBClient {
    private $index;
    private $embedClient;
    private $redis;
    private $indexConfig;

    public function __construct(string $index, EmbeddingClient $embedClient, Redis $redis) {
        $this->index = $index;
        $this->embedClient = $embedClient;
        $this->redis = $redis;
        $this->indexConfig = null;
    }

    public function createIndex(IndexConfig $config): void {
        // Validate dimensions
        if ($config->dimensions <= 0) {
            throw new \Exception("Dimensions must be positive");
        }

        // Validate distance metric
        $validMetrics = ['L2', 'COSINE', 'IP'];
        if (!in_array($config->distanceMetric, $validMetrics)) {
            throw new \Exception("Invalid distance metric");
        }

        // Create Redis search index (using Predis or PhpRedis)
        // Uses Redis FT.CREATE command
        $this->redis->call('FT.CREATE', $this->index,
            'ON', 'HASH',
            'PREFIX', '1', "{$this->index}:",
            'SCHEMA',
            'content', 'TEXT',
            'embedding', 'VECTOR', 'HNSW', '6',
            'DIM', $config->dimensions,
            'DISTANCE_METRIC', $config->distanceMetric,
            'TYPE', 'FLOAT32'
        );

        $this->indexConfig = $config;
    }

    public function storeDocument(Document $doc): void {
        if ($this->indexConfig === null) {
            throw new \Exception("Index not created: call createIndex first");
        }

        // Step 1: Embed document content
        $embeddings = $this->embedClient->embedTexts([$doc->content]);
        $vector = $embeddings[0];

        // Step 2: Validate dimensions
        if (count($vector) !== $this->indexConfig->dimensions) {
            throw new \Exception("Embedding dimension mismatch");
        }

        // Step 3: Convert float64 to float32 and encode as binary
        $vector32 = array_map(fn($v) => (float)$v, $vector);
        $binaryVector = $this->encodeFloat32Vector($vector32);

        // Step 4: Store in Redis Hash
        $key = "{$this->index}:{$doc->id}";
        $this->redis->hSet($key, [
            'id' => $doc->id,
            'content' => $doc->content,
            'metadata' => json_encode($doc->meta),
            'embedding' => $binaryVector,
        ]);
    }

    public function storeDocumentsBatch(array $docs): void {
        if (empty($docs)) {
            return;
        }

        if ($this->indexConfig === null) {
            throw new \Exception("Index not created: call createIndex first");
        }

        // Step 1: Extract all contents and embed in batch
        $contents = array_map(fn($doc) => "#{$doc->id}\n{$doc->content}", $docs);
        $embeddings = $this->embedClient->embedTexts($contents);

        // Step 2: Start Redis pipeline
        $this->redis->multi();

        foreach ($docs as $i => $doc) {
            $vector = $embeddings[$i];

            if (count($vector) !== $this->indexConfig->dimensions) {
                throw new \Exception("Embedding dimension mismatch for {$doc->id}");
            }

            $vector32 = array_map(fn($v) => (float)$v, $vector);
            $binaryVector = $this->encodeFloat32Vector($vector32);

            $key = "{$this->index}:{$doc->id}";
            $this->redis->hSet($key, [
                'id' => $doc->id,
                'content' => $doc->content,
                'metadata' => json_encode($doc->meta),
                'embedding' => $binaryVector,
            ]);
        }

        // Step 3: Execute pipeline
        $this->redis->exec();
    }

    public function updateDocument(Document $doc): void {
        $this->storeDocument($doc); // Overwrites existing
    }

    public function deleteDocument(string $id): void {
        $key = "{$this->index}:{$id}";
        $this->redis->del($key);
    }

    public function searchDocuments(DocumentSearch $search): array {
        if ($this->indexConfig === null) {
            throw new \Exception("Index not created: call createIndex first");
        }

        if ($search->topK <= 0) {
            throw new \Exception("TopK must be positive");
        }

        if (empty($search->query)) {
            throw new \Exception("Query cannot be empty");
        }

        // Step 1: Embed query
        $embeddings = $this->embedClient->embedTexts([$search->query]);
        $queryVector = $embeddings[0];

        // Step 2: Validate dimensions
        if (count($queryVector) !== $this->indexConfig->dimensions) {
            throw new \Exception("Query vector dimension mismatch");
        }

        // Step 3: Convert to float32 and encode
        $queryVec32 = array_map(fn($v) => (float)$v, $queryVector);
        $binaryVector = $this->encodeFloat32Vector($queryVec32);

        // Step 4: Build KNN query
        $query = "*=>[KNN {$search->topK} @embedding \$vec AS score]";

        // Step 5: Execute search (Redis FT.SEARCH)
        $results = $this->redis->call('FT.SEARCH', $this->index, $query,
            'PARAMS', '2', 'vec', $binaryVector,
            'RETURN', '4', 'id', 'content', 'metadata', 'score',
            'DIALECT', '2'
        );

        // Step 6: Parse results
        $documents = [];
        for ($i = 2; $i < count($results); $i += 2) { // Skip count and skip results[0]
            $fields = array_combine($results[$i], $results[$i + 1]);

            $metadata = [];
            if (!empty($fields['metadata'])) {
                $metadata = json_decode($fields['metadata'], true) ?: [];
            }

            $doc = new Document(
                $fields['id'] ?? '',
                $fields['content'] ?? '',
                $metadata
            );

            $documents[] = new DocumentWithScore($doc, $fields['score'] ?? '0');
        }

        return $documents;
    }

    /**
     * Convert array of float32 to binary representation
     */
    private function encodeFloat32Vector(array $floats): string {
        $binary = '';
        foreach ($floats as $f) {
            $bits = unpack('N', pack('f', $f))[1];
            $binary .= pack('I', $bits); // Using native endian
        }
        return $binary;
    }
}
```

---

## 3. Usage Example

```php
<?php
use YourApp\Embedding\OpenAIEmbeddings;
use YourApp\VectorDB\RedisVectorDB;
use YourApp\VectorDB\Document;
use YourApp\VectorDB\DocumentSearch;
use YourApp\VectorDB\IndexConfig;
use OpenAI;
use Redis;

// 1. Create clients
$openaiClient = OpenAI::client('sk-...');
$embeddingClient = new OpenAIEmbeddings($openaiClient, 'text-embedding-3-small');

$redis = new Redis();
$redis->connect('localhost', 6379);

// 2. Create vector DB
$vectorDB = new RedisVectorDB('my_index', $embeddingClient, $redis);

// 3. Create index
$vectorDB->createIndex(new IndexConfig(1536, 'COSINE'));

// 4. Store documents
$vectorDB->storeDocumentsBatch([
    new Document('php', 'php is a backend language', ['category' => 'backend']),
    new Document('go', 'go is a backend language', ['category' => 'backend']),
    new Document('javascript', 'javascript is a frontend language', ['category' => 'frontend']),
]);

// 5. Search
$results = $vectorDB->searchDocuments(new DocumentSearch('backend programming', 2));

foreach ($results as $doc) {
    echo "{$doc->id}: {$doc->content} (score: {$doc->score})\n";
}
```

---

## 4. Redis Library Options for PHP

### Option 1: Predis (Pure PHP)
```php
$redis = new Predis\Client('redis://localhost:6379');
$redis->call('FT.CREATE', ...);
```

### Option 2: phpredis (PHP Extension)
```php
$redis = new Redis();
$redis->connect('localhost', 6379);
$redis->call('FT.CREATE', ...);
```

### Option 3: php-redis-client
- Full Redis Stack support
- Better for Redis Search (Vector Search)

**Recommendation**: Use **php-redis-client** or **phpredis** for better vector search support.

---

## 5. Important Notes

1. **Vector Encoding**: Must convert float64 → float32 and encode as binary
   - PHP: Use `pack('f', value)` and `unpack('N', ...)` carefully
   - Byte order matters (native endian)

2. **Redis Requirements**: Redis 7.0+ with Search module
   - `FT.CREATE`, `FT.SEARCH` commands needed
   - HNSW algorithm support required

3. **Batch Efficiency**: Always use batch methods when possible
   - Single API call for embeddings
   - Redis pipeline for storage

4. **Error Handling**:
   - Check if index is created before operations
   - Validate vector dimensions
   - Wrap API errors appropriately

5. **Metadata Storage**: Store as JSON string in Redis
   - Easy to serialize/deserialize
   - Searchable if needed later
