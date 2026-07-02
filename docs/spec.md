# SmartDB 要件定義書 v3.1

## 1. 概要

### 1.1 目的

複数の個人開発プロジェクトや小〜中規模サービス向けに、SQLiteベースのデータベース環境を安全かつ簡単に構築・管理できるセルフホスト型プラットフォームを提供する。

### 1.2 解決する課題

* プロジェクトごとのDB環境構築が面倒
* SQLiteファイル管理が煩雑
* Migration運用が属人的
* バックアップ・復元手順が統一されていない
* 複数プロジェクト間の設定やデータの混線リスク

### 1.3 コンセプト

> SQLiteをクラウドDBのように扱えるセルフホスト管理基盤

本システムは「SQLiteサーバー」ではなく「SQLiteプロジェクト管理プラットフォーム」として設計する。

---

# 2. 想定ユーザー

## メインターゲット

* 個人開発者
* フリーランスエンジニア
* 自宅サーバー運用者
* 小規模チーム

## 想定規模

* 数個〜数百個のプロジェクト管理
* 数MB〜数GB規模のSQLite DB
* 数十〜数百 Write/sec 程度

---

# 3. システム構成

## 3.1 デプロイ方式

* Docker Compose
* Linux
* Ubuntu
* Proxmox
* LXC
* Docker

---

## 3.2 アーキテクチャ

```text
smartdb
├─ REST API
├─ Authentication
├─ Project Manager
├─ Migration Engine
├─ Backup Manager
├─ SQLite Manager
└─ Web UI
```

---

## 3.3 Project単位管理

本システムの最小管理単位はDatabaseではなくProjectとする。

```text
project-a
├─ database.db
├─ migrations/
├─ backups/
└─ logs/
```

---

# 4. 技術要件

## バックエンド

* Go

候補:

* net/http
* chi
* gin

---

## データベース

* SQLite 3
* WALモード必須

```sql
PRAGMA journal_mode=WAL;
```

---

## 認証

API Key認証

```http
Authorization: Bearer <API_KEY>
```

---

## 設定保存

内部管理用メタデータはSQLiteを使用

---

# 5. プロジェクト構造

```text
/data

project-a/
├─ database.db
├─ migrations/
│   ├─ 001_init.sql
│   └─ 002_users.sql
├─ backups/
└─ logs/

project-b/
├─ database.db
└─ ...
```

---

# 6. API要件

## 管理API

### Project作成

```http
POST /api/v1/projects
```

---

### Project削除

```http
DELETE /api/v1/projects/{project}
```

---

### Project一覧

```http
GET /api/v1/projects
```

---

### Project詳細

```http
GET /api/v1/projects/{project}
```

---

### Project更新

```http
PATCH /api/v1/projects/{project}
```

---

### Project状態取得

```http
GET /api/v1/projects/{project}/stats
```

Response

```json
{
  "size": "120MB",
  "tables": 12,
  "backup_count": 8,
  "migration_version":"01_init"
}
```

---

## SQL API

### テーブル一覧取得

```http
GET /api/v1/projects/{project}/tables
```

Response

```json
{
  "table": [...]
}
```

---

### テーブルスキーマ取得

```http
GET /api/v1/projects/{project}/tables/{table}
```

Response

```json
{
  "schema": [...]
}
```

---

### SQL実行

```http
POST /api/v1/projects/{project}/sql
```

認証は `Authorization: Bearer <token>` ヘッダで行う（§7 参照）。

Request

```json
{
  "sql": "SELECT * FROM users"
}
```

Response

```json
{
  "success": true,
  "result": {
    "rows": [...],
    "affectedRows": 0
  }
}
```

---

## API Key管理

### 発行

```http
POST /api/v1/projects/{project}/apikeys
```

### 一覧

```http
GET /api/v1/projects/{project}/apikeys
```

### 無効化

```http
DELETE /api/v1/projects/{project}/apikeys/{id}
```

---

# 7. 権限設計

## System Key

`project_id = NULL`、ロールは常に `system`。DB単位(個々のプロジェクトのデータ)ではなく、より上位のフリート管理のためのキー。

可能操作

* Project作成
* Project一覧
* Project削除(論理削除、state→deleted)
* Project wipe(実データ削除、state→wiped)
* プロジェクトへの初期APIキー発行(新規プロジェクトを誰も触れない状態にしないための例外)

不可

* SQL実行・Query
* Backup / Restore

起動時に `api_keys` テーブルにSystem Keyが存在しない場合、自動でブートストラップされる。`SDB_SYSTEM_TOKEN` 環境変数を指定すればそのトークンを使用し、未指定ならランダム生成してログに一度だけ出力する。追加のSystem Keyを発行するAPIは用意せず、必要な場合は環境変数側で管理する。

---

## Admin Key

プロジェクト内での全権限(プロジェクトを跨いだ操作は不可)

可能操作

* Migration
* Backup
* Restore
* Query
* プロジェクト内APIキーの発行・失効

---

## Read/Write Key

可能操作

* Query
* Insert
* Update
* Delete

不可

* Project操作
* Migration
* Backup
* Restore

---

## Read Only Key

可能操作

* SELECT

不可

* INSERT
* UPDATE
* DELETE
* DDL

---

# 8. Migration機能

## CLI提供

```bash
sdb-cli create blog
```

```bash
sdb-cli migration create add_users
```

```bash
sdb-cli up
```

```bash
sdb-cli down
```

---

## Migration管理テーブル

```sql
CREATE TABLE __migrations (
    version TEXT PRIMARY KEY,
    applied_at DATETIME
);
```

---

## 機能

* Up
* Down
* Status確認
* 差分確認

---

# 9. バックアップ機能

## 手動バックアップ

```http
POST /api/v1/projects/{project}/backup
```

---

## 自動バックアップ

設定可能

例

```text
毎日
毎週
毎月
```

---

## 復元

```http
POST /api/v1/projects/{project}/restore
```

---

## 世代管理

保持世代数を設定可能

例

```text
7世代
30世代
90世代
```

---

# 10. Web UI

## 必須

### Dashboard

表示内容

* Project一覧
* DBサイズ
* 最終バックアップ日時
* Migration状態

---

### Project画面

表示内容

* テーブル一覧
* レコード閲覧
* SQL実行
* API Key管理
* Backup管理

---

# 11. 障害対策

## Query Timeout

デフォルト

```text
5秒
```

---

## Project Lock

プロジェクトIDごとにプロセス内ロック（共有/排他）を保持する。

* **共有ロック**（同時に複数保持可）: 通常のSQL実行（Query/Execute）、Backup（`VACUUM INTO`はWALのスナップショットを取るため通常SQLと干渉しない）
* **排他ロック**（保持中は他の共有・排他ロックを一切許可しない）: Restore、Migration

ロックを即座に取得できない場合は待機せず `409 Conflict`（`PROJECT_LOCKED`）を返す。別プロジェクトのロックとは独立。

---

## DB破損対策

* 自動バックアップ
* WAL運用
* Restore機能

---

## セキュリティ

* プロジェクトIDのバリデーション（`^[a-z0-9][a-z0-9_-]*$` のみ許可）
* パス横断攻撃の防止（`../` を含むIDの拒否）
* リクエストボディサイズ制限（デフォルト1MB、`SDB_MAX_BODY_BYTES` で変更可。超過時は 413 Request Entity Too Large）
* SQL分類の正確性（GoATS構文解析による分類、§6 QueryJudge参照）

---

# 12. インフラ要件

## 設定管理

環境変数による設定。未指定時はデフォルト値を使用。

| 環境変数 | デフォルト | 説明 |
|---|---|---|
| SDB_PORT | 8080 | HTTPサーバポート |
| SDB_DATA_DIR | ./data | プロジェクトデータ格納先 |
| SDB_LOG_LEVEL | info | ログレベル |
| SDB_QUERY_TIMEOUT | 5s | SQLクエリタイムアウト |
| SDB_SYSTEM_TOKEN | (未指定時はランダム生成) | System Keyのトークンを固定したい場合に指定 |
| SDB_MAX_BODY_BYTES | 1048576 (1MB) | リクエストボディサイズ上限（バイト） |

## Graceful Shutdown

SIGTERM/SIGINT受信時、処理中リクエスト完了後にDB接続をクリーンに閉じる。

## Health Check

```http
GET /health
```

Response

```json
{
  "status": "ok",
  "version": "1.0.0"
}
```

## エラーレスポンス形式

全エンドポイント共通。

```json
{
  "error": {
    "code": "INVALID_PROJECT_NAME",
    "message": "Project name must match [a-z0-9][a-z0-9_-]*"
  }
}
```

## Request ID

全レスポンスに `X-Request-ID` ヘッダを付与。ログにも同IDを記録し、トレーサビリティを確保する。

---

# 13. v1.0 スコープ

(旧§12)

実装対象

* Project管理
* SQLite管理
* API Key認証
* SQL実行
* Migration
* Backup
* Restore
* Web UI

実装対象外

* GraphQL
* 自動CRUD生成
* レプリケーション
* 分散構成
* マルチノード
* ジョブキュー
* Workerシステム

---

# 14. v2候補

* Job Queue
* Workerシステム
* 非同期バックアップ
* 非同期集計
* Webhook
* Git連携Migration
* S3バックアップ
* PostgreSQL対応

---

# 15. 成功指標

* Docker Composeのみで起動可能
* 5分以内に新規Project作成可能
* Migration適用がCLIのみで完結
* バックアップから復元可能
* 個人開発者がDB管理を意識せず利用可能
