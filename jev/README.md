# Jev decisions

`github.com/mhrlife/goai-kit/jev` is a standalone client for Jev's typed decision API.
It uses only the Go standard library and does not require a chat agent.

Jev evaluates a state against named questions and returns structured answers:

| Question | Answer | Meaning |
| --- | --- | --- |
| `NoulQuestion` | `NoulAnswer` | Probability of yes, between 0 and 1 |
| `ChoiceQuestion` | `ChoiceAnswer` | Selected option, full probability distribution, and confidence |
| `ScoreQuestion` | `ScoreAnswer` | Weighted rubric score, legend, probability distribution, and confidence |

## Configuration

A client is built from a `ClientConfig`: the endpoint URL, the API key, and the
default model. Each provider has a constructor that fills the URL for you, so you
never pair the wrong URL with the wrong model slug:

```go
// OpenRouter
config := jev.NewOpenRouterClientConfig(os.Getenv("OPENROUTER_API_KEY"), jev.OpenRouterModel)

// TypeSafe
config := jev.NewTypeSafeClientConfig(os.Getenv("TYPESAFE_API_KEY"), jev.TypeSafeModel)
```

Pass the config to `jev.New`:

```go
client := jev.New(config)
```

`BaseURL` is the **full endpoint URL**, including its path:
OpenRouter is `https://openrouter.ai/api/alpha/decisions` with model
`~typesafe/jev-latest`, TypeSafe is `https://api.typesafe.ai/v1/systemone` with
model `jev-latest`. See the [TypeSafe API reference](https://docs.typesafe.ai/api).
To reach another deployment, build the `jev.ClientConfig` yourself.

## Usage

```go
client := jev.New(jev.NewOpenRouterClientConfig(os.Getenv("OPENROUTER_API_KEY"), jev.OpenRouterModel))
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
answer, err := response.Answer[jev.NoulAnswer]("urgent")
if err != nil {
    return err
}
fmt.Println(answer.Noul)
```

Imports: `context`, `fmt`, `os`, `time`, and `github.com/mhrlife/goai-kit/jev`.
For a complete program demonstrating all three primitives, see
[examples/jev](../examples/jev/main.go). Run it with `OPENROUTER_API_KEY` set:

```sh
go run ./examples/jev
```

## Requests and behavior

- State and instructions accept strings, objects, or arrays that `encoding/json` can encode.
- Question types include their `type` discriminator automatically. Question IDs map to matching answer IDs.
- `Options` maps choice names to descriptions. An empty description encodes as JSON `null`.
- Score criteria are ordered level descriptions; provide at least two levels.
- The config is embedded in the client, so `client.BaseURL` and `client.Model` can still be adjusted before the first call.
- Every answer has a `String()`, so `fmt.Println(answer)` gives one readable line:

  ```
  noul 0.92
  choice "billing" (confidence 0.80): billing 0.75, technical 0.20, other 0.05
  score 1.40 of 2 (confidence 0.60): Calm 0.10, Frustrated 0.40, Very angry 0.50
  ```

  A choice ranks the options by probability; a score keeps rubric order and names the
  levels from its legend. A noul prints the probability of yes without rounding it to
  a verdict, since where the line sits between yes and no is the caller's decision.
- `fmt.Println(response.Answers)` prints the whole set, one answer per line, sorted by
  question id and aligned:

  ```
  frustration: score 1.40 of 2 (confidence 0.60): Calm 0.10, Frustrated 0.40, Very angry 0.50
  team:        choice "billing" (confidence 0.80): billing 0.75, technical 0.20, other 0.05
  urgent:      noul 0.92
  ```
- `response.Answer[A]("id")` reads one answer as its concrete type. It errors on an unknown id or a
  type that does not match the question asked, so it never panics. The raw `response.Answers` map is
  still there when you want to range over every answer with a type switch.
- Use `client.Do(ctx, &jev.Request{...})` for an explicit request. A nonempty request model overrides the client default. `Do` does not modify the request.
- Set `client.HTTP` for custom transports or timeouts. A nil `HTTP` uses `http.DefaultClient`.
- There is no default timeout: use a context deadline or an HTTP client timeout.
- Configure the client before concurrent use, and do not mutate shared request maps or state while calls are running.
- Responses always include token usage. `Usage.Cost` (dollars), `ID` and `Provider` are
  filled by OpenRouter only; TypeSafe leaves them at their zero value.
- Neither provider reports a latency, so `Response.Latency` is measured by this package
  around the HTTP round trip. It includes network time and is not part of the JSON.
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
