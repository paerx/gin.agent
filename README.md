# Gin AI Lark Bot

Gin AI Lark Bot makes selected Gin APIs callable from Lark through natural language. The important boundary is explicit registration: the AI can only propose calls to tools that developers expose with `ginai.Bind`, and code still owns permission checks, parameter validation, confirmation, invocation, formatting, and audit logging.

## What Works In v0.1

- Explicit Gin API registration as AI tools
- Struct tag to JSON Schema conversion
- JSON Schema argument validation before execution
- OpenAI-compatible planner, plus a local rule planner for demos and tests
- Read-only tools execute directly
- Write tools enter a confirmation flow
- Redis-backed memory store and in-process memory store
- HTTP invoker with `X-GinAI-Internal-Token`
- Lark text webhook adapter
- SQLite audit log store
- Runnable `get_user_info` / `update_user_info` example

## Project Layout

- [pkg/ginai](/Users/pangaichen/Desktop/gin.agent/pkg/ginai): tool model, registry, Gin binding, schema, validation, debug route
- [pkg/agent](/Users/pangaichen/Desktop/gin.agent/pkg/agent): planner, memory interface, confirmation flow, formatter, agent orchestration
- [pkg/adapter/lark](/Users/pangaichen/Desktop/gin.agent/pkg/adapter/lark): Lark webhook parsing, verification, sender
- [pkg/auth](/Users/pangaichen/Desktop/gin.agent/pkg/auth): role-based permission checks
- [pkg/transport](/Users/pangaichen/Desktop/gin.agent/pkg/transport): HTTP API invoker
- [pkg/storage](/Users/pangaichen/Desktop/gin.agent/pkg/storage): Redis memory, in-memory memory, SQLite audit
- [examples/lark-bot](/Users/pangaichen/Desktop/gin.agent/examples/lark-bot): runnable demo app

## Quick Start

Run the demo without Redis or OpenAI:

```bash
export GINAI_MEMORY_STORE=memory
export GINAI_INTERNAL_TOKEN=dev-internal-token
export AUDIT_DSN='file:ginai_audit.db'
go run ./examples/lark-bot
```

In another terminal, trigger the local Lark-style flow:

```bash
bash scripts/local_lark_flow.sh
```

The demo prints Lark replies to stdout when `LARK_APP_ID` and `LARK_APP_SECRET` are not configured.

Inspect registered tools:

```bash
curl -s http://localhost:8080/debug/ginai/tools
```

The example pre-registers its demo tools on startup, so the local Lark flow works before any direct API call. You can still call the internal demo API directly:

```bash
curl -s 'http://localhost:8080/api/getUserinfo?wallet=0xabc' \
  -H 'X-GinAI-Internal-Token: dev-internal-token'
```

## Configuration

Configuration is loaded from environment variables in [pkg/config/config.go](/Users/pangaichen/Desktop/gin.agent/pkg/config/config.go).

- `SERVER_ADDR`: default `:8080`
- `LLM_BASE_URL`: default `https://api.openai.com/v1`
- `LLM_MODEL`: default `gpt-4.1`
- `OPENAI_API_KEY`: enables the OpenAI-compatible planner; empty uses the rule planner
- `LARK_APP_ID`, `LARK_APP_SECRET`: enables real Lark message sending
- `LARK_VERIFICATION_TOKEN`, `LARK_ENCRYPT_KEY`: webhook verification
- `LARK_EVENT_MODE`: `http` for webhook mode, `ws` for Lark long connection mode
- `LARK_DOMAIN`: `https://open.feishu.cn` for Feishu China, `https://open.larksuite.com` for Lark global
- `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB`: Redis memory config
- `GINAI_MEMORY_STORE=memory`: use in-process memory instead of Redis
- `AUDIT_DSN`: default `file:ginai_audit.db`
- `GINAI_BASE_URL`: default `http://localhost:8080`
- `GINAI_INTERNAL_TOKEN`: default `dev-internal-token`
- `GINAI_CONTEXT_TTL`: default `168h`
- `GINAI_MAX_HISTORY`: default `30`

## Registering Tools

```go
api.GET("/getUserinfo", binder.Bind(ginai.Tool{
    Name:        "get_user_info",
    Description: "查询用户信息，支持通过钱包地址或用户ID查询",
    Params:      GetUserInfoReq{},
    ReadOnly:    true,
}), handler.GetUserInfo)

api.POST("/updateUserinfo", binder.Bind(ginai.Tool{
    Name:        "update_user_info",
    Description: "更新用户基础信息，只允许更新 nickname、avatar、bio",
    Params:      UpdateUserInfoReq{},
    ReadOnly:    false,
    NeedConfirm: true,
    Roles:       []string{"admin", "operator"},
    AllowFields: []string{"nickname", "avatar", "bio"},
}), handler.UpdateUserInfo)
```

## Safety Defaults

- Unregistered Gin APIs are invisible to the agent.
- `DELETE` tools are rejected in v0.1.
- Write tools default to confirmation.
- Tool arguments are validated against generated JSON Schema.
- Write tool fields can be constrained with `AllowFields`.
- Permission checks run before confirmation and before execution. Users who can write should also receive `readonly` if they need read tools.
- Audit logs mask sensitive argument values before storing them.

## Verification

```bash
go test ./...
```

The test suite includes unit coverage plus an end-to-end Lark webhook test that exercises query, confirmation, execution, and audit logging.

## Lark Long Connection Mode

For the Lark console option "使用长连接接收事件", start the demo with:

```bash
export LARK_EVENT_MODE=ws
export LARK_APP_ID=cli_xxx
export LARK_APP_SECRET=xxx
export LARK_VERIFICATION_TOKEN=xxx
export LARK_ENCRYPT_KEY=xxx
export LARK_DOMAIN=https://open.larksuite.com
export GINAI_INTERNAL_TOKEN=dev-internal-token
go run ./examples/lark-bot
```

This mode uses the official Lark Go SDK WebSocket client, so you do not need a public callback URL. Keep the Gin server running because the agent still invokes the local demo APIs through HTTP.
