# deepseek-sdk-go

English | [简体中文](README.zh-CN.md)

Typed, standard-library-only Go SDK for the DeepSeek API: Chat Completions, Responses, Messages, FIM, Files, models, balance, and search/fetch/OCR/TTS/ASR tools, with streaming, thinking mode, and cross-protocol history conversion.

## Install

```sh
go get github.com/chimixiong/deepseek-sdk-go
```

```go
import (
	deepseek "github.com/chimixiong/deepseek-sdk-go"
	"github.com/chimixiong/deepseek-sdk-go/chat"
)
```

Set `DEEPSEEK_API_KEY`; `deepseek.New(deepseek.Config{})` reads it automatically.
Requires Go 1.27 or newer (`iter.Seq2` and `new(expr)`).

```go
core := deepseek.New(deepseek.Config{
	// APIKey:  optional, defaults to DEEPSEEK_API_KEY
	// BaseURL: optional, defaults to https://api.deepseek.com
})
```

The transport retries 408, 429, 500, 502, 503, 504 and network errors with
exponential backoff,
honours `Retry-After`, parses both OpenAI-shaped and Anthropic-shaped errors into
`*deepseek.APIError`, and exposes request ids from `x-request-id` /
`x-ds-trace-id`.

## Endpoints

| Endpoint | Package | Docs |
|---|---|---|
| `POST /chat/completions`, `/beta/chat/completions` | `chat` | [Chat Completions](https://api-docs.deepseek.com/zh-cn/api/create-chat-completion) |
| `POST /responses` | `responses` | [Responses](https://api-docs.deepseek.com/zh-cn/api/create-response) |
| `POST /anthropic/v1/messages` (`/anthropic/messages`) | `messages` | [Messages](https://api-docs.deepseek.com/zh-cn/api/create-message) |
| `POST /beta/completions` | `fim` | [FIM](https://api-docs.deepseek.com/zh-cn/api/create-completion) |
| `GET /models` | `models` | [Models](https://api-docs.deepseek.com/zh-cn/api/list-models) |
| `GET /user/balance` | `billing` | [Balance](https://api-docs.deepseek.com/zh-cn/api/get-user-balance) |
| `POST /files`, `GET /files`, `GET /files/{id}`, `DELETE /files/{id}` | `files` | [Files](https://api-docs.deepseek.com/zh-cn/api/create-file) |
| `POST /v1/tools/search`, `/fetch`, `/ocr`, `/tts`, `/asr` | `tools` | [Tools](https://api-docs.deepseek.com/zh-cn/api/search) |

## Chat Completions

```go
ctx := context.Background()
client := chat.New(core)

response, err := client.Create(ctx, &chat.Request{
	Model:    "deepseek-flash",
	Messages: []chat.Message{chat.UserMessage("9.11 and 9.8, which is greater?")},
})
fmt.Println(response.AssistantMessage().Content.Text)
```

Streaming yields typed chunks; thinking fragments are separate fields:

```go
for chunk, err := range client.CreateStream(ctx, request) {
	if err != nil { log.Fatal(err) }
	if len(chunk.Choices) == 0 { continue }
	delta := chunk.Choices[0].Delta
	if delta.ReasoningContent != nil { fmt.Print(*delta.ReasoningContent) }
	if delta.Content != nil { fmt.Print(*delta.Content) }
}
```

`chat.Accumulate` drains a stream into one `*chat.Response`, concatenating tool
call argument fragments by index. On error it returns the partial response
alongside the error, so usage from a failed generation stays readable.

Thinking mode:
- `Thinking` and `ReasoningEffort` map to `thinking` and `reasoning_effort`.
- When a request carries `tools`, every prior assistant message must include
  `reasoning_content`. Build history with `chat.AssistantToolCalls(reasoning, calls...)`
  or keep the response message as-is.
- `temperature`, `presence_penalty` and `frequency_penalty` have no effect in
  thinking mode; `tool_choice: "required"` or a named function returns HTTP 400.
  `Request.Validate` reports the latter before sending when thinking is
  explicitly enabled.
- `assistant.prefix` requires `chat.NewBeta(core)`.

Content blocks, the `stop` union, tool definitions and `tool_choice` are typed;
unknown request fields go through `Request.Extra map[string]json.RawMessage`.
Unknown content block types are preserved in `ContentPart.Raw`.

## Responses and Messages

`responses` and `messages` mirror their wire formats: input/output item unions
with `Reasoning`/`Thinking` blocks, semantic SSE events, usage details and
passthrough of opaque fields (`annotations`, `server_tool_calls`, citations).
`messages.New` sends both `Authorization: Bearer` and `x-api-key`.

## Files

```go
uploaded, err := files.New(core).Create(ctx, &files.CreateRequest{
	Purpose:     files.PurposeUserData,
	Filename:    "diagram.png",
	ContentType: "image/png",
	File:        pngBytes,
})
```

`purpose` is sent before `file`, as the API requires. `CreateRequest.ExpiresAfter`
maps to the paired `expires_after[anchor]` / `expires_after[seconds]` fields.

## Tools

```go
found, err := tools.New(core).Search(ctx, &tools.SearchRequest{Query: "DeepSeek V4", Count: new(5)})
page, err := tools.New(core).Fetch(ctx, &tools.FetchRequest{URL: found.Results[0].URL})
audio, err := tools.New(core).Speak(ctx, &tools.TTSRequest{Text: "hello"}) // audio.Reader streams WAV/PCM
if err == nil {
	defer audio.Close()
}
```

OCR and ASR take `[]byte` payloads plus a filename and encode multipart in the
documented order. `Speak` returns the response body unread so audio can be
copied while it is produced.

Multipart bodies are encoded in memory once so a retry can replay them. A
request therefore holds the payload roughly twice: the `[]byte` you pass plus
the encoded body. `Validate` caps uploads at 8 MiB for ASR, 100 MiB for OCR, and
64/100/32 MiB for Files images, documents and text/spreadsheets/ebooks.

## Semantics

- Optional scalars are pointers, so "unset" and zero are distinct.
- `Request.Validate` checks documented structural constraints before any network
  call and returns `*deepseek.ValidationError`.
- Requests carry `Extra` for fields the SDK does not model, and responses keep
  open objects as `json.RawMessage`, so new DeepSeek fields do not require a
  release. An `Extra` key that names a modelled field is rejected rather than
  silently overriding it.
- Streaming errors surface from the iterator; retries happen only before the
  first byte of a stream or audio response.
- A generation that fails after the status line still returns HTTP 200.
  Chat Completions and Messages bill the tokens produced and include `usage`
  in the error body. `chat.Create` and `messages.Create` detect that `error`
  object, return `*deepseek.ResponseError` and hand back the decoded response
  so its `usage` stays readable. Streaming callers see the error from the
  iterator: Chat Completions sets `Chunk.Error`, Messages sends an `error`
  event carrying the billed usage; `chat.Accumulate` also returns the partial
  response.
- `tools.OCR` detects a 200 `error` object and returns
  `*deepseek.ResponseError` with the decoded response. OCR error bodies carry
  no `usage`, and failed or timed-out OCR is not billed.
- `chat.ChunkChoice.ServerToolEvent` exposes the streaming web-tool progress
  events (`step_reasoning_done`, `call_started`, `call_completed`) verbatim; the
  complete `server_tool_calls` and `annotations` arrive on the final chunk.

## Switching protocols

`history` converts text, images, files, tool calls and tool results between the
three chat protocols and reports every loss:

```go
transcript, warnings := history.FromChat(messages)
instructions, items, warnings := history.ToResponses(transcript, history.Options{})
system, msgs, warnings := history.ToMessages(transcript, history.Options{DropImages: true})
```

Thinking is deliberately absent from the neutral model. DeepSeek requires
Messages signatures and Responses item ids to be replayed verbatim, so
cross-protocol conversion drops reasoning and emits a `Warning`. Keep a tool
loop on the protocol that started it. `ToResponses` returns the instructions
string separately; pass it as `responses.Request.Instructions`.

## Examples

Every endpoint has a runnable example. They read `DEEPSEEK_API_KEY` from the
environment and fall back to the nearest `.env` file, so
`go run ./examples/<name>` works from the repository root.

| Command | Endpoint |
|---|---|
| `go run ./examples/chat` | `POST /chat/completions` with thinking and usage |
| `go run ./examples/chat_stream` | Chat Completions streaming |
| `go run ./examples/chat_tools` | Tool loop with `reasoning_content` replay |
| `go run ./examples/chat_beta` | `POST /beta/chat/completions` prefix continuation |
| `go run ./examples/responses` | `POST /responses` |
| `go run ./examples/responses_stream` | Responses semantic event stream |
| `go run ./examples/messages` | `POST /anthropic/v1/messages` with thinking signature |
| `go run ./examples/messages_stream` | Messages event stream |
| `go run ./examples/fim` | `POST /beta/completions` |
| `go run ./examples/fim_stream` | FIM completion stream |
| `go run ./examples/models` | `GET /models` |
| `go run ./examples/balance` | `GET /user/balance` |
| `go run ./examples/files` | create, list, retrieve and delete a file |
| `go run ./examples/search` | `POST /v1/tools/search` |
| `go run ./examples/fetch` | `POST /v1/tools/fetch` |
| `go run ./examples/ocr` | `POST /v1/tools/ocr` |
| `go run ./examples/tts` | `POST /v1/tools/tts`, writes `speech.wav` |
| `go run ./examples/asr` | `POST /v1/tools/asr`, reads `speech.wav` |
| `go run ./examples/tools_pipeline` | search → chat |
| `go run ./examples/ocr_tts page.pdf speech.wav` | OCR → TTS |

`examples/files`, `examples/ocr`, `examples/tts` and `examples/asr` generate or
reuse their own fixtures when called without arguments.

## Testing

```sh
go test ./...                                   # offline, httptest fixtures
DEEPSEEK_API_KEY=... go test -tags live -run TestLive -v ./...
```

The live suite exercises all 15 endpoints, including a tool-call turn that
replays `reasoning_content`, structured output, beta prefix continuation, file
upload/list/retrieve/delete, OCR, TTS and ASR. It generates its own PNG, PDF and
speech fixtures, deletes uploaded files and is skipped without an API key.

## License

MIT. See [LICENSE](LICENSE).
