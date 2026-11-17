# deepseek-sdk-go

[English](README.md) | 简体中文

DeepSeek API 的类型化 Go SDK，仅依赖标准库：支持 Chat Completions、Responses、
Messages、FIM、Files、模型列表、余额查询，以及 search/fetch/OCR/TTS/ASR 工具；
包含流式输出、思考模式和跨协议历史转换。

## 安装

```sh
go get github.com/chimixiong/deepseek-sdk-go
```

```go
import (
	deepseek "github.com/chimixiong/deepseek-sdk-go"
	"github.com/chimixiong/deepseek-sdk-go/chat"
)
```

设置 `DEEPSEEK_API_KEY` 后，`deepseek.New(deepseek.Config{})` 会自动读取。
需要 Go 1.27 或更高版本（使用 `iter.Seq2` 与 `new(expr)`）。

```go
core := deepseek.New(deepseek.Config{
	// APIKey:  可选，默认读取 DEEPSEEK_API_KEY
	// BaseURL: 可选，默认 https://api.deepseek.com
})
```

传输层对 429/5xx 和网络错误做指数退避重试，遵守 `Retry-After`，同时解析
OpenAI 与 Anthropic 两种错误体为 `*deepseek.APIError`，并从 `x-request-id`
或 `x-ds-trace-id` 提取请求 ID。

## 端点

| 端点 | 包 | 文档 |
|---|---|---|
| `POST /chat/completions`、`/beta/chat/completions` | `chat` | [Chat Completions](https://api-docs.deepseek.com/zh-cn/api/create-chat-completion) |
| `POST /responses` | `responses` | [Responses](https://api-docs.deepseek.com/zh-cn/api/create-response) |
| `POST /anthropic/v1/messages`（`/anthropic/messages`） | `messages` | [Messages](https://api-docs.deepseek.com/zh-cn/api/create-message) |
| `POST /beta/completions` | `fim` | [FIM](https://api-docs.deepseek.com/zh-cn/api/create-completion) |
| `GET /models` | `models` | [模型列表](https://api-docs.deepseek.com/zh-cn/api/list-models) |
| `GET /user/balance` | `billing` | [余额](https://api-docs.deepseek.com/zh-cn/api/get-user-balance) |
| `POST /files`、`GET /files`、`GET /files/{id}`、`DELETE /files/{id}` | `files` | [文件](https://api-docs.deepseek.com/zh-cn/api/create-file) |
| `POST /v1/tools/search`、`/fetch`、`/ocr`、`/tts`、`/asr` | `tools` | [工具](https://api-docs.deepseek.com/zh-cn/api/search) |

## Chat Completions

```go
ctx := context.Background()
client := chat.New(core)

response, err := client.Create(ctx, &chat.Request{
	Model:    "deepseek-flash",
	Messages: []chat.Message{chat.UserMessage("9.11 和 9.8 哪个大？")},
})
fmt.Println(response.AssistantMessage().Content.Text)
```

流式响应返回类型化 chunk，思维链与正文是分开的字段：

```go
for chunk, err := range client.CreateStream(ctx, request) {
	if err != nil {
		log.Fatal(err)
	}
	if len(chunk.Choices) == 0 {
		continue
	}
	delta := chunk.Choices[0].Delta
	if delta.ReasoningContent != nil {
		fmt.Print(*delta.ReasoningContent)
	}
	if delta.Content != nil {
		fmt.Print(*delta.Content)
	}
}
```

`chat.Accumulate` 可以把整个流聚合成一个 `*chat.Response`，工具参数的增量会按
`index` 拼接。

思考模式：

- `Thinking` 与 `ReasoningEffort` 分别映射到 `thinking` 和 `reasoning_effort`。
- 请求携带 `tools` 时，历史中每条 assistant 消息都必须带 `reasoning_content`。
  用 `chat.AssistantToolCalls(reasoning, calls...)` 构造历史，或直接保留响应里的
  assistant 消息。
- 思考模式下 `temperature`、`presence_penalty`、`frequency_penalty` 不生效；
  `tool_choice: "required"` 或指定函数会返回 HTTP 400。显式开启思考时，
  `Request.Validate` 会在发请求前拦截后两种写法。
- `assistant.prefix` 必须使用 `chat.NewBeta(core)`。

内容块、`stop` 联合类型、工具定义和 `tool_choice` 都是类型化的；请求的未知字段
通过 `Request.Extra map[string]json.RawMessage` 透传，未知内容块类型保留在
`ContentPart.Raw`。

## Responses 与 Messages

`responses` 和 `messages` 与各自线上格式一一对应：输入/输出 item 联合类型、
`Reasoning`/`Thinking` 块、语义化 SSE 事件、用量明细，以及 `annotations`、
`server_tool_calls`、citations 等开放字段的原样透传。
`messages.New` 会同时发送 `Authorization: Bearer` 和 `x-api-key`。

## Files

```go
uploaded, err := files.New(core).Create(ctx, &files.CreateRequest{
	Purpose:     files.PurposeUserData,
	Filename:    "diagram.png",
	ContentType: "image/png",
	File:        pngBytes,
})
```

`purpose` 会排在 `file` 之前发送，这是 API 的要求。`CreateRequest.ExpiresAfter`
映射到成对出现的 `expires_after[anchor]` 与 `expires_after[seconds]` 字段。

## 工具

```go
found, err := tools.New(core).Search(ctx, &tools.SearchRequest{Query: "DeepSeek V4", Count: new(5)})
page, err := tools.New(core).Fetch(ctx, &tools.FetchRequest{URL: found.Results[0].URL})
audio, err := tools.New(core).Speak(ctx, &tools.TTSRequest{Text: "hello"})
if err == nil {
	defer audio.Close()
}
```

`audio.Reader` 会边生成边返回 WAV/PCM 数据。OCR 与 ASR 接收 `[]byte` 文件内容和
文件名，按文档要求的顺序编码 multipart；`Speak` 返回未读取的响应体，可以在音频
生成过程中直接拷贝或播放。

## 语义约定

- 可选标量使用指针，区分“未传”和零值。
- `Request.Validate` 在发起网络请求前检查文档中的结构约束，返回
  `*deepseek.ValidationError`。
- 请求支持 `Extra`，响应中的开放对象保留为 `json.RawMessage`，DeepSeek 新增字段
  不需要等 SDK 发版。
- 流式错误由迭代器返回；重试只发生在流或音频的第一个字节之前。

## 跨协议转换

`history` 在 Chat Completions、Responses、Messages 之间转换文本、图片、文件、
工具调用和工具结果，并报告所有信息损失：

```go
transcript, warnings := history.FromChat(messages)
instructions, items, warnings := history.ToResponses(transcript, history.Options{})
system, msgs, warnings := history.ToMessages(transcript, history.Options{DropImages: true})
```

中立模型刻意不包含思维链。DeepSeek 要求 Messages 的 signature 和 Responses 的
item id 原样回传，因此跨协议转换会丢弃推理内容并返回 `Warning`。工具调用循环应
在发起它的协议内完成。`ToResponses` 会单独返回 instructions 字符串，调用方需要
把它赋给 `responses.Request.Instructions`。

## 示例

每个端点都有可运行示例。示例优先读取环境变量，其次向上查找最近的 `.env` 文件，
因此在仓库根目录直接 `go run ./examples/<name>` 即可。

| 命令 | 端点 |
|---|---|
| `go run ./examples/chat` | `POST /chat/completions`，含思考模式与用量 |
| `go run ./examples/chat_stream` | Chat Completions 流式 |
| `go run ./examples/chat_tools` | 工具调用循环，回传 `reasoning_content` |
| `go run ./examples/chat_beta` | `POST /beta/chat/completions` 前缀续写 |
| `go run ./examples/responses` | `POST /responses` |
| `go run ./examples/responses_stream` | Responses 语义事件流 |
| `go run ./examples/messages` | `POST /anthropic/v1/messages`，含 thinking signature |
| `go run ./examples/messages_stream` | Messages 事件流 |
| `go run ./examples/fim` | `POST /beta/completions` |
| `go run ./examples/models` | `GET /models` |
| `go run ./examples/balance` | `GET /user/balance` |
| `go run ./examples/files` | 文件上传、列表、查询与删除 |
| `go run ./examples/search` | `POST /v1/tools/search` |
| `go run ./examples/fetch` | `POST /v1/tools/fetch` |
| `go run ./examples/ocr` | `POST /v1/tools/ocr` |
| `go run ./examples/tts` | `POST /v1/tools/tts`，写入 `speech.wav` |
| `go run ./examples/asr` | `POST /v1/tools/asr`，读取 `speech.wav` |
| `go run ./examples/tools_pipeline` | 搜索 → 对话 |
| `go run ./examples/ocr_tts page.pdf speech.wav` | OCR → 语音合成 |

`examples/files`、`examples/ocr`、`examples/tts`、`examples/asr` 在无参数时会
生成或复用自带 fixture。

## 测试

```sh
go test ./...                                   # 离线，httptest fixture
DEEPSEEK_API_KEY=... go test -tags live -run TestLive -v ./...
```

live 测试覆盖全部 15 个端点，包括回传 `reasoning_content` 的工具调用、结构化输出、
beta 前缀续写、文件上传/列表/查询/删除、OCR、TTS 与 ASR。测试会自行生成 PNG、
PDF 和语音 fixture，用完删除上传文件；未设置 key 时自动跳过。

## 许可证

MIT，见 [LICENSE](LICENSE)。
