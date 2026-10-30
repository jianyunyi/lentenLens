# 生产回归报告（2026-10-30）

## 结果摘要

Instabox 回归环境为 Debian GNU/Linux 13.7（x86_64）。测试前后对 150 个源码快照文件做了完整性校验，哈希均匹配。指定 Go 与扩展回归检查全部通过；临时源码和传输文件已清理，测试虚拟机已恢复暂停状态。

本次验证的安全边界是：没有经过验证的操作系统沙箱时，`exercise-run` 保持失败关闭；题目验证和评分不会启动题目中的 Go 测试或作答代码。没有调用真实模型或外部模型/API。

## 测试命令与结果

以下 Go 命令在仓库根目录运行，扩展命令在 `extension/` 目录运行。

```sh
go test ./internal/exercise -run '^(TestSpecRejectsExternalImportsAndUnsafeChanges|TestVersionBindsRequirementsBaselineAndTests|TestRunFailsClosedWithoutVerifiedSandbox|TestClosedSandboxGateDoesNotInvokeChildProcessCallback)$' -v -count=1 -timeout=90s
```

结果：4 个目标测试通过，退出码 0。

```sh
go test ./internal/app -run '^(TestExecutionPlanStatesRunnerIsClosed|TestExerciseRunFailsClosedAndSingleFileApplyWorks|TestPrepareExerciseApplyRejectsMultipleFilesWithoutChangingSources|TestExerciseViewHidesReferenceAndRunFailsClosed|TestApplyCrashRecoveryAndExternalConflict|TestApplyRejectsMultipleFilesWithoutAnyWrite|TestApplyFailureRollsBackAndPreservesPermissions|TestAtomicReplaceRejectsLinksAndConflict|TestStagedSourceCleanedAfterCrash|TestPrivateAbandonedTemporarySourceIsRemoved)$' -v -count=1 -timeout=90s
```

结果：10 个目标应用与恢复测试通过，退出码 0。

```sh
go vet ./internal/exercise ./internal/app
```

结果：通过，退出码 0。

```sh
npm test
```

结果：77/77 通过，失败 0，退出码 0。

```sh
npm run check
```

结果：通过，退出码 0。

## 结论边界

这些回归结果不证明已经实现或验证了 OS 沙箱；也不证明 Windows 或真实 VS Code Extension Host 验收、模型回答准确率或生产就绪。它们仅支持本报告所列的目标回归检查与当前执行关闭行为。