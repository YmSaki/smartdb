package api

import (
	"net/http"
	v1 "smartdb/internal/api/v1"
	"smartdb/internal/domain"
)

func RouterMux(App *domain.App) *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle(
		"/v1/",
		http.StripPrefix("/v1", v1.RouterMux(App)),
	)

	return mux
}
