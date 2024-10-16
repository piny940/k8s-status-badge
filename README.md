# k8s-status-badges

kubernetes の healthy 状態を表示するためのバッジを生成するためのツールです。

## 開発

- go をインストール
- `.env.sample` に従って `.env` を作成
- `go run .`

## デプロイ

- PR 作成：テスト実行
- 管理者が承認：staging 環境にデプロイ
- PR をマージ：本番環境にデプロイ
