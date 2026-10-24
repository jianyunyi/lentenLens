# 模型解释复核模式

IntentLens 的模型分析仍须在项目设置中显式启用并确认接收地址。复核模式不会启用模型、改变密钥来源或绕过现有 token 预算。

## 轻量复核（默认）

`light` 保留两阶段流程：第一阶段根据证据生成简短解释；第二阶段独立检查每项说法。它采用较短提示，并只把已引用的证据送入复核阶段。每阶段的输出上限分别为 1,024 和 384 tokens。旧配置未设置模式时也会按 `light` 处理。

对于使用 OpenAI-compatible 协议且 Base URL 主机名恰为 `api.deepseek.com` 的配置，IntentLens 在 `light` 请求中添加 DeepSeek [官方指南](https://api-docs.deepseek.com/guides/thinking_mode)支持的 `thinking: {"type":"disabled"}`。不会向 OpenAI、Anthropic、Gemini 或其他兼容服务发送该字段。

## 快速模式

`fast` 适用于较小、低风险的函数解释，只发出一个模型请求，不做独立复核。它仅在问题不超过 2,000 字节、证据记录不超过 8 条、证据源码合计不超过 8 KiB 时生效；条件按字节数及条数衡量，不代表系统能识别业务风险。

快速结果中的说法一律保持“待确认”，并明确标注未做独立复核。证据 ID 的文件和行范围来自本地证据记录，不能由模型编造；VS Code 面板会在每项说法旁显示这些位置。若 fast 请求超过任一小范围阈值，系统会自动改用 `strict` 的双阶段复核，并在结果中说明升级原因。高风险场景不要使用 fast，应主动选择 strict。

DeepSeek 官方端点在 `fast` 模式下也会关闭 thinking；其他协议和兼容服务不会收到 DeepSeek 专属字段。

## 严格复核

`strict` 使用完整解释提示和更大的输出上限（第一阶段 2,048 tokens，复核阶段 1,024 tokens），并在第二阶段逐条要求检查说法。它仍只依据所提供的已引用证据，不能保证模型判断正确，也不是正确性证明。DeepSeek 的 strict 请求不附加关闭 thinking 的字段。

## 选择与查看

在 VS Code 执行 **IntentLens: 启用或调整模型分析**，选择模式；最终确认框会说明该模式的请求数量和复核边界。模式保存于项目本地模型配置中，`status` 可查看。通用 CLI 也可在 `configure` 命令的 `settings.model.verification_mode` 中设置 `light`、`fast` 或 `strict`；不传时使用 `light`。

这些模式适用于函数解释、轮次解释以及使用同一 claim 验证流程的修订/共享生成。练习生成与反馈使用其独立结构化请求，不由此模式改变。项目累计 token 上限不是费用上限；实际消耗仍取决于服务和输入。
