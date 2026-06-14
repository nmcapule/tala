package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"tala/internal/app"
	"tala/internal/domain"
	"tala/internal/mcp"
)

type Server struct {
	service *app.Service
	static  http.Handler
}

func New(service *app.Service, static http.Handler) *Server {
	return &Server{service: service, static: static}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Route("/api", func(r chi.Router) {
		r.Get("/issues", s.listIssues)
		r.Post("/issues", s.createIssue)
		r.Post("/uploads/images", s.uploadImage)
		r.Get("/issues/{id}", s.getIssue)
		r.Patch("/issues/{id}", s.updateIssue)
		r.Get("/issues/{id}/comments", s.listComments)
		r.Post("/issues/{id}/comments", s.addComment)
		r.Put("/issues/{id}/parent", s.setParent)
		r.Post("/issues/{id}/blockers", s.addBlocker)
		r.Delete("/issues/{id}/blockers/{blockerID}", s.removeBlocker)
		r.Get("/tags", s.listTags)
		r.Post("/tags", s.createTag)
		r.Patch("/tags/{id}", s.updateTag)
	})
	r.Get("/uploads/images/{filename}", s.serveUploadedImage)
	mcpServer := mcp.New(s.service)
	r.Handle("/mcp", mcpServer)
	r.NotFound(s.notFound)
	r.MethodNotAllowed(s.methodNotAllowed)
	return r
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	if isAPIPath(r.URL.Path) {
		writeError(w, domain.NewError(domain.CodeNotFound, "Endpoint not found.", "path"))
		return
	}
	if s.static != nil {
		s.static.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	if isAPIPath(r.URL.Path) {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"error": domain.NewError(domain.CodeValidationError, "Method not allowed.", "method"),
		})
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func (s *Server) listIssues(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	issues, err := s.service.SearchIssues(r.Context(), domain.IssueFilters{
		Status:    q.Get("status"),
		Priority:  q.Get("priority"),
		Assignee:  q.Get("assignee"),
		Tag:       q.Get("tag"),
		ID:        q.Get("id"),
		ParentID:  q.Get("parent_id"),
		BlockedBy: q.Get("blocked_by"),
		BlockerOf: q.Get("blocker_of"),
		State:     q.Get("state"),
		Query:     q.Get("q"),
		Sort:      q.Get("sort"),
		Order:     q.Get("order"),
	})
	respond(w, issues, err, http.StatusOK)
}

func (s *Server) createIssue(w http.ResponseWriter, r *http.Request) {
	if !requireUsername(w, r) {
		return
	}
	var req app.CreateIssueRequest
	if !decode(w, r, &req) {
		return
	}
	issue, err := s.service.CreateIssue(r.Context(), r.Header.Get("X-Tala-Username"), req)
	respond(w, issue, err, http.StatusCreated)
}

func (s *Server) getIssue(w http.ResponseWriter, r *http.Request) {
	issue, err := s.service.GetIssue(r.Context(), chi.URLParam(r, "id"))
	respond(w, issue, err, http.StatusOK)
}

func (s *Server) updateIssue(w http.ResponseWriter, r *http.Request) {
	if !requireUsername(w, r) {
		return
	}
	var req app.UpdateIssueRequest
	if !decode(w, r, &req) {
		return
	}
	issue, err := s.service.UpdateIssue(r.Context(), chi.URLParam(r, "id"), req)
	respond(w, issue, err, http.StatusOK)
}

func (s *Server) addComment(w http.ResponseWriter, r *http.Request) {
	if !requireUsername(w, r) {
		return
	}
	var req app.CommentRequest
	if !decode(w, r, &req) {
		return
	}
	comment, err := s.service.AddComment(r.Context(), chi.URLParam(r, "id"), r.Header.Get("X-Tala-Username"), req)
	respond(w, comment, err, http.StatusCreated)
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	comments, err := s.service.ListComments(r.Context(), chi.URLParam(r, "id"))
	respond(w, comments, err, http.StatusOK)
}

func (s *Server) uploadImage(w http.ResponseWriter, r *http.Request) {
	if !requireUsername(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, app.MaxImageUploadBytes+1024)
	if err := r.ParseMultipartForm(app.MaxImageUploadBytes + 1024); err != nil {
		writeError(w, domain.NewError(domain.CodeValidationError, "Invalid image upload.", "image"))
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, domain.NewError(domain.CodeValidationError, "Image file is required.", "image"))
		return
	}
	defer file.Close()
	name := ""
	if header != nil {
		name = header.Filename
	}
	uploaded, err := s.service.UploadImage(r.Context(), r.Header.Get("X-Tala-Username"), name, file)
	respond(w, uploaded, err, http.StatusCreated)
}

func (s *Server) serveUploadedImage(w http.ResponseWriter, r *http.Request) {
	image, err := s.service.OpenUploadedImage(chi.URLParam(r, "filename"))
	if err != nil {
		writeError(w, err)
		return
	}
	defer image.File.Close()
	w.Header().Set("Content-Type", image.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Length", strconv.FormatInt(image.Size, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, image.File)
}

func (s *Server) setParent(w http.ResponseWriter, r *http.Request) {
	if !requireUsername(w, r) {
		return
	}
	body, err := readJSONBody(w, r)
	if err != nil {
		return
	}
	fields, ok := decodeObjectFields(w, body)
	if !ok {
		return
	}
	rawParentID, ok := fields["parent_issue_id"]
	if !ok {
		writeError(w, domain.NewError(domain.CodeValidationError, "Parent issue ID is required.", "parent_issue_id"))
		return
	}
	var parentID *string
	if string(rawParentID) != "null" {
		var parsed string
		if err := json.Unmarshal(rawParentID, &parsed); err != nil {
			writeError(w, domain.NewError(domain.CodeValidationError, "Invalid parent issue ID.", "parent_issue_id"))
			return
		}
		parentID = &parsed
	}
	issue, err := s.service.SetParent(r.Context(), chi.URLParam(r, "id"), parentID)
	respond(w, issue, err, http.StatusOK)
}

func (s *Server) addBlocker(w http.ResponseWriter, r *http.Request) {
	if !requireUsername(w, r) {
		return
	}
	body, err := readJSONBody(w, r)
	if err != nil {
		return
	}
	fields, ok := decodeObjectFields(w, body)
	if !ok {
		return
	}
	rawBlockerID, ok := fields["blocker_issue_id"]
	if !ok {
		writeError(w, domain.NewError(domain.CodeValidationError, "Blocker issue ID is required.", "blocker_issue_id"))
		return
	}
	if string(rawBlockerID) == "null" {
		writeError(w, domain.NewError(domain.CodeValidationError, "Blocker issue ID must not be null.", "blocker_issue_id"))
		return
	}
	var blockerIssueID string
	if err := json.Unmarshal(rawBlockerID, &blockerIssueID); err != nil {
		writeError(w, domain.NewError(domain.CodeValidationError, "Invalid blocker issue ID.", "blocker_issue_id"))
		return
	}
	err = s.service.AddBlocker(r.Context(), chi.URLParam(r, "id"), blockerIssueID)
	respond(w, map[string]string{"status": "ok"}, err, http.StatusOK)
}

func (s *Server) removeBlocker(w http.ResponseWriter, r *http.Request) {
	if !requireUsername(w, r) {
		return
	}
	err := s.service.RemoveBlocker(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "blockerID"))
	respond(w, map[string]string{"status": "ok"}, err, http.StatusOK)
}

func (s *Server) listTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.service.ListTags(r.Context())
	respond(w, tags, err, http.StatusOK)
}

func (s *Server) createTag(w http.ResponseWriter, r *http.Request) {
	if !requireUsername(w, r) {
		return
	}
	body, err := readJSONBody(w, r)
	if err != nil {
		return
	}
	fields, ok := decodeObjectFields(w, body)
	if !ok {
		return
	}
	var name string
	if rawName, ok := fields["name"]; ok {
		if string(rawName) == "null" {
			writeError(w, domain.NewError(domain.CodeValidationError, "Tag name must not be null.", "name"))
			return
		}
		if err := json.Unmarshal(rawName, &name); err != nil {
			writeError(w, domain.NewError(domain.CodeValidationError, "Invalid tag name.", "name"))
			return
		}
	}
	var color *string
	if rawColor, ok := fields["color"]; ok && string(rawColor) != "null" {
		var parsed string
		if err := json.Unmarshal(rawColor, &parsed); err != nil {
			writeError(w, domain.NewError(domain.CodeValidationError, "Invalid tag color.", "color"))
			return
		}
		color = &parsed
	}
	tag, err := s.service.CreateTag(r.Context(), name, color)
	respond(w, tag, err, http.StatusCreated)
}

func (s *Server) updateTag(w http.ResponseWriter, r *http.Request) {
	if !requireUsername(w, r) {
		return
	}
	body, err := readJSONBody(w, r)
	if err != nil {
		return
	}
	fields, ok := decodeObjectFields(w, body)
	if !ok {
		return
	}
	var name *string
	if rawName, ok := fields["name"]; ok {
		if string(rawName) == "null" {
			writeError(w, domain.NewError(domain.CodeValidationError, "Tag name must not be null.", "name"))
			return
		}
		var parsed string
		if err := json.Unmarshal(rawName, &parsed); err != nil {
			writeError(w, domain.NewError(domain.CodeValidationError, "Invalid tag name.", "name"))
			return
		}
		name = &parsed
	}
	var color **string
	if rawColor, ok := fields["color"]; ok {
		var value *string
		if string(rawColor) != "null" {
			var parsed string
			if err := json.Unmarshal(rawColor, &parsed); err != nil {
				writeError(w, domain.NewError(domain.CodeValidationError, "Invalid tag color.", "color"))
				return
			}
			value = &parsed
		}
		color = &value
	}
	tag, err := s.service.UpdateTag(r.Context(), chi.URLParam(r, "id"), name, color)
	respond(w, tag, err, http.StatusOK)
}

func readJSONBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		writeError(w, domain.NewError(domain.CodeValidationError, "Invalid request body.", "body"))
		return nil, err
	}
	return body, nil
}

func decodeObjectFields(w http.ResponseWriter, body []byte) (map[string]json.RawMessage, bool) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		writeError(w, domain.NewError(domain.CodeValidationError, "Invalid JSON request body.", "body"))
		return nil, false
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, domain.NewError(domain.CodeValidationError, "Invalid JSON request body.", "body"))
		return nil, false
	}
	return fields, true
}

func decode(w http.ResponseWriter, r *http.Request, dest any) bool {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, domain.NewError(domain.CodeValidationError, "Invalid request body.", "body"))
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		writeError(w, domain.NewError(domain.CodeValidationError, "Invalid JSON request body.", "body"))
		return false
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, domain.NewError(domain.CodeValidationError, "Invalid JSON request body.", "body"))
		return false
	}
	if !isJSONObject(raw) {
		writeError(w, domain.NewError(domain.CodeValidationError, "Invalid JSON request body.", "body"))
		return false
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			writeError(w, appErr)
			return false
		}
		writeError(w, domain.NewError(domain.CodeValidationError, "Invalid JSON request body.", "body"))
		return false
	}
	return true
}

func isJSONObject(raw json.RawMessage) bool {
	return strings.HasPrefix(strings.TrimSpace(string(raw)), "{")
}

func requireUsername(w http.ResponseWriter, r *http.Request) bool {
	if strings.TrimSpace(r.Header.Get("X-Tala-Username")) == "" {
		writeError(w, domain.NewError(domain.CodeMissingUsername, "Username is required for this operation.", "username"))
		return false
	}
	return true
}

func respond(w http.ResponseWriter, data any, err error, okStatus int) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, okStatus, data)
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		appErr = domain.NewError(domain.CodeInternal, "Internal server error.", "")
	}
	status := http.StatusBadRequest
	switch appErr.Code {
	case domain.CodeNotFound:
		status = http.StatusNotFound
	case domain.CodeMissingUsername:
		status = http.StatusUnauthorized
	case domain.CodeConflict, domain.CodeCycleDetected:
		status = http.StatusConflict
	case domain.CodeInternal:
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]any{"error": appErr})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
