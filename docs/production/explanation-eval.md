# 模型解释质量评估

`cmd/intentlens-eval` 用真实模型跑一组固定样例，走与 `explain` 命令相同的路径（`App.Explain`：inspect、证据收集、复核）。它会输出机器初评，以及一张供人工评分的表。

## 样例

`eval/explain/cases/<name>/` 下每个目录是一个独立的 Go 模块，含 `case.json`：

- `file`、`function`：要解释的函数；
- `expect`：解释应当讲到的要点，每项是一组关键词，任一关键词出现在 `supported` 结论里就算命中；
- `forbid`：代码里没有、模型容易编造的说法（并发安全、TTL、LRU、日志等），出现在 `supported` 结论里就算违规。

关键词匹配只是粗筛，会有误判。质量结论以人工评分为准。

## 运行

```bash
export INTENTLENS_API_KEY=...            # 不要写进仓库
# OpenRouter 免费模型（模型 ID 以 :free 结尾，可用列表以 openrouter.ai/models 为准）
go run ./cmd/intentlens-eval -model <provider/model:free>
# DeepSeek
go run ./cmd/intentlens-eval -base-url https://api.deepseek.com -model deepseek-chat
```

其他参数：`-mode light|fast|strict`（默认 light）、`-only retry,cache`、`-out eval-out`、`-token-limit`（每个样例的预算）。每个样例会把源码复制到临时目录，用独立的状态目录运行，不会读写当前项目的 IntentLens 状态。

## 输出

- `summary.md`：每个样例的结论数、复核状态分布、无效引用数、要点命中数、违规项、token 用量和耗时；
- `report.json`：完整结论、证据和评分；
- `review.csv`：每条结论一行，评审人填写：
  - `correct (0-2)`：0 错误，1 部分正确或含糊，2 正确；
  - `grounded (y/n)`：引用的源码行是否足以支持这条结论；
  - `useful (0-2)`：对不熟悉这段代码的开发者是否有帮助；
  - `notes`。

## 验收口径（建议）

至少 2 名评审人对同一次运行独立评分；分歧项讨论后定稿。建议的通过线：`supported` 结论中正确率 ≥ 90%、grounded ≥ 95%，所有样例没有 `forbid` 违规，无效引用为 0。免费模型的结果只代表该模型，不代表用户自选的模型。
