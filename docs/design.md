# 深度/模板生命周期设计（主机 / 右机）

## 1. 设计目标与核心不变量

同一份深度附件被左眼、右眼交替处理时，最典型的缺陷是把上一只眼的解析结果
当成当前帧数据。本设计用一条不变量杜绝该问题：

> 每个派生产物（深度解析、遮罩、模板）都携带 `eye` 与 `attachmentID` 标签；
> 任何读取都必须通过眼别与附件身份双重校验，校验失败即拒绝并写审计日志。

主机（`Host`）与右机（`RightMachine`）各自持有独立的 `Manager`，互不共享
槽位、缓存与日志；机器身份只用于日志事件和模板二进制头。

## 2. 状态机

每只眼一个槽位（`eyeSlot`），状态严格线性推进，清除/格式变化可回到起点：

```
Empty ──Load──▶ Loaded ──Parse──▶ Parsed ──ApplyMask──▶ Masked ──Save──▶ Saved
  ▲                                                    │
  └──────────────── Clear / 格式版本变化 ──────────────┘
```

- 越权转换（未加载先解析、未解析先遮罩、未通过先保存）返回哨兵错误并记
  `invalid-transition` 事件。
- 遮罩被拒绝时槽位停在 `Masked` 且 `MaskResult.Passed=false`，`Save` 明确
  返回 `ErrTemplateRejected`，失败模板永远不会被持久化为“通过”。

## 3. 五个生命周期操作

| 操作 | 入口 | 关键行为 |
| --- | --- | --- |
| 清除 | `Clear(eye)` / `ClearAll()` | 释放该眼全部指针回到 `Empty`，记 `cleared`，不影响另一只眼 |
| 加载 | `Load(eye, raw, frame)` | 校验二进制头并计算 SHA-256 附件 ID；同附件重绑为幂等；格式版本变化先清空派生产物并记 `format-changed` |
| 解析 | `Parse(eye)` | 产出带眼别标签的 `DepthResult`；同附件同眼跨帧直接复用并记 `parse-reused` |
| 保存 | `Save(eye)` | 仅遮罩通过时生成 `Template` 及自描述二进制 blob，头内含 machine/eye 标签 |
| 跨帧复用 | `Reuse(eye, attID, frame)` | 只读判定；附件不符记 `reuse-rejected`，眼别不符记 `stale-eye` |

另有 `Restore(eye, blob)`：恢复落盘模板时校验 machine/eye 标签，跨眼或跨机
恢复按陈旧数据拒绝。

## 4. 陈旧眼别数据如何被抓到

1. `DepthResult`、`MaskResult`、`Template` 均实现 `eyeTagged`
   （`TaggedEye` / `TaggedAttachment`）。
2. 统一入口 `Manager.guard` 比对标签与当前槽位：
   - 眼别不符 → 返回 `ErrStaleEye`，日志写 `stale-eye`，附眼别明细；
   - 附件不符 → 返回 `ErrStaleAttachment`，日志写 `stale-attachment`。
3. `Parse`、`ApplyMask`、`Save`、`Reuse`、`Restore` 全部经过 `guard`，
   因此即使错误数据被外部错误地写入槽位，也不会被静默消费。
4. `MemoryLogger` 提供 `Events / CountByKind / Last`，测试可精确断言
   `stale-eye` 事件的次数、眼别与附件 ID。

## 5. 附件身份与格式变化

- 附件身份 = 原始字节的 SHA-256；格式头为 `magic + version + width + height`
  （大端），载荷为 uint16 深度采样。
- 同一版本、不同载荷：替换附件并丢弃全部派生产物（状态回 `Loaded`），
  不产生 `format-changed`，旧 ID 的复用请求被 `ErrAttachmentMoved` 拒绝。
- 版本号变化：先写 `format-changed`（事件携带新版本号），再清空
  depth/mask/template，强制重新解析、重新遮罩、重新保存。

## 6. 必测场景与对应测试

| 场景 | 测试函数 | 断言要点 |
| --- | --- | --- |
| 模板遮罩仅单眼通过 | `TestMaskSingleEyePassAndSave` | 左眼通过并可保存；右眼槽位保持 `Empty`，无右眼事件 |
| 遮罩拒绝不可保存 | `TestMaskRejectedBlocksSave` | `Passed=false` 时 `Save` 返回 `ErrTemplateRejected` |
| 左眼已解析、右眼未完成 | `TestLeftParsedRightIncomplete` | 右眼的复用/遮罩/保存全部报缺前置状态；左眼附件 ID 不能在右眼复用 |
| 附件格式变化 | `TestAttachmentFormatChangeInvalidatesArtifacts` | `format-changed` 事件、派生产物清空、新版本重新解析 |
| 同版本换载荷 | `TestSameVersionNewPayloadNoStaleReuse` | 无 `format-changed`，但旧结果不可复用 |
| 陈旧眼别数据 | `TestStaleEyeDataCaptured` | 白盒注入左眼结果到右眼槽，`Parse`/`Reuse` 均报 `ErrStaleEye` 且日志可查 |
| 跨机/跨眼恢复 | `TestRestoreRejectsForeignEyeAndMachine` | blob 的 machine/eye 标签不符时拒绝并记 `stale-eye` |
| 主机与右机隔离 | `TestHostAndRightMachineIsolation` | 两台机器独立状态、独立日志，blob 机器标签不同 |
| 并发与跨帧复用 | `TestConcurrentEyeLifecycle` / `TestCrossFrameReuseSameAttachment` | `-race` 通过，同附件复用、异附件拒绝 |

## 7. 目录与依赖

```
depthlife/          生命周期包（仅依赖标准库：crypto/sha256、encoding、sync 等）
cmd/demo/           可运行演示
docs/design.md      本设计文档
```

模块零第三方依赖，`go test ./... -race` 即可全量验证。
