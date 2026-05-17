# Security Notes

- 只有显式注册的 tool 可被调用
- 写操作默认进入确认流
- `DELETE` 默认禁用
- 权限、字段白名单、internal token 都在代码层校验
- 审计与输出都做敏感字段脱敏
