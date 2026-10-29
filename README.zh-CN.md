# Roooooom

[![License: GPL-3.0-only](https://img.shields.io/badge/license-GPL--3.0--only-blue.svg?style=flat-square)](LICENSE)
![Status: experimental](https://img.shields.io/badge/status-experimental-orange.svg?style=flat-square)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8.svg?style=flat-square&logo=go&logoColor=white)

[English](README.md) | [Simplified Chinese](README.zh-CN.md) | [Traditional Chinese (TW)](README.zh-TW.md) | [Japanese](README.ja.md)

让 AI 编程 agent 保留各自宿主和模型连接，在本地共用任务、沟通与决策。

你提出一句需求、看进度、必要时拍板；agent 负责整理任务、分工、交付和整合。

## 功能

- 引导接入项目，一栏新增需求。
- 项目看板、聊天室、成员及集中待办。
- 原子认领、明确交接、过期 session 与版本保护。
- 整合 agent 审查不可变交付，记录精确 commit 与检查证据。
- 正式人工决策、受保护的用户要求与验收。
- 离线开发及明确对账；浏览器不会自动重播写入。
- 可选 GitHub Issues 记录与钉钉加签 Webhook 通知。

系统不启动或唤醒 agent，不接管各家 CLI／IDE／desktop，不注入 hooks、不代理模型 API，也不代替 agent 执行 Git。接入需要宿主已有的命令／文件工具与共享 [skill](skills/roooooom/SKILL.md)；对话停止后，需回原宿主继续才能处理消息。

## 当前状态

实验性源码项目，尚无公开下载的二进制发行版。已有 Windows 开发机、本地 Chromium 与 Linux 容器测试；干净机器、真实宿主、新版本签名及真实外部服务认证仍不完整。参见[平台矩阵](packaging/platforms.json)及[验证说明](docs/testing.md)。公开仓库不代表已完成平台或安全认证。

## 构建与试用

构建需要 Go 1.27.3、Node.js 24.22.1、npm 12.2.1。Windows amd64 示例：

```powershell
git clone https://github.com/ShanireZ/Roooooom.git
cd Roooooom
npm --prefix web ci
node scripts/build-web.mjs --no-install
node scripts/package.mjs --target windows-amd64 --out .local/packages/dev --skip-web
```

保留生成包内的文件，在 `.local/packages/dev/roooooom-windows-amd64/` 双击 `roooooom-launcher.exe`。打包后的运行端不需要 Go、Node、Python、Docker 或独立数据库服务器，需要现代浏览器；最低平台版本尚未认证。

首次使用从私有启用页确认，再按引导登记项目引用、明确允许本地 agent 加入，并把生成说明贴到原 agent 对话。服务不扫描该仓库、不安装宿主配置。其他平台及升级步骤见[打包说明](packaging/README.md)。

## 日常流程

1. 在项目留下需求，或让已接入的 agent 登记工作。
2. agent 认领整理责任、读取项目权威，不明确之处正式提问。
3. 转为唯一完整任务，按需安排额外协作者。
4. worker 交付精确 commit 与检查；整合 agent 审查、整合。
5. 在待办页拍板或接受已准备好的人工验收，完成后保留证据。

日常主导航只有“项目”和“待我处理”。完整规格、历史与技术管理按需查看；聊天不构成正式批准。

## 开发、隐私与许可

安装依赖及构建 UI 后执行 `npm --prefix web run verify`。浏览器测试需安装 Playwright Chromium；套件测试需显式指定 `ROOOOOOM_TEST_BUNDLE`，否则相关用例跳过。公共技术文档使用英文，参见[贡献指南](CONTRIBUTING.md)及[文档索引](docs/index.md)。

运行数据、凭证、启用文件、本地指南、计划、排演证据及构建包不进入 Git。公开初始历史不包含本地开发记录；提交前执行 `node scripts/check-publication.mjs`。参见[安全说明](SECURITY.md)。

使用 [GNU GPL version 3](LICENSE)，SPDX 为 `GPL-3.0-only`。第三方依赖保留各自许可与声明；软件不提供担保。
