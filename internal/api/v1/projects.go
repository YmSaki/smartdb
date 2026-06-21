package v1

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"smartdb/internal/domain"
	"smartdb/internal/handler"
	"smartdb/internal/project"
)

type CreateProjectRequest struct {
	Name string `json:"name"`
}

type CreateProjectResponse struct {
	ProjectID string `json:"projectID"`
}

func CreateProjectHandler(App *domain.App) http.HandlerFunc {
	return handler.HandleJson(func(w http.ResponseWriter, r *http.Request, req CreateProjectRequest) {
		if req.Name == "" {
			http.Error(
				w,
				"name is required",
				http.StatusBadRequest,
			)
			return
		}

		projectID, err := project.Create(req.Name, App.SystemDB)

		if err != nil {
			slog.Error(err.Error())
			http.Error(
				w,
				"Project creation failed.",
				http.StatusInternalServerError,
			)
			return
		}

		w.Header().Set(
			"Content-Type",
			"application/json",
		)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(
			CreateProjectResponse{
				ProjectID: projectID,
			},
		)
	})
}

func GetProjectsHandler(App *domain.App) http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		filter := project.ProjectFilter{}
		filter.State = []domain.ProjectState{domain.StateInactive, domain.StateActive}
		list, err := project.GetProjectList(App.SystemDB, filter)
		if err != nil {
			http.Error(
				w,
				err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
		jsonData, err := json.Marshal(list)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set(
			"Content-Type",
			"application/json",
		)
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write(jsonData)
	}
}

func GetProjectDetailHandler(App *domain.App) http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		projectID := r.PathValue("project")
		if projectID == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		projectData, err := project.GetProject(App.SystemDB, projectID)
		if err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				w.WriteHeader(http.StatusNotFound)
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set(
			"Content-Type",
			"application/json",
		)

		jsonData, err := json.Marshal(projectData)
		if err != nil {
			slog.Warn("json convert error", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jsonData)
	}
}

// RemoveProjectHandler 実はstateをdeletedに変えるだけという。wipeの時まで実は実態が残る。
func RemoveProjectHandler(App *domain.App) http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		projectID := r.PathValue("project")
		err := project.UpdateProjectState(App.SystemDB, projectID, domain.StateDeleted)
		if err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				slog.Warn("A non-existent project name was specified.")
				w.WriteHeader(http.StatusNotFound)

			default:
				slog.Error("Project Remove Error", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type ExecuteSQLRequest struct {
	Token string `json:"token"`
	SQL   string `json:"sql"`
}

type ExecuteSQLResponse struct {
	IsSuccess bool `json:"success"`
	Result    struct {
		Rows         []map[string]any `json:"rows"`
		AffectedRows int64            `json:"affectedRows"`
	} `json:"result"`
}

func ExecuteSQLHandler(App *domain.App) http.HandlerFunc {
	return handler.HandleJson(func(w http.ResponseWriter, r *http.Request, req ExecuteSQLRequest) {
		projectId := r.PathValue("project")

		queryType, err := project.QueryJudge(req.SQL)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx := context.Background()
		var responseData = ExecuteSQLResponse{IsSuccess: true}

		switch queryType {
		case project.SQLTypeRead:
			qMap, err := project.Query(ctx, App.SystemDB, projectId, req.SQL)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				responseData.IsSuccess = false
			} else {
				responseData.Result.Rows = qMap
			}
		default:
			aRaws, err := project.Execute(ctx, App.SystemDB, projectId, req.SQL)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				responseData.IsSuccess = false
			} else {
				responseData.Result.AffectedRows = aRaws
			}
		}

		w.Header().Set(
			"Content-Type",
			"application/json",
		)

		jsonData, err := json.Marshal(responseData)
		if err != nil {
			slog.Warn("json convert error", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if responseData.IsSuccess {
			w.WriteHeader(http.StatusOK)
		}

		_, _ = w.Write(jsonData)
	})
}
