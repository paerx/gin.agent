# Quickstart

1. 启动 Redis。
2. 配置环境变量：
   - `SERVER_ADDR=:8080`
   - `REDIS_ADDR=localhost:6379`
   - `GINAI_INTERNAL_TOKEN=dev-internal-token`
   - `GINAI_MEMORY_STORE=memory` 可跳过 Redis，直接用进程内存跑 demo
   - `GINAI_OWNER_USER_ID=你的_lark_user_id` 可让你在群里管理用户权限
3. 运行：

```bash
go run ./examples/lark-bot
```

本地无 OpenAI Key 时会自动使用规则 planner，便于直接演示：

- `查一下 0xabc 用户信息`
- `把他的昵称改成 Paer`
- `确认`
- `myuserid`
- `clean`
- `addme operator readonly`
- `approve <request_id>`

也可以直接执行本地 Lark webhook 模拟脚本：

```bash
GINAI_MEMORY_STORE=memory go run ./examples/lark-bot
bash scripts/local_lark_flow.sh
```

示例应用启动时会预注册 demo tools，所以不需要先手动访问业务 API。
