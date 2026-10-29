# 華龍算力 1.2.0 发布清单

## 变更

- `account.listModels` 在原有 `id` 与 `keyIds` 之外，一并返回网关给的模型名称与上下文窗口。
- 网关字段兼容：`name` / `display_name` 与 `context_length` / `context_window` / `max_context_length`；
  名称超 120 字节、窗口非正或超 2000 万一律视为没给。
- 同一模型被多把 Key 收录时：名称取首个非空、窗口取最大值，`keyIds` 语义不变。
- 宿主缺失元数据时用 models.dev 公共目录补齐，插件不内建模型表。

## 产物

- ZIP：`plugin-dist/gonavi-plugin-hualong-ai-1.2.0.zip`
- SHA-256：`587a4b76e3800650f64631bd0f4bddfdcf994848b16b7144208b28e6f346a1be`
- 传输大小：15,865,451 bytes
- 规范内容摘要：`05eeba305ebf1afe5ae7493858449fd49c90353eca56806d55a3086df766f341`
- 展开大小：36,220,584 bytes
- 平台：Windows amd64、Linux amd64/arm64、macOS amd64/arm64
- 宿主最低版本：1.1.5（不变）

## 发布步骤

1. 打 tag 并推送：

   ```sh
   git tag hualong-ai-provider-v1.2.0
   git push origin hualong-ai-provider-v1.2.0
   ```

2. 用该 tag 建 GitHub Release，上传上面的 ZIP；资产名保持
   `gonavi-plugin-hualong-ai-1.2.0.zip`。

3. 下载 Release 资产复核 SHA-256，再更新 `public/catalog.json` 的对应条目。

## 注意

- 打包必须使用市场仓根 `cmd/buildtool`。清单声明了 `contributes.aiProviders[].icon`
  时包内必须带该文件，否则宿主 `InspectPackage` 会以「declared entry is not a regular
  package file」拒绝安装；1.2.0 打包前已补上 `icons` 目录的复制（含回归测试）。
- 不提交 `plugin-dist/`、ZIP 或运行态数据。
- 宿主侧模型区（名称与上下文档位）依赖 models.dev 公共目录，属宿主能力，与本插件版本无关。
