# 异步重投影任务队列

这是一个不依赖图形 SDK 的 Go 实现，用于验证异步重投影中的
**帧版本、姿态版本、任务所有权和迟到结果隔离**。

## 状态机

```text
queued ──start──> running ──work returns──> computed ──commit guard──> committed
   │                  │                         │
   │                  │                         ├──stale────────────> discarded
   │                  │                         └──error────────────> failed
   └──cancel/new frame/new pose──> cancelled or superseded
```

若任务仍在运行，取消或过期只会先撤销提交权；worker 退出后才释放其私有资源。

`computed` 只表示计算已经返回，不表示结果有权展示。只有同时满足以下条件才可
提交：

1. 任务输入帧等于当前帧版本；
2. 任务姿态版本等于当前姿态版本；
3. 输出缓冲仍由该任务独占所有。

任一版本在任务开始后推进，任务立即失去提交权并进入 `superseded`。worker
之后即使继续写自己的 staging buffer，也只能写入私有资源，不能触碰当前展示
缓冲。

## 所有权与取消

- 每个任务在提交时复制输入帧，并分配独立的输入/输出 `BufferID`。
- `WorkFunc` 只能拿到自己的 `StagingBuffer`，从不接触当前展示缓冲。
- 排队中的任务取消或过期时立即释放自己的输入/输出。
- 运行中的任务取消或过期时先撤销提交权；必须等 `WorkFunc` 返回后才释放其
  缓冲，避免释放仍在写入的内存。
- `bufferPool` 的释放和展示操作都会校验 owner；一个任务不能回收另一个任务
  的缓冲，当前展示缓冲也不能由取消操作回收。

## 可重放日志

每次状态转换都会写入 `Event`。`TaskRecord` 可直接复核：

- `ID`
- `InputFrame`
- `PoseVersion`
- `CompletedAt`：worker 返回的真实完成时刻，迟到任务也会记录
- `FinalUsedVersion`：最终成功提交的姿态版本；未提交时为 `0`
- `FinishedAt`、`State`、`Reason`、`Error`

`Replay(events)` 不依赖实时调度器，会根据事件重新校验状态迁移、资源 owner
和最终展示版本，并重建 `Snapshot`。测试中使用同一组记录比较实时快照与回放
快照，避免仅以队列吞吐作为正确性结论。

## 使用

```go
s, err := reproject.New(reproject.Config{
    Workers: 2,
    Work: func(ctx context.Context, in reproject.ReprojectionInput, dst *reproject.StagingBuffer) error {
        _, err := dst.Write(in.Pixels)
        return err
    },
})
if err != nil {
    return err
}
defer s.Close()

_ = s.SetPose(1)
taskID, _ := s.SubmitFrame(reproject.Frame{Version: 1, Pixels: input})
record, _ := s.Wait(ctx, taskID)
snapshot := s.Snapshot()
replayed, _ := s.ReplayJournal()
```

帧版本和姿态版本必须单调递增。新帧提交会立即撤销所有旧帧活动请求；新姿态到
达会立即撤销所有旧姿态活动请求。

## 验证

测试覆盖：

- 任务开始后新姿态到达，旧任务迟到完成但不提交；
- 取消运行任务，只回收自身资源；
- 下一帧覆盖旧请求，旧任务迟到完成不改变当前缓冲 ID 和像素；
- 每个场景的实时快照与事件回放结果一致；
- 已提交任务不可再取消。

```bash
go test ./...
go test -race ./...
go vet ./...
```
