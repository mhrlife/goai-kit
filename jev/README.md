# Jev decisions

`github.com/mhrlife/goai-kit/jev` is a standalone client for Jev's typed decision API.
It uses only the Go standard library and does not require a chat agent.

Jev evaluates a state against named questions and returns structured answers:

| Question | Answer | Meaning |
| --- | --- | --- |
| `NoulQuestion` | `NoulAnswer` | Probability of yes, between 0 and 1 |
| `ChoiceQuestion` | `ChoiceAnswer` | Selected option, full probability distribution, and confidence |
| `ScoreQuestion` | `ScoreAnswer` | Weighted rubric score, legend, probability distribution, and confidence |

## OpenRouter

```go
client := jev.New(os.Getenv("OPENROUTER_API_KEY"))
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

response, err := client.Decide(ctx, "My payment keeps failing.", jev.Questions{
    "urgent": jev.NoulQuestion{
        Instructions: "Does this need urgent attention?",
    },
})
if err != nil {
    return err
}
answer, ok := response.Answers["urgent"].(jev.NoulAnswer)
if !ok {
    return fmt.Errorf("missing or unexpected urgent answer")
}
fmt.Println(answer.Noul)
```

Imports: `context`, `fmt`, `os`, `time`, and `github.com/mhrlife/goai-kit/jev`.
For a complete program demonstrating all three primitives, see
[examples/jev](../examples/jev/main.go). Run it with `OPENROUTER_API_KEY` set:

```sh
go run ./examples/jev
```

## TypeSafe

Configure the endpoint and model together before making calls:

```go
client := jev.New(os.Getenv("TYPESAFE_API_KEY"))
client.BaseURL = jev.TypeSafeURL
client.Model = jev.TypeSafeModel
```

`BaseURL` is the **full endpoint URL**, including its path. Defaults are
`https://openrouter.ai/api/alpha/decisions` and `~typesafe/jev-latest`;
TypeSafe uses `https://api.typesafe.ai/v1/systemone` and `jev-latest`.
See the [TypeSafe API reference](https://docs.typesafe.ai/api).

## Requests and configuration

- State and instructions accept strings, objects, or arrays that `encoding/json` can encode.
- Question types include their `type` discriminator automatically. Question IDs map to matching answer IDs.
- `Options` maps choice names to descriptions. An empty description encodes as JSON `null`.
- Score criteria are ordered level descriptions; provide at least two levels.
- Use `client.Do(ctx, &jev.Request{...})` for an explicit request. A nonempty request model overrides the client default. `Do` does not modify the request.
- Set `client.HTTP` for custom transports or timeouts. A nil `HTTP` uses `http.DefaultClient`.
- There is no default timeout: use a context deadline or an HTTP client timeout.
- Configure the client before concurrent use, and do not mutate shared request maps or state while calls are running.
- Responses include token usage and, when returned by OpenRouter, cost and latency.
- Unknown question or answer types produce an error containing the affected question ID.

## Errors

Non-200 responses return `*jev.APIError` with the HTTP status and response body.
Use `errors.As` to inspect it:

```go
var apiErr *jev.APIError
if errors.As(err, &apiErr) && apiErr.Retryable() {
    // Apply your application's bounded backoff policy before retrying.
}
```

`Retryable` identifies HTTP 429 and 500–599, including 529. The client does not
retry automatically. Transport errors retain their cause, so cancellation can be
checked with `errors.Is(err, context.Canceled)`.
