# SmartDB App HTTP API

## Create Project

POST /api/v1/projects

Request

```json
{
  "name": "blog"
}
```

Response

```json
{
  "id": "blog"
}
```

---

## Query

POST /api/v1/projects/{project}/query

Request

```json
{
  "sql": "SELECT * FROM users"
}
```

Response

```json
{
  "rows": []
}
```
