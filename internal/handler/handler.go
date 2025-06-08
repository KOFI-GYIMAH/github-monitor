package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/KOFI-GYIMAH/github-monitor/internal/models"
	"github.com/KOFI-GYIMAH/github-monitor/internal/service"
	"github.com/KOFI-GYIMAH/github-monitor/internal/worker"
	"github.com/KOFI-GYIMAH/github-monitor/pkg/errors"
	"github.com/KOFI-GYIMAH/github-monitor/pkg/logger"
	"github.com/gorilla/mux"
)

type RepositoryHandler struct {
	service *service.RepositoryService
	ctx     context.Context
}

func NewRepositoryHandler(ctx context.Context, service *service.RepositoryService) *RepositoryHandler {
	return &RepositoryHandler{
		service: service,
		ctx:     ctx,
	}
}

func (h *RepositoryHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/repositories/{owner}/{repo}", h.getRepository).Methods("GET")
	r.HandleFunc("/repositories", h.AddRepository).Methods("POST")
	r.HandleFunc("/repositories/{owner}/{name}/commits", h.getCommits).Methods("GET")
	r.HandleFunc("/repositories/{owner}/{name}/top-authors", h.getTopCommitAuthors).Methods("GET")
	r.HandleFunc("/repositories/{owner}/{name}/reset-collection", h.resetCollection).Methods("POST")
	r.HandleFunc("/repositories/{owner}/{name}/monitor", h.monitorRepository).Methods("POST")
}

func writeSuccess(w http.ResponseWriter, data interface{}, message ...string) {
	resp := APIResponse{
		Status: "success",
		Data:   data,
	}
	if len(message) > 0 {
		resp.Message = message[0]
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// getRepository godoc
// @Summary Get Repository
// @Description Fetch repository metadata from DB
// @Tags Repository
// @Produce json
// @Param owner path string true "Repository Owner"
// @Param repo path string true "Repository Name"
// @Success 200 {object} models.Repository
// @Failure 500 {string} string "Internal Server Error"
// @Router /repositories/{owner}/{repo} [get]
func (h *RepositoryHandler) getRepository(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	owner := vars["owner"]
	repoName := vars["repo"]

	fullName := owner + "/" + repoName
	repository, err := h.service.GetRepository(r.Context(), fullName)
	if err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	logger.Info("Fetched repository %s", fullName)
	w.Header().Set("Content-Type", "application/json")
	writeSuccess(w, repository, "Successfully fetched repository")
}

// @Summary Add a repository to monitor
// @Description Adds a new GitHub repository to be monitored
// @Tags Repository
// @Accept json
// @Produce json
// @Param repository body AddRepositoryRequest true "Repository to Add"
// @Success 201 {object} map[string]string
// @Failure 400 {string} string "Invalid request"
// @Failure 409 {string} string "Repository already monitored"
// @Failure 500 {string} string "Failed to sync repository"
// @Router /repositories [post]
func (h *RepositoryHandler) AddRepository(w http.ResponseWriter, r *http.Request) {
	var req AddRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	repoName := fmt.Sprintf("%s/%s", req.Owner, req.Name)

	// * Check if repo already exists
	existingRepos, err := h.service.ListAllRepositories(ctx)
	if err != nil {
		errors.WriteHTTPError(w, err)
		return
	}
	for _, r := range existingRepos {
		if r.Name == repoName {
			logger.Info("Repository %s already exists", repoName)
			http.Error(w, "Repository already monitored", http.StatusConflict)
			return
		}
	}

	// * Sync the new repo
	if err := h.service.SyncRepository(ctx, req.Owner, req.Name, time.Time{}); err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	// * Start monitoring
	go func() {
		syncInterval, _ := time.ParseDuration(os.Getenv("SYNC_INTERVAL"))
		w := worker.NewSyncWorker(h.service, syncInterval, req.Owner, req.Name)
		w.Run(h.ctx)
	}()

	w.WriteHeader(http.StatusCreated)
	writeSuccess(w, map[string]string{
		"message": "Repository successfully added and monitoring started.",
	}, "Repository added")
}

// getCommits godoc
// @Summary Get Commits
// @Description List commits for a repository (supports filtering & pagination)
// @Tags Commits
// @Produce json
// @Param owner path string true "Repository Owner"
// @Param name path string true "Repository Name"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Number of items per page" default(30)
// @Param since query string false "Start date (RFC3339)"
// @Param until query string false "End date (RFC3339)"
// @Success 200 {array} models.Commit
// @Failure 500 {string} string "Internal Server Error"
// @Router /repositories/{owner}/{name}/commits [get]
func (h *RepositoryHandler) getCommits(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	owner := vars["owner"]
	repoName := vars["name"]

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 30
	}

	var since, until *time.Time
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = &t
		}
	}
	if u := r.URL.Query().Get("until"); u != "" {
		if t, err := time.Parse(time.RFC3339, u); err == nil {
			until = &t
		}
	}

	fullName := owner + "/" + repoName
	commits, err := h.service.GetCommits(r.Context(), fullName, since, until)
	if err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	start := (page - 1) * limit
	if start > len(commits) {
		json.NewEncoder(w).Encode([]any{})
		return
	}
	end := min(start+limit, len(commits))

	logger.Info("Fetched %d commits for %s", end-start, fullName)
	writeSuccess(w, commits[start:end], "Successfully fetched commits")
}

// getTopCommitAuthors godoc
// @Summary Get Top Authors
// @Description Fetch top commit authors by number of commits
// @Tags Analytics
// @Produce json
// @Param owner path string true "Repository Owner"
// @Param name path string true "Repository Name"
// @Param limit query int false "Max authors to return" default(10)
// @Success 200 {array} models.AuthorCommitCount
// @Failure 500 {string} string "Internal Server Error"
// @Router /repositories/{owner}/{name}/top-authors [get]
func (h *RepositoryHandler) getTopCommitAuthors(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	owner := vars["owner"]
	repoName := vars["name"]

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 10
	}

	fullName := owner + "/" + repoName
	authors, err := h.service.GetTopAuthors(r.Context(), fullName, limit)
	if err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	if authors == nil {
		authors = []models.AuthorCommitCount{}
	}

	logger.Info("Fetched top authors for %s", fullName)
	w.Header().Set("Content-Type", "application/json")
	writeSuccess(w, authors, "Successfully fetched top authors")
}

// resetCollection godoc
// @Summary Reset Repository Data
// @Description Deletes and reloads repo data from GitHub starting from a given date
// @Tags Repository
// @Accept json
// @Produce json
// @Param owner path string true "Repository Owner"
// @Param name path string true "Repository Name"
// @Param request body models.DateRequest true "Start date"
// @Success 200 {object} map[string]string
// @Failure 400 {string} string "Bad Request"
// @Failure 500 {string} string "Internal Server Error"
// @Router /repositories/{owner}/{name}/reset-collection [post]
func (h *RepositoryHandler) resetCollection(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	owner := vars["owner"]
	repoName := vars["name"]

	var request struct {
		Since time.Time `json:"since"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	fullName := owner + "/" + repoName
	if err := h.service.ResetRepository(r.Context(), fullName, request.Since); err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	logger.Info("Reset repository %s data since %s", fullName, request.Since.Format(time.RFC3339))
	w.WriteHeader(http.StatusOK)
	writeSuccess(w, map[string]string{
		"message": "Repository data reset successfully",
		"since":   request.Since.Format(time.RFC3339),
	}, "Repository data reset successfully")
}

// monitorRepository godoc
// @Summary Monitor Repository
// @Description Starts monitoring repository for new data since a given date
// @Tags Repository
// @Accept json
// @Produce json
// @Param owner path string true "Repository Owner"
// @Param name path string true "Repository Name"
// @Param request body models.DateRequest true "Start date"
// @Success 200 {object} map[string]string
// @Failure 400 {string} string "Bad Request"
// @Failure 500 {string} string "Internal Server Error"
// @Router /repositories/{owner}/{name}/monitor [post]
func (h *RepositoryHandler) monitorRepository(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	owner := vars["owner"]
	repoName := vars["name"]

	var request struct {
		Since time.Time `json:"since"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	if err := h.service.SyncRepository(r.Context(), owner, repoName, request.Since); err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	logger.Info("Started monitoring repository %s since %s", owner+"/"+repoName, request.Since.Format(time.RFC3339))
	w.WriteHeader(http.StatusOK)
	writeSuccess(w, map[string]string{
		"message": "Repository monitoring started successfully",
		"since":   request.Since.Format(time.RFC3339),
	}, "Repository monitoring started successfully")
}
