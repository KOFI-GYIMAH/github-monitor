package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/KOFI-GYIMAH/github-monitor/internal/models"
	"github.com/KOFI-GYIMAH/github-monitor/internal/service"
	"github.com/gorilla/mux"
)

type RepositoryHandler struct {
	service *service.RepositoryService
}

func NewRepositoryHandler(service *service.RepositoryService) *RepositoryHandler {
	return &RepositoryHandler{
		service: service,
	}
}

func (h *RepositoryHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/repositories/{owner}/{repo}", h.getRepository).Methods("GET")
	r.HandleFunc("/repositories/{owner}/{name}/commits", h.getCommits).Methods("GET")
	r.HandleFunc("/repositories/{owner}/{name}/top-authors", h.getTopCommitAuthors).Methods("GET")
	r.HandleFunc("/repositories/{owner}/{name}/reset-collection", h.resetCollection).Methods("POST")
	r.HandleFunc("/repositories/{owner}/{name}/monitor", h.monitorRepository).Methods("POST")
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repository)
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

	// Parse pagination params
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 30
	}

	// Parse date filters
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Implement pagination
	start := (page - 1) * limit
	if start > len(commits) {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	end := start + limit
	if end > len(commits) {
		end = len(commits)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(commits[start:end])
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

	// Parse limit parameter
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 10
	}

	fullName := owner + "/" + repoName
	authors, err := h.service.GetTopAuthors(r.Context(), fullName, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if authors == nil {
		authors = []models.AuthorCommitCount{} // Return empty array instead of null
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authors)
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

	// Parse request body
	var request struct {
		Since time.Time `json:"since"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	fullName := owner + "/" + repoName
	if err := h.service.ResetRepository(r.Context(), fullName, request.Since); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Repository data reset successfully",
		"since":   request.Since.Format(time.RFC3339),
	})
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

	// Parse request body
	var request struct {
		Since time.Time `json:"since"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.SyncRepository(r.Context(), owner, repoName, request.Since); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Repository monitoring started successfully",
		"since":   request.Since.Format(time.RFC3339),
	})
}
