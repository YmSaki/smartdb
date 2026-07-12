# SmartDB HTTP API Reference

認証: 特記なきエンドポイントは `Authorization: Bearer <token>` ヘッダ必須。

## APIキーのロール

APIキーは `project_id` の有無で「システムキー」と「プロジェクトキー」に大別され、ロールも別体系になっている。

* **システムキー**(`project_id` = NULL): ロールは常に `system`。プロジェクトの作成・一覧・削除・wipe(§Projects参照)のみ実行可能。**個々のプロジェクトのSQL実行・バックアップ・リストア等、データそのものに触れる操作は一切できない**(意図的な設計。DB単位ではなく、より上位のプロジェクト管理のためのキー)。ただし新規プロジェクト作成直後にそのプロジェクト用の最初のAPIキーを発行する用途に限り、`POST /projects/{project}/apikeys` の呼び出しは許可されている。
* **プロジェクトキー**(`project_id` = 対象プロジェクトID): ロールは `admin` / `read_write` / `read_only` のいずれか。SQL実行・バックアップ・API Key管理など、そのプロジェクト内の操作を行う。

起動時、`api_keys` テーブルにシステムキーが1件も存在しない場合、自動的にブートストラップされる。

* 環境変数 `SDB_SYSTEM_TOKEN` が設定されている場合、そのトークンをハッシュ化してシステムキーとして登録する(運用者が任意のトークンを固定できる)。
* 未設定の場合、ランダムなトークンを生成し起動ログに一度だけ出力する。

---

## Health Check

GET /health

認証不要。

Response

```json
{
  "status": "ok",
  "version": "1.0.0"
}
```

---

## Projects

### Create Project

POST /api/v1/projects

認証: システム管理キー必須。

Request

```json
{
  "name": "blog"
}
```

Response (201)

```json
{
  "projectID": "blog-a1b2c3d4"
}
```

### List Projects

GET /api/v1/projects

認証: システム管理キー必須。

Response (200)

```json
[
  {
    "ID": "blog-a1b2c3d4",
    "Name": "blog",
    "State": "active",
    "CreatedAt": "2026-01-01T00:00:00Z",
    "UpdatedAt": "2026-01-01T00:00:00Z"
  }
]
```

### Project Detail

GET /api/v1/projects/{project}

認証: システム管理キーまたは該当プロジェクトキー。

Response (200)

```json
{
  "ID": "blog-a1b2c3d4",
  "Name": "blog",
  "State": "active",
  "CreatedAt": "2026-01-01T00:00:00Z",
  "UpdatedAt": "2026-01-01T00:00:00Z"
}
```

### Update Project

PATCH /api/v1/projects/{project}

認証: システムキーまたは該当プロジェクトキー。

Request

```json
{
  "state": "active"
}
```

`state` に `deleted` / `deleting` / `wiped` を指定することはできない(403)。これらは `DELETE /projects/{project}` と `POST /projects/{project}/wipe` (共にシステムキー限定)経由でのみ遷移可能。

Response (200): 更新後のプロジェクト情報。

### Delete Project

DELETE /api/v1/projects/{project}

認証: システムキー必須。

state を `deleted` に遷移させる論理削除。`domain.CanTransitionTo` により現在の state からの遷移が許可されている場合のみ成功する(例: `creating` からの直接削除や、二重削除は 409 `INVALID_TRANSITION`)。実データやAPIキーはこの時点ではまだ残る(下記 Wipe Project 参照)。

Response: 204 No Content

### Wipe Project

POST /api/v1/projects/{project}/wipe

認証: システムキー必須(プロジェクト側のadminキーでも不可)。

対象プロジェクトが `deleted` 状態である場合のみ実行可能。以下を実施する:

1. そのプロジェクトに紐づく全APIキーを失効
2. プロジェクトディレクトリ(database.db・backups・migrations)をディスクから完全削除
3. state を `wiped` に遷移

`deleted` 以外の状態のプロジェクトに対しては 409 `INVALID_TRANSITION` を返す。

Response: 204 No Content

---

## Project Stats

GET /api/v1/projects/{project}/stats

Response (200)

```json
{
  "size": 16384,
  "tables": 3,
  "backup_count": 2,
  "migration_version": "001"
}
```

---

## Tables

### List Tables

GET /api/v1/projects/{project}/tables

内部テーブル (`__` prefix) は除外。

Response (200)

```json
{
  "tables": ["users", "posts", "comments"]
}
```

### Table Schema

GET /api/v1/projects/{project}/tables/{table}

Response (200)

```json
{
  "schema": [
    {
      "cid": 0,
      "name": "id",
      "type": "INTEGER",
      "notnull": 1,
      "dflt_value": null,
      "pk": 1
    }
  ]
}
```

---

## SQL Execute

POST /api/v1/projects/{project}/sql

Restore/Migration中は409 `PROJECT_LOCKED`（§spec.md Project Lock参照）。

Request

```json
{
  "sql": "SELECT * FROM users"
}
```

Response (200)

```json
{
  "success": true,
  "result": {
    "rows": [{"id": 1, "name": "Alice"}],
    "affectedRows": 0
  }
}
```

SQL分類 (GoATS):
- read (SELECT, WITH+SELECT): rows 返却
- edit (INSERT, UPDATE, DELETE): affectedRows 返却
- manage (PRAGMA, EXPLAIN): rows 返却
- admin (CREATE, DROP, ALTER, VACUUM): affectedRows 返却
- 実行不可 (400 `INVALID_SQL`): `ATTACH` / `VACUUM INTO`。プロジェクトの
  admin キーであっても、プロセスが読み書きできる任意のファイル（他プロジェクトの
  database.db や system.db を含む）に到達できてしまうため、adminロールでも
  実行を許可しない。素の `VACUUM`（INTO句なし、対象DBファイル内で完結する
  最適化コマンド）は admin カテゴリとして実行可能。

ロール別の実行可否（`internal/auth/authorize.go` `CheckSQLPermission`）:
- `admin`: read / edit / manage / admin すべて実行可能。
- `read_write`: read / edit のみ実行可能。manage（`PRAGMA`等）・admin は403 `FORBIDDEN`。
- `read_only`: read のみ実行可能。edit・manage・admin は403 `FORBIDDEN`。

---

## API Keys

### Create API Key

POST /api/v1/projects/{project}/apikeys

Request

```json
{
  "name": "CI Pipeline Key",
  "role": "read_write"
}
```

Response (201)

```json
{
  "id": "a1b2c3d4e5f6",
  "name": "CI Pipeline Key",
  "role": "read_write",
  "token": "sdb_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

`token` はこのレスポンスでのみ返却。再取得不可。

### List API Keys

GET /api/v1/projects/{project}/apikeys

Response (200)

```json
[
  {
    "id": "a1b2c3d4e5f6",
    "name": "CI Pipeline Key",
    "role": "read_write",
    "created_at": "2026-01-01T00:00:00Z",
    "revoked_at": null
  }
]
```

`token_hash` は含まれない。

### Revoke API Key

DELETE /api/v1/projects/{project}/apikeys/{id}

認証: プロジェクトのadminキーまたはsystemキーのみ(read_write/read_onlyキーは403)。

Response: 204 No Content

論理削除 (`revoked_at` 設定)。物理削除はしない。

---

## Backup & Restore

いずれもプロジェクトのadminキーのみ実行可能(read_write/read_only/systemキーは403)。Project Lock（§spec.md参照）によりRestore/Migrationと衝突する場合は409 `PROJECT_LOCKED`。

### Create Backup

POST /api/v1/projects/{project}/backup

Response (200)

```json
{
  "backup": "20260101-120000-a1b2c3d4.db"
}
```

### Restore from Backup

POST /api/v1/projects/{project}/restore

Request

```json
{
  "backup": "20260101-120000-a1b2c3d4.db"
}
```

Response: 204 No Content

---

## エラーレスポンス

全エンドポイント共通形式。

```json
{
  "error": {
    "code": "INVALID_PROJECT_NAME",
    "message": "Project name must match [a-z0-9][a-z0-9_-]*"
  }
}
```

主要エラーコード:
- `UNAUTHORIZED` (401): 認証なし/不正トークン
- `FORBIDDEN` (403): 権限不足/プロジェクト不一致
- `PROJECT_NOT_FOUND` (404): プロジェクト不在
- `INVALID_PROJECT_ID` (400): ID不正/パス横断
- `INVALID_SQL` (400): SQL分類失敗/空クエリ
- `SQL_ERROR` (400): SQLite実行エラー
- `PROJECT_LOCKED` (409): Restore/Migration中で対象Projectがロックされている（§Project Lock参照）

## ヘッダ

全レスポンスに `X-Request-ID` ヘッダ付与。
