# Nginx Inspector 1.0.10

## Added

- 插件包新增 `icons/logo.png`（144×144 PNG，取自 nginx 官方标识的字形），清单声明
  `icon: "icons/logo.png"`。此前清单没有 `icon` 字段、包内也没有 `icons/` 目录，宿主与
  市场都取不到任何图片，插件管理页只能显示名称首字母「N」。

## Changed

- 版本号 1.0.10；README 打包与安装示例中的压缩包名同步到 1.0.10。

## Verified

- `go run ./cmd/buildtool` 构建产物包含 `icons/logo.png`，并被打进发行 ZIP。
- 用宿主的 `plugin.RasterIconDataURL` 读取该文件可正常转成 `data:image/png;base64,...`
  （9058 字节），确认宿主侧渲染路径可用。
