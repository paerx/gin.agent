# Lark 接入

需要配置：

- `LARK_APP_ID`
- `LARK_APP_SECRET`
- `LARK_VERIFICATION_TOKEN`
- `LARK_ENCRYPT_KEY`

HTTP 回调地址默认示例为 `/lark/events`。如果在飞书控制台选择“使用长连接接收事件”，配置：

```bash
LARK_EVENT_MODE=ws
LARK_APP_ID=cli_xxx
LARK_APP_SECRET=xxx
LARK_VERIFICATION_TOKEN=xxx
LARK_ENCRYPT_KEY=xxx
LARK_DOMAIN=https://open.larksuite.com
```

长连接模式使用飞书官方 Go SDK，不需要公网回调地址。  
本项目当前只处理文本消息，并按 `message_id` 做去重。
