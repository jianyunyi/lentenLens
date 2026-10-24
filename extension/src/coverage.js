'use strict';

const reasonNames = {
  snapshot_budget: 'Go 源码总预算已耗尽',
  file_too_large: '文件超过单文件大小上限',
  binary_or_invalid_utf8: '不是可索引的 UTF-8 文本',
  parse_error: 'Go 语法无法完整解析',
  unsafe_path: '路径包含链接或不安全路径',
  read_error: '文件无法读取',
};
function count(value) { return Number.isFinite(value) && value > 0 ? Math.floor(value) : 0; }
function limitedCoverage(coverage) {
  return !!coverage && (coverage.complete === false || count(coverage.omitted_count) > 0 || count(coverage.excluded_count) > 0);
}
function coverageSummary(coverage) {
  if (!coverage) return '索引覆盖范围未知；请使用与扩展匹配的新版 CLI。';
  if (!limitedCoverage(coverage)) return 'Go 源码索引完整（当前允许范围内）。';
  const omitted = count(coverage.omitted_count), excluded = count(coverage.excluded_count);
  const details = [];
  if (omitted) details.push(`${omitted} 个 Go 文件未纳入`);
  if (excluded) details.push(`${excluded} 项文件或目录按默认规则 / .intentignore 排除`);
  return `索引范围不完整${details.length ? '：' + details.join('；') : '：存在未纳入的源码'}。不代表整个工作区均已分析。`;
}
function coverageLines(coverage) {
  if (!coverage) return [coverageSummary(coverage)];
  const lines = [coverageSummary(coverage)];
  if (Number.isFinite(coverage.budget_bytes) && coverage.budget_bytes > 0) {
    lines.push(`仅 Go 源码计入索引预算：${(count(coverage.captured_bytes) / 1048576).toFixed(2)} / ${(coverage.budget_bytes / 1048576).toFixed(2)} MiB；其他文件不占用此预算。`);
  }
  if (count(coverage.excluded_count)) lines.push('被排除的目录按一项计数，不递归统计其中的文件；为保护隐私，不列出被排除路径。嵌套 .worktrees 与常见缓存不参与索引。');
  for (const item of (Array.isArray(coverage.omitted) ? coverage.omitted : []).slice(0, 20)) {
    if (item && typeof item.file === 'string') lines.push(`${item.file}：${reasonNames[item.reason] || '未纳入索引'}`);
  }
  if (coverage.truncated) lines.push('遗漏示例已截断，仅显示部分路径；以上遗漏总数仍为完整计数。');
  if (limitedCoverage(coverage)) lines.push('请检查默认排除规则、.intentignore、文件大小及 Go 源码预算；未显示入口不等于文件里没有 Go 函数。');
  return lines;
}
module.exports = { limitedCoverage, coverageSummary, coverageLines };
