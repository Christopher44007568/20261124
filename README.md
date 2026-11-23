# 光学配置管理（Go 标准库实现）

版本化的每眼光学配置：畸变、色差、面板中心、安全边界。
支持保存/重载，正向、反向与往返映射校验；任何字段缺失、版本不兼容
或左右眼格式不一致时，保留上次可用画面并指出具体字段。

## 结构

- `optics/config.go` — 版本化配置类型与字段级错误
- `optics/validate.go` — 必填字段、版本范围、左右眼格式一致性校验
- `optics/mapping.go` — 径向畸变正向映射、牛顿法反向映射、往返校验
- `optics/manager.go` — 保留上次可用配置的管理器，原子化保存/重载
- `cmd/opticsdemo` — 端到端演示

## 运行

```sh
go test ./...
go run ./cmd/opticsdemo
```
