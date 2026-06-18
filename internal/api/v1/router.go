package v1

import (
	"net/http"
	"smartdb/internal/domain"
)

func RouterMux(App *domain.App) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc(
		"POST /projects",
		CreateProjectHandler(App),
	)
	mux.HandleFunc(
		"GET /projects",
		GetProjectsHandler(App),
	)

	mux.HandleFunc(
		"GET /projects/{project}",
		GetProjectDetailHandler(App),
	)
	mux.HandleFunc(
		"DELETE /projects/{project}",
		RemoveProjectHandler(App),
	)

	mux.HandleFunc(
		"POST /projects/{project}/query",
		QueryExecuteHandler(App),
	)

	return mux
}
