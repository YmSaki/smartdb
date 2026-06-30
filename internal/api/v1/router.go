package v1

import (
	"net/http"
	"smartdb/internal/auth"
	"smartdb/internal/domain"
	"smartdb/internal/handler"
)

func RouterMux(App *domain.App) *http.ServeMux {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", HealthHandler())

	// Project management — system key required
	mux.Handle(
		"POST /projects",
		auth.RequireSystemKey(App.SystemDB)(http.HandlerFunc(CreateProjectHandler(App))),
	)
	mux.Handle(
		"GET /projects",
		auth.RequireSystemKey(App.SystemDB)(http.HandlerFunc(GetProjectsHandler(App))),
	)
	mux.Handle(
		"DELETE /projects/{project}",
		auth.RequireSystemKey(App.SystemDB)(http.HandlerFunc(RemoveProjectHandler(App))),
	)

	// Project detail / update — project or system key
	mux.Handle(
		"GET /projects/{project}",
		auth.RequireProjectAccess(App.SystemDB)(http.HandlerFunc(GetProjectDetailHandler(App))),
	)
	mux.Handle(
		"PATCH /projects/{project}",
		auth.RequireProjectAccess(App.SystemDB)(http.HandlerFunc(PatchProjectHandler(App))),
	)

	// Project stats and schema
	mux.Handle(
		"GET /projects/{project}/stats",
		auth.RequireProjectAccess(App.SystemDB)(http.HandlerFunc(GetProjectStatsHandler(App))),
	)
	mux.Handle(
		"GET /projects/{project}/tables",
		auth.RequireProjectAccess(App.SystemDB)(http.HandlerFunc(GetProjectTablesHandler(App))),
	)
	mux.Handle(
		"GET /projects/{project}/tables/{table}",
		auth.RequireProjectAccess(App.SystemDB)(http.HandlerFunc(GetTableSchemaHandler(App))),
	)

	// SQL execution
	mux.Handle(
		"POST /projects/{project}/sql",
		auth.RequireProjectAccess(App.SystemDB)(http.HandlerFunc(ExecuteSQLHandler(App))),
	)

	// API key management
	mux.Handle(
		"POST /projects/{project}/apikeys",
		auth.RequireProjectAccess(App.SystemDB)(http.HandlerFunc(CreateAPIKeyHandler(App))),
	)
	mux.Handle(
		"GET /projects/{project}/apikeys",
		auth.RequireProjectAccess(App.SystemDB)(http.HandlerFunc(ListAPIKeysHandler(App))),
	)
	mux.Handle(
		"DELETE /projects/{project}/apikeys/{id}",
		auth.RequireProjectAccess(App.SystemDB)(http.HandlerFunc(RevokeAPIKeyHandler(App))),
	)

	// Backup and restore
	mux.Handle(
		"POST /projects/{project}/backup",
		auth.RequireProjectAccess(App.SystemDB)(http.HandlerFunc(BackupHandler(App))),
	)
	mux.Handle(
		"POST /projects/{project}/restore",
		auth.RequireProjectAccess(App.SystemDB)(http.HandlerFunc(RestoreHandler(App))),
	)

	// Apply CORS to all routes
	return withCORS(mux)
}

// withCORS wraps the mux with CORS middleware and returns a new mux.
func withCORS(next *http.ServeMux) *http.ServeMux {
	wrapped := http.NewServeMux()
	wrapped.Handle("/", handler.CORSMiddleware(next))
	return wrapped
}

func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"version": "1.0.0",
		})
	}
}
