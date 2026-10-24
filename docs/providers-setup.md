# 配置不同厂商的大模型

适用于当前扩展源码。模型用于解释、证据检查、共享生成、出题与理由反馈；轮次捕获可由 IDE 手动入口、Skill / 通用 CLI 或可选 Claude Code hooks 接入，模型服务与编码工具可以不同。

## 在 VS Code 中设置

1. 如从当前源码打包，按[本地打包说明](release.md)生成并校验 VSIX，再从 VS Code 手动安装；打开自己的本地 Go 项目。
2. `Ctrl+Shift+P` → **IntentLens: 启用或调整模型分析**。
3. 选择厂商，核对 API 基础地址，填写账户可用的模型 ID。
4. 填写**密钥环境变量的名称**，例如 `DEEPSEEK_API_KEY`；不要在这里输入真正的密钥。
5. 设置项目累计 token 上限，检查最终提示中的接收地址、模型、发送范围与已有用量，然后选择“同意并启用”。取消不保存设置。
6. 把实际 API 密钥设置到同名本机环境变量。Windows 可以在开始菜单搜索“编辑账户的环境变量”，在用户变量里新建对应变量名和密钥值。完全退出并重新启动 VS Code，使新进程继承变量。Claude Code hook 进程也需要继承该变量。
7. 点击函数上方入口，再点“生成解释”。保存配置不会发起测试调用，只有主动生成或已启用的 Stop hook 分析会发送请求。

不要把密钥放到 Go 文件、`.env` 提交记录、截图或聊天里。本产品不读取其他 AI 工具的登录凭据；厂商聊天订阅不等于这份 API 密钥可用。环境变量由你配置，IntentLens 不自动安装服务或购买 API 额度。

## 厂商预设与自定义地址

| 选项 | Base URL 默认值 | 环境变量名 | 协议 |
|---|---|---|---|
| OpenAI | `https://api.openai.com/v1` | `OPENAI_API_KEY` | Chat Completions，max_completion_tokens |
| Anthropic / Claude | `https://api.anthropic.com/v1` | `ANTHROPIC_API_KEY` | Messages |
| Google Gemini | `https://generativelanguage.googleapis.com/v1beta` | `GEMINI_API_KEY` | generateContent |
| DeepSeek | `https://api.deepseek.com/v1` | `DEEPSEEK_API_KEY` | OpenAI 兼容，max_tokens |
| 通义千问 / 百炼 | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `DASHSCOPE_API_KEY` | OpenAI 兼容，max_tokens |
| 其他兼容服务 | 按服务商文档填写 | 默认 `INTENTLENS_API_KEY`，可改 | OpenAI 兼容，max_tokens |

地址与协议依据：[OpenAI Chat Completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)、[Anthropic Messages](https://platform.claude.com/docs/en/api/messages)、[Gemini generateContent](https://ai.google.dev/api/generate-content)、[DeepSeek](https://api-docs.deepseek.com/api/create-chat-completion/)、[百炼地域地址](https://help.aliyun.com/en/model-studio/base-url)。百炼预设是北京地域，国际地域/其他部署按账户对应文档修改地址。

填写的是基础地址，不带 `/chat/completions`、`/messages` 或 `:generateContent`，程序会按协议拼接。Gemini 模型名不含 `models/` 前缀。模型名称不预填，避免使用你的账户无权调用或已更新的模型。

其他厂家或代理服务，只要支持兼容的 Chat Completions 文本协议、`system/user` 消息、`max_tokens`、非流式答案及 `usage.total_tokens`，可选择自定义兼容服务。协议适配不等于对所有厂商和模型实测通过。Responses-only 模型、Azure 部署专属路径、Vertex OAuth、Bedrock 签名及厂商私有鉴权暂未支持。

远程地址必须 HTTPS。允许明确配置本机 loopback HTTP，例如 `http://127.0.0.1:1234/v1`；若本机服务不校验密钥，仍需给指定环境变量设置一个非空占位值。本机服务是否会再转发请求由该服务决定。本产品不自动跟随重定向或改换厂商。

## 用量、隐私与排错

切换厂商不会重置项目已用 token 和未决占用。不同模型价格不同，token 上限不是货币上限，没有配置费率时不显示虚构费用。若远端未返回有效用量、响应截断、超时或无法解析，会保留保守未决占用；不会假定调用免费，也不会自动重试。

项目设置（含地址、模型、变量名）、用量和个人数据只在仓库外保存；实际密钥不写设置或共享记录。获准源码和你的问题会送到确认的服务；练习反馈还包含答案和理由。暂停阻止新请求，已发出的请求可能继续完成。

- 提示缺少环境变量：确认变量名完全一致，并重启 VS Code/终端。执行“查看状态与预算”可看到当前模型配置，但不会显示密钥值。
- HTTP 401/403：检查厂商 API 权限、密钥和地址是否匹配。错误不展示远端响应正文。
- HTTP 404：核对模型 ID、地域和 Base URL，不要重复拼接接口路径。
- JSON 或用量错误：该模型/兼容服务可能不满足输出契约；记录协议、模型和脱敏错误，不上传密钥或完整请求内容。
- 预算不足：主动调整累计上限，或缩小上下文；更换厂商不会绕过预算。

离线预检可明确选择环境变量，例如 `node scripts/preflight.cjs --key-env DEEPSEEK_API_KEY`。不带参数默认检查旧 Anthropic 变量；仅检查变量是否存在，不验证鉴权，不发请求。

协议适配及错误边界使用本地 HTTP fixture 验证；真实账户、模型 ID 与解释质量仍待联调。切换或回退到不支持当前模型字段的旧版前，先暂停分析并按该版本的配置格式检查，不能假定旧客户端兼容新配置。
