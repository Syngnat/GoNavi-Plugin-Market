# Nginx Inspector 1.0.7

## Added

- 清单新增 `contributes.navigation`(`nginx.inspector`,`target: window`,`location: tools`):
  宿主会把「Nginx Inspector」入口放到标题栏工具条,与 GoNavi SSH 位于同一区域,点击直接打开
  插件声明的独立窗口(1280×840)。宿主只在插件显式声明入口时才显示按钮,未声明的插件仍从
  插件管理详情页的「打开窗口」进入。

## Changed

- 版本号 1.0.7;README 安装说明补充顶栏入口与新的打包命令版本号。

## Verified

- `node scripts/verify-plugin-catalog.mjs`:清单与 catalog 校验通过(catalog 在打 tag 发布后
  由 CI 更新到 1.0.7)。
- 本机安装 1.0.7 后 `state.json` 中 `contributes.navigation` 与 `window` 均存在、`state=running`,
  点击后宿主拉起 `--plugin-window` 子进程。
