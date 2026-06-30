# SmartDB HTTP API Reference

認証: 特記なきエンドポイントは `Authorization: Bearer <token>` ヘッダ必須。

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

認証: システム管理キー必須。

Request

```json
{
  "state": "active"
}
```

Response (200): 更新後のプロジェクト情報。

### Delete Project

DELETE /api/v1/projects/{project}

認証: システム管理キー必須。

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
- admin (CREATE, DROP, ALTER): affectedRows 返却

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

Response: 204 No Content

論理削除 (`revoked_at` 設定)。物理削除はしない。

---

## Backup & Restore

### Create Backup

POST /api/v1/projects/{project}/backup

Response (200)

```json
{
  "backup": "database_20260101_120000.db"
}
```

### Restore from Backup

POST /api/v1/projects/{project}/restore

Request

```json
{
  "backup": "database_20260101_120000.db"
}
```

Response (200)

```json
{
  "restored": "database_20260101_120000.db"
}
```

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

## ヘッダ

全レスポンスに `X-Request-ID` ヘッダ付与。
