# Changelog

## 0.0.4

- FIM: a stream that ends before `[DONE]` is an error, not a silent success.
- Chat: `Create` rejects `stream: true`; tool messages are validated against the
  preceding assistant `tool_calls`; empty tool output is accepted;
  `Accumulate` returns `object: "chat.completion"`.
- Streaming: every SSE endpoint rejects a non-`text/event-stream` 2xx response
  with the server message and request id; Responses requires a terminal event.
- History: empty tool results validate in Chat, empty files and empty text
  blocks are dropped with warnings, invalid tool arguments become `{}`, inline
  image media types are detected instead of guessed, and `ToResponses` keeps
  item order.
- Messages: empty string content and empty text blocks are rejected.
- Responses: `top_p=0` is accepted; `Create` rejects `stream: true`.
- Limits: oversize errors report exact byte counts.
- Clients: nil receivers return an error instead of panicking; a nil
  `context.Context` returns `*deepseek.TransportError` instead of panicking.
- Errors: `responses.ResponseError` is an alias of `deepseek.ResponseError`.

## 0.0.3

- Report `Version = 0.0.3` so the released tag passes the CI tag check. No
  functional changes.

## 0.0.2

- Surface in-band generation failures and truncated streams.
- Fix history conversion, request validation, and upload limits.
- Harden SSE parsing, error bodies, and credential scoping.
- Add CI with live-build, race, formatting, and tag-version checks.

## 0.0.1

- Initial release of the typed DeepSeek API client: Chat Completions, Responses,
  Messages, FIM, Files, models, balance, and search/fetch/OCR/TTS/ASR tools.
- Streaming, thinking mode, cross-protocol history conversion, request
  validation, retries with `Retry-After`, and in-band generation error handling.
