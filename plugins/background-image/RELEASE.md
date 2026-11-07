# 背景图片 1.0.0

在设置中心「主题与配色 → 背景图片」原地接管背景设置：上传/更换自定义背景、启用开关、
透明度、位置调整与内置图库的浏览、下载、切换、删除全部由插件页面完成；宿主只负责绘制、
存储与图库落地。插件未安装或停用时，设置中心保留原入口并提示安装。

- 最低宿主版本：`1.2.3`（`contributes.settingsSections` 与 `appearance.*` 能力起始版本）
- 产物：`gonavi-plugin-background-image-1.0.0.zip`
- 传输大小：22,957 bytes
- SHA-256：`17da0336d682e40534bae01eb064bcb5ff470b7cafb0d46abfc93a3e4a471e09`
- 规范内容摘要：`354b10470d2f096260082a14604f38ce9f16e59d6ce474f493d4d44d60b3a352`
- 包内容：`gonavi.plugin.json`（沙箱页面已内联）、`README.md`、`LICENSE`

## 发布步骤

1. 构建并打包：

   ```bash
   node plugins/background-image/build.mjs
   # 将 plugin-dist/background-image 内容打包为 gonavi-plugin-background-image-1.0.0.zip
   ```

2. 建立并推送 tag，用该 tag 创建 GitHub Release 并上传 ZIP：

   ```bash
   git tag background-image-v1.0.0
   git push origin background-image-v1.0.0
   ```

3. 下载 Release 资产复核 SHA-256 后，把条目追加到 `public/catalog.json` 的 `items` 再推送
   `main`（`public/**` 变更会触发站点部署）。**不要在 Release 可下载前写入死链。**

## 验收

- 宿主 1.2.3 及以上：安装/替换/启用插件后，设置中心原位渲染插件页面，不出现安装提示。
- 上传背景、切换启用、调整透明度与位置、图库下载/切换/删除均生效，刷新后状态保留。
- 停用或卸载插件：原位置显示安装提示，已设置的背景仍由宿主继续绘制。
