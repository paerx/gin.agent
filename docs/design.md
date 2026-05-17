# Gin AI Lark Bot Design

核心链路：

`Lark -> Adapter -> Agent -> Permission -> Confirm -> Invoker -> Formatter -> Audit`

库侧采用显式注册模型，只有通过 `ginai.Bind(...)` 注册过的 Gin API 才会被暴露给 planner。  
v0.1 先保证安全闭环和可运行 demo，不做多 tool 编排和复杂平台能力。
