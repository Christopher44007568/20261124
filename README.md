# late-latch 可重放工具

这是一个只使用 Go 标准库的 late-latching 回放器。它把每一帧的提交、截止、合成和 scanout 固定成四条可审计时间线，并用矩阵版本、姿态样本版本、PNG 画面和资源读取记录建立完整对照关系。

## 运行

```bash
go test ./...
go run ./cmd/replay -list
go run ./cmd/replay -scenario all -out artifacts/replay
go run ./cmd/replay -scenario deadline-plus-1ms -out artifacts/replay
go run ./cmd/replay -scenario scenarios/custom-on-time.json -out artifacts/replay
```

`-scenario all` 会分别运行内置的四个注入场景。自定义场景使用绝对纳秒时间，格式参考 `scenarios/custom-on-time.json`。每个场景输出到 `-out/<scenario-name>/`。

## 锁定规则

- 帧的 `submitNs` 使用上一帧已经提交的矩阵作为临时相机。
- 截止时间比较为闭区间：`receivedAt <= deadlineNs` 的数据可参与当前帧。
- 一帧的输入窗口是从上一帧截止后到本帧截止。窗口内到达的左右眼样本在 `deadline` 时统一锁定。
- 左右眼必须同时存在、均未丢失，而且两只眼都带来了相对于当前矩阵的新样本，才会创建新矩阵版本。只有单眼变化的帧记为 `held_incomplete_update`，继续使用旧矩阵。
- 同一只眼出现相同 `timestampNs` 时，第一个包生效；后续包记为 `duplicate`，不增加姿态版本，也不创建矩阵。
- 比该眼最新样本更旧的包记为 `out_of_order`，不会回退姿态。
- `tracking_lost` 使该眼在截止时不可用，帧继续使用上一版矩阵，并标记为 `held_tracking_lost`。只有更晚时间戳的有效样本才能恢复追踪。
- 活动帧截止后才到达的数据不会影响该帧，只会进入下一帧输入窗口；`queuedFromFrameId` 和 `lateQueuedEventIds` 会明确记录这次排队。

矩阵版本只在左右眼成对提交时递增。每一帧的 `usedLeftSampleId`、`usedRightSampleId`、`usedLeftPoseVersion`、`usedRightPoseVersion` 和 `usedMatrixVersion` 共同回答“这一帧到底用了哪一版头姿态”。

## 四条时间线

每次运行都会生成四个独立的 JSONL 文件，一行对应一帧：

- `timeline-submit.jsonl`：提交时刻、临时矩阵、提交前已到达的待选事件。
- `timeline-deadline.jsonl`：截止前矩阵、最终矩阵、是否替换、接受/忽略/追踪/晚到排队的事件 ID。
- `timeline-composite.jsonl`：冻结后的矩阵版本，以及为 late-latch 重投影读取的旧资源 ID。
- `timeline-scanout.jsonl`：最终矩阵、PNG 路径和摘要、扫描输出资源 ID。

四个文件加上 `replay.json` 中的 `timelines` 字段内容一致，便于逐个 diff 或用脚本串联。

## 交叉对照

主要产物：

- `replay.json`：场景、输入分类、矩阵历史、帧摘要、资源、资源读取、四条时间线和一致性检查。
- `summary.json`：矩阵数、替换帧数、迟到排队数、重复时间戳数、追踪丢失次数等。
- `audit.csv`：按帧汇总提交/截止/合成/scanout 时刻、矩阵版本、左右眼姿态版本、PNG 摘要、旧资源读取及旧矩阵版本。
- `frames/Fxxxx.png`：确定性渲染的双目画面。
- `timeline-*.jsonl`：四条时间线。

对照路径如下：

1. `frames[].usedMatrixVersion` -> `matrices[].version`。
2. `matrices[].leftSampleId/rightSampleId` -> `inputs[].sampleId` -> 原始 `scenario.events[]`。
3. `frames[].imageSha256` -> `frames/Fxxxx.png` -> `audit.csv` 的 `imageSha256`。
4. `frames[].oldResourceReadIds` -> `resourceReads[].id` -> 生产者帧矩阵版本与消费者矩阵版本。
5. `resources[].producerFrameId` 和 `producerMatrixVersion` 描述每个颜色/深度资源的来源。

`replay.json` 的 `checks` 会验证矩阵版本、姿态样本版本、PNG 摘要、资源生产者、旧资源消费者、四条时间线长度和迟到排队关系。CLI 在检查失败时返回非零退出码。

PNG 顶部 10 行是可视版本指纹：从左到右依次为矩阵版本、左眼姿态版本、右眼姿态版本，各占 64 像素；每个版本是 16 个 4 像素块，低位在左，白色表示 1。

## 内置注入场景

| 场景 | 注入与预期 |
| --- | --- |
| `deadline-minus-1ms` | F0003 截止前 1 ms 到达左右眼新姿态；F0003 直接创建并冻结 M2。 |
| `deadline-plus-1ms` | F0003 截止后 1 ms 到达左右眼新姿态；F0003 保持 M1 并记录晚到排队，F0004 才创建 M2。 |
| `duplicate-timestamp` | 覆盖截止前和晚到路径的重复捕获时间戳；重复包全部忽略，矩阵版本不增加。 |
| `tracking-loss` | F0003 右眼追踪丢失，矩阵保持 M1；F0004 右眼恢复且左眼已有新样本后创建 M2。 |

## 场景 JSON

时间字段均为 `int64` 纳秒。`kind` 支持 `pose` 和 `tracking_lost`，`eye` 支持 `left` 和 `right`。

```json
{
  "schemaVersion": 1,
  "name": "example",
  "frameCount": 4,
  "firstSubmitNs": 0,
  "framePeriodNs": 16666667,
  "deadlineOffsetNs": 8000000,
  "compositeOffsetNs": 8100000,
  "scanoutOffsetNs": 15000000,
  "events": [
    {
      "atNs": 23666667,
      "kind": "pose",
      "eye": "left",
      "timestampNs": 18000000,
      "yawDeg": 3.0,
      "pitchDeg": 1.0,
      "rollDeg": 0.0
    }
  ]
}
```

`atNs` 是接收时刻，`timestampNs` 是追踪设备捕获时刻。`tracking_lost` 不需要 `timestampNs`；恢复由之后到达且时间戳更新的 `pose` 事件完成。

## 测试覆盖

`go test ./...` 覆盖：

- 截止前 1 ms 同帧替换，以及截止后 1 ms 排队到下一帧。
- 重复时间戳不增加姿态/矩阵版本。
- 追踪丢失冻结矩阵，恢复后才推进版本。
- 单眼更新等待另一只眼形成完整新姿态对。
- 四条时间线、PNG、CSV 和 JSONL 产物完整性。
- 画面摘要、资源生产者和旧资源读取的交叉校验。
- 同一场景重复回放结果逐字节一致。
