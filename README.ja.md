# Roooooom

[![License: GPL-3.0-only](https://img.shields.io/badge/license-GPL--3.0--only-blue.svg?style=flat-square)](LICENSE)
![Status: experimental](https://img.shields.io/badge/status-experimental-orange.svg?style=flat-square)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8.svg?style=flat-square&logo=go&logoColor=white)

[English](README.md) | [Simplified Chinese](README.zh-CN.md) | [Traditional Chinese (TW)](README.zh-TW.md) | [Japanese](README.ja.md)

各 AI コーディング agent の既存ホストとモデル接続を保ちながら、タスク、会話、判断を共有するローカル協調サービスです。

利用者は短い要望を伝え、進捗を確認し、必要な判断を行います。agent が仕様の整理、分担、成果の提出と統合を担当します。

## 機能

- プロジェクト参加ガイドと、入力欄一つから始める要望登録。
- プロジェクトボード、会話、参加者、判断待ち一覧。
- 原子的な担当取得、明示的な引き継ぎ、古い session と版の保護。
- 統合 agent による不変の提出物の確認と、正確な commit・検査結果の記録。
- 正式な人の判断、利用者が指定した制約、受け入れ確認。
- オフライン作業と明示的な照合。ブラウザーの書き込みは自動再送しません。
- 任意の GitHub Issues 記録と、署名付き DingTalk Webhook 通知。

agent の起動・呼び出し、各社 CLI／IDE／desktop の管理、hooks の注入、モデル API の中継、agent に代わる Git 操作は行いません。既存ホストのコマンド・ファイル操作機能と共有 [skill](skills/roooooom/SKILL.md) が必要です。停止した会話がメッセージを処理するには、元のホストで再開してください。

## 現在の状態

実験的なソースプロジェクトです。公開ダウンロード用のバイナリリリースはまだありません。Windows 開発機、ローカル Chromium、Linux コンテナーの検証がありますが、クリーン環境、実際のホスト、新版の署名、外部サービスの認証は未完了です。[プラットフォーム一覧](packaging/platforms.json)と[検証説明](docs/testing.md)を参照してください。公開リポジトリの存在はサポートや安全性の認証を意味しません。

## ビルドと試用

Go 1.27.3、Node.js 24.22.1、npm 12.2.1 を使用します。Windows amd64 の例：

```powershell
git clone https://github.com/ShanireZ/Roooooom.git
cd Roooooom
npm --prefix web ci
node scripts/build-web.mjs --no-install
node scripts/package.mjs --target windows-amd64 --out .local/packages/dev --skip-web
```

生成されたパッケージ内のファイルを同じ場所に保ち、`.local/packages/dev/roooooom-windows-amd64/roooooom-launcher.exe` を開きます。パッケージの実行には Go、Node、Python、Docker、外部 DB サーバーは不要です。現代的なブラウザーが必要で、最低プラットフォーム版は未認証です。

初回は非公開の有効化ページで確認し、プロジェクト参照とローカル agent の参加許可を設定します。生成された説明を元の agent の会話に貼り付けます。リポジトリの走査やホスト設定のインストールは行いません。ほかの対象と更新方法は[パッケージ説明](packaging/README.md)にあります。

## 日常の流れ

1. プロジェクトに要望を登録するか、接続済み agent に作業登録を依頼します。
2. agent が整理担当を取得し、プロジェクトの根拠を読み、曖昧な点を正式に質問します。
3. 一つの完全なタスクに変換し、必要に応じて協力者を分担します。
4. worker が正確な commit と検査を提出し、統合 agent が確認・統合します。
5. 判断待ちページで回答または準備済み成果の受け入れを行います。完了後も証拠が残ります。

日常の主な入口は Projects と Needs my attention です。詳細仕様、履歴、管理機能は必要なときに開けます。会話は正式な承認にはなりません。

## 開発・プライバシー・ライセンス

依存の導入と UI ビルド後に `npm --prefix web run verify` を実行します。ブラウザーテストには Playwright Chromium、パッケージテストには明示した `ROOOOOOM_TEST_BUNDLE` が必要です。未指定のパッケージケースは skip されます。公開技術文書は英語です。[貢献ガイド](CONTRIBUTING.md)と[文書一覧](docs/index.md)を参照してください。

実行データ、認証情報、有効化ファイル、ローカルガイド、計画、実演証拠、生成パッケージは Git に含めません。公開初期履歴にローカル開発記録は含めません。公開前に `node scripts/check-publication.mjs` を実行してください。[セキュリティ](SECURITY.md)も参照してください。

[GNU GPL version 3](LICENSE)、SPDX `GPL-3.0-only` を適用します。第三者の依存はそれぞれのライセンスと通知を維持します。無保証のソフトウェアです。
