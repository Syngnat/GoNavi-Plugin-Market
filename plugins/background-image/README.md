# 背景图片

在设置中心「主题与配色 → 背景图片」中管理工作台背景：

- 上传 / 更换自定义背景图片，支持 PNG、JPEG、WebP（自动压缩到 1920px、JPEG 质量自适应）
- 启用 / 关闭背景显示，调节透明度（5%–30%）
- 拖动预览或滑杆调整背景位置，一键居中
- 浏览内置图库，下载、切换、删除图库背景，查看作者与许可信息

## 安装

从插件市场安装，或把本目录（构建后的 `plugin-dist/background-image`）作为本地包安装。

## 构建

```bash
node plugins/background-image/build.mjs
```

构建脚本把 `ui/page.template.html`、`ui/styles.css`、`ui/app.js` 与 `ui/locales.json`
内联进清单的沙箱页面，产物写入仓库根的 `plugin-dist/background-image/`。

## 说明

- 背景的实际绘制与本地存储由宿主完成；插件通过 `host.appearance.read` /
  `host.appearance.write` 能力读写背景状态。
- 插件未安装或停用时，设置中心原位置仍显示「背景图片」入口，并提示安装插件。
- 卸载插件不会清除已设置的背景；宿主继续显示，重新安装后可以继续管理。
