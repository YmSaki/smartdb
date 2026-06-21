# SmartDB

> わざわざVibeCodingなしでこれ作ってるのほめてほしい。これGo4日目の人間が書くコードじゃないよな！？

SQLiteをクラウドDBのように扱える、個人開発・小規模サービス向けのセルフホスト型プロジェクト管理プラットフォーム。

## 特徴

- **プロジェクト単位の管理**: 1つのシステムで複数のSQLiteデータベース環境を独立して管理
- **REST API経由の操作**: プロジェクトの作成・削除や、SQLの実行が可能
- **今後のロードマップ**: Migration機能、自動バックアップ、Web UIの搭載を予定

## 開発環境の起動

本プロジェクトではタスクランナーとして [Task](https://taskfile.dev/) を使用しています。

### 1. 依存関係の解決

```bash
go mod download
```

### 2. アプリケーションの起動

```bash
task run
```

デフォルトでは `http://localhost:8080` でサーバーが起動します。

### 3. テストと品質管理

```bash
# フォーマット
task fmt

# リンター実行
task lint

# テストの実行
task test

```

## API 仕様（簡易版）

詳細な仕様は `docs/api.md` を参照してください。

### プロジェクト作成

* **URL**: `POST /api/v1/projects`
* **Body**: `{"name": "my-project"}`

### プロジェクト削除

* **URL**: `DELETE /api/v1/projects/{project_id}`

---

© 2026 YmSaki
