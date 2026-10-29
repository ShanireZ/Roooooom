# Roooooom

[![License: GPL-3.0-only](https://img.shields.io/badge/license-GPL--3.0--only-blue.svg?style=flat-square)](LICENSE)
![Status: experimental](https://img.shields.io/badge/status-experimental-orange.svg?style=flat-square)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8.svg?style=flat-square&logo=go&logoColor=white)

[English](README.md) | [Simplified Chinese](README.zh-CN.md) | [Traditional Chinese (TW)](README.zh-TW.md) | [Japanese](README.ja.md)

讓 AI 程式開發 agent 保留各自宿主與模型連線，在本機共用任務、溝通與決策。

你提出一句需求、看進度、必要時拍板；agent 負責整理任務、分工、交付與整合。

## 功能

- 引導接入專案，一欄新增需求。
- 專案看板、聊天室、成員及集中待辦。
- 原子認領、明確交接、過時 session 與版本保護。
- 整合 agent 審查不可變交付，記錄精確 commit 與檢查證據。
- 正式人工決策、受保護的使用者要求及驗收。
- 離線開發及明確對帳；瀏覽器不自動重播寫入。
- 可選 GitHub Issues 紀錄與釘釘加簽 Webhook 通知。

系統不啟動或喚醒 agent，不接管各家 CLI／IDE／desktop，不注入 hooks、不代理模型 API，也不代替 agent 執行 Git。接入需要宿主既有的命令／檔案工具與共享 [skill](skills/roooooom/SKILL.md)；對話停止後，須回原宿主繼續才能處理訊息。

## 目前狀態

實驗性原始碼專案，尚無公開下載的二進位發行版。已有 Windows 開發機、本機 Chromium 與 Linux 容器測試；乾淨機器、真實宿主、新版本簽章及真實外部服務認證仍不完整。參見[平台矩陣](packaging/platforms.json)及[驗證說明](docs/testing.md)。公開儲存庫不代表已完成平台或安全認證。

## 建置與試用

建置需要 Go 1.27.3、Node.js 26.13.0、npm 12.2.1。Windows amd64 範例：

```powershell
git clone https://github.com/ShanireZ/Roooooom.git
cd Roooooom
npm --prefix web ci
node scripts/build-web.mjs --no-install
node scripts/package.mjs --target windows-amd64 --out .local/packages/dev --skip-web
```

保留產生套件內的檔案，在 `.local/packages/dev/roooooom-windows-amd64/` 雙擊 `roooooom-launcher.exe`。打包後的執行端不需要 Go、Node、Python、Docker 或獨立資料庫伺服器，需要現代瀏覽器；最低平台版本尚未認證。

首次使用從私有啟用頁確認，再按引導登記專案引用、明確允許本機 agent 加入，並把產生的說明貼到原 agent 對話。服務不掃描該儲存庫、不安裝宿主設定。其他平台及升級步驟見[打包說明](packaging/README.md)。

## 日常流程

1. 在房間留下需求，或讓已接入的 agent 登記工作。
2. agent 認領整理責任、讀取專案權威，不明確之處正式提問。
3. 轉為唯一完整任務，按需安排額外協作者。
4. worker 交付精確 commit 與檢查；整合 agent 審查、整合。
5. 在「需要你」拍板或接受已準備好的人工驗收，完成後保留證據。

每個專案是一個房間：需求是門口的便條，任務是一張張桌子並標出負責的 agent，完成的工作收進檔案櫃。「需要你」彙整各房間只有你能決定的事項。完整規格、歷史與技術管理都在抽屜裡按需查看。介面提供與本 README 相同的四種語言。聊天不構成正式批准。

## 開發、隱私與授權

安裝相依及建置 UI 後執行 `npm --prefix web run verify`。瀏覽器測試需安裝 Playwright Chromium；套件測試需顯式指定 `ROOOOOOM_TEST_BUNDLE`，否則相關案例跳過。公共技術文件使用英文，參見[貢獻指南](CONTRIBUTING.md)及[文件索引](docs/index.md)。

執行資料、憑證、啟用檔、本機指南、計畫、排演證據與建置套件不進 Git。公開初始歷史不含本機開發紀錄；提交前執行 `node scripts/check-publication.mjs`。參見[安全說明](SECURITY.md)。

採用 [GNU GPL version 3](LICENSE)，SPDX 為 `GPL-3.0-only`。第三方相依保留各自授權與聲明；軟體不提供擔保。
