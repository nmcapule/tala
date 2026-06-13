package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"tala/internal/app"
	"tala/internal/domain"
)

type Server struct {
	service *app.Service
}

func New(service *app.Service) *Server {
	return &Server{service: service}
}

type request struct {
	JSONRPC   string          `json:"jsonrpc"`
	ID        any             `json:"id,omitempty"`
	HasID     bool            `json:"-"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params,omitempty"`
	HasResult bool            `json:"-"`
	HasError  bool            `json:"-"`
}

func (r *request) UnmarshalJSON(data []byte) error {
	type alias request
	var raw struct {
		*alias
		ID     json.RawMessage `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	raw.alias = (*alias)(r)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.HasResult = raw.Result != nil
	r.HasError = raw.Error != nil
	if raw.ID != nil {
		r.HasID = true
		if string(raw.ID) == "null" {
			r.ID = nil
		} else {
			id, err := decodeJSONValue(raw.ID)
			if err != nil {
				return err
			}
			r.ID = id
		}
	}
	return nil
}

func decodeJSONValue(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

type response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !allowedOrigin(r.Header.Get("Origin")) {
		writeTransportError(w, http.StatusForbidden, -32000, "Forbidden origin")
		return
	}
	w.Header().Set("MCP-Protocol-Version", "2025-06-18")
	if !validProtocolVersion(r.Header.Get("MCP-Protocol-Version")) {
		writeTransportError(w, http.StatusBadRequest, -32000, "Unsupported MCP protocol version")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeTransportError(w, http.StatusMethodNotAllowed, -32000, "MCP endpoint only supports POST")
		return
	}
	if !validPostAccept(r.Header.Values("Accept")) {
		writeTransportError(w, http.StatusNotAcceptable, -32000, "MCP POST requires Accept: application/json, text/event-stream")
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "Parse error"}})
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		writeJSON(w, response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "Parse error"}})
		return
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		writeJSON(w, response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "Parse error"}})
		return
	}
	if !isJSONObject(raw) {
		writeJSON(w, response{JSONRPC: "2.0", Error: &rpcError{Code: -32600, Message: "Invalid Request"}})
		return
	}
	var req request
	if err := json.Unmarshal(raw, &req); err != nil {
		writeJSON(w, response{JSONRPC: "2.0", Error: &rpcError{Code: -32600, Message: "Invalid Request"}})
		return
	}
	if isJSONRPCResponse(req) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if rpcErr := validateRequest(req); rpcErr != nil {
		writeJSON(w, response{JSONRPC: "2.0", Error: rpcErr})
		return
	}
	if !req.HasID {
		if isJSONRPCNotification(req) {
			_, _ = s.handle(r, req)
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	result, rpcErr := s.handle(r, req)
	writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rpcErr})
}

func isJSONRPCResponse(req request) bool {
	return req.JSONRPC == "2.0" &&
		req.HasID &&
		validRequestID(req.ID) &&
		strings.TrimSpace(req.Method) == "" &&
		((req.HasResult && !req.HasError) || (req.HasError && !req.HasResult))
}

func isJSONRPCNotification(req request) bool {
	return strings.HasPrefix(strings.TrimSpace(req.Method), "notifications/")
}

func isJSONObject(raw json.RawMessage) bool {
	return strings.HasPrefix(strings.TrimSpace(string(raw)), "{")
}

func (s *Server) handle(r *http.Request, req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return capabilities(), nil
	case "notifications/initialized":
		return nil, nil
	case "tools/list":
		return map[string]any{"tools": tools()}, nil
	case "resources/list":
		return map[string]any{"resources": resources()}, nil
	case "resources/templates/list":
		return map[string]any{"resourceTemplates": resourceTemplates()}, nil
	case "tools/call":
		fields, rpcErr := objectParams(req.Params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		name, rpcErr := requiredStringParam(fields, "name")
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.callTool(r, name, fields["arguments"])
	case "resources/read":
		fields, rpcErr := objectParams(req.Params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		uri, rpcErr := requiredStringParam(fields, "uri")
		if rpcErr != nil {
			return nil, rpcErr
		}
		return s.readResource(r, uri)
	default:
		return nil, &rpcError{Code: -32601, Message: "Method not found"}
	}
}

func validateRequest(req request) *rpcError {
	if req.JSONRPC != "2.0" || strings.TrimSpace(req.Method) == "" || req.HasResult || req.HasError || (req.HasID && !validRequestID(req.ID)) {
		return &rpcError{Code: -32600, Message: "Invalid Request"}
	}
	return nil
}

func validRequestID(id any) bool {
	switch value := id.(type) {
	case nil, string:
		return true
	case json.Number:
		return validIntegerJSONNumber(value.String())
	default:
		return false
	}
}

func validIntegerJSONNumber(value string) bool {
	if value == "" {
		return false
	}
	if value[0] == '-' {
		value = value[1:]
		if value == "" {
			return false
		}
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (s *Server) callTool(r *http.Request, name string, raw json.RawMessage) (any, *rpcError) {
	ctx := r.Context()
	var rpcErr *rpcError
	raw, rpcErr = normalizeToolArguments(raw)
	if rpcErr != nil {
		return nil, rpcErr
	}
	switch name {
	case "issue_create":
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, invalidParams(err)
		}
		username, appErr := usernameArgument(fields)
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := rejectNullArguments(fields, "title", "description_markdown", "priority"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		title, appErr := optionalStringArgument(fields, "title", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		descriptionMarkdown, appErr := optionalStringArgument(fields, "description_markdown", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		priority, appErr := optionalStringArgument(fields, "priority", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		assignee, appErr := nullableStringArgument(raw, "assignee", "Argument must be a string or null.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		parentIssueID, appErr := nullableStringArgument(raw, "parent_issue_id", "Argument must be a string or null.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		tagNames, _, appErr := optionalStringSliceArgument(fields, "tag_names")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		issue, err := s.service.CreateIssue(ctx, username, app.CreateIssueRequest{
			Title: title, DescriptionMarkdown: descriptionMarkdown, Priority: priority,
			Assignee: assignee, TagNames: tagNames, ParentIssueID: parentIssueID,
		})
		return toolResult("Created issue "+issue.ID+".", issue, err)
	case "issue_update":
		var assignee **string
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, invalidParams(err)
		}
		_, appErr := usernameArgument(fields)
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := rejectNullArguments(fields, "title", "description_markdown", "status", "priority"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		issueID, appErr := requiredStringArgument(fields, "issue_id", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		title, appErr := optionalStringPointerArgument(fields, "title", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		descriptionMarkdown, appErr := optionalStringPointerArgument(fields, "description_markdown", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		status, appErr := optionalStringPointerArgument(fields, "status", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		priority, appErr := optionalStringPointerArgument(fields, "priority", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		if _, ok := fields["assignee"]; ok {
			value, appErr := nullableStringArgument(raw, "assignee", "Argument must be a string or null.")
			if appErr != nil {
				return toolResult("", nil, appErr)
			}
			assignee = &value
		}
		tagNames, tagNamesSet, appErr := optionalStringSliceArgument(fields, "tag_names")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		issue, err := s.service.UpdateIssue(ctx, issueID, app.UpdateIssueRequest{
			Title:               title,
			DescriptionMarkdown: descriptionMarkdown,
			Status:              status,
			Priority:            priority,
			Assignee:            assignee,
			TagNames:            tagNames,
			TagNamesSet:         tagNamesSet,
		})
		return toolResult("Updated issue "+issueID+".", issue, err)
	case "issue_search":
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, invalidParams(err)
		}
		filters, appErr := issueFilterArguments(fields)
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		issues, err := s.service.SearchIssues(ctx, filters)
		return toolResult("Found issues.", issues, err)
	case "issue_get":
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, invalidParams(err)
		}
		issueID, appErr := requiredStringArgument(fields, "issue_id", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		issue, err := s.service.GetIssue(ctx, issueID)
		return toolResult("Fetched issue "+issueID+".", issue, err)
	case "issue_comment":
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, invalidParams(err)
		}
		username, appErr := usernameArgument(fields)
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := requireArgument(raw, "body_markdown"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := rejectNullArguments(fields, "body_markdown"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		issueID, appErr := requiredStringArgument(fields, "issue_id", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		bodyMarkdown, appErr := stringArgument(fields, "body_markdown", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		comment, err := s.service.AddComment(ctx, issueID, username, app.CommentRequest{BodyMarkdown: bodyMarkdown})
		return toolResult("Added comment.", comment, err)
	case "issue_set_parent":
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, invalidParams(err)
		}
		_, appErr := usernameArgument(fields)
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := requireArgument(raw, "parent_issue_id"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		issueID, appErr := requiredStringArgument(fields, "issue_id", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		parentIssueID, appErr := nullableStringArgument(raw, "parent_issue_id", "Argument must be a string or null.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		issue, err := s.service.SetParent(ctx, issueID, parentIssueID)
		return toolResult("Updated parent.", issue, err)
	case "issue_add_blocker":
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, invalidParams(err)
		}
		_, appErr := usernameArgument(fields)
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := requireArgument(raw, "blocker_issue_id"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := rejectNullArguments(fields, "blocker_issue_id"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		issueID, appErr := requiredStringArgument(fields, "issue_id", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		blockerIssueID, appErr := stringArgument(fields, "blocker_issue_id", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		err := s.service.AddBlocker(ctx, issueID, blockerIssueID)
		return toolResult("Added blocker.", map[string]string{"status": "ok"}, err)
	case "issue_remove_blocker":
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, invalidParams(err)
		}
		_, appErr := usernameArgument(fields)
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := requireArgument(raw, "blocker_issue_id"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := rejectNullArguments(fields, "blocker_issue_id"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		issueID, appErr := requiredStringArgument(fields, "issue_id", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		blockerIssueID, appErr := stringArgument(fields, "blocker_issue_id", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		err := s.service.RemoveBlocker(ctx, issueID, blockerIssueID)
		return toolResult("Removed blocker.", map[string]string{"status": "ok"}, err)
	case "issue_assign":
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, invalidParams(err)
		}
		_, appErr := usernameArgument(fields)
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := requireArgument(raw, "assignee"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		issueID, appErr := requiredStringArgument(fields, "issue_id", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		assignee, appErr := nullableStringArgument(raw, "assignee", "Argument must be a string or null.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		issue, err := s.service.AssignIssue(ctx, issueID, assignee)
		return toolResult("Updated assignee.", issue, err)
	case "issue_set_status":
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, invalidParams(err)
		}
		_, appErr := usernameArgument(fields)
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := requireArgument(raw, "status"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := rejectNullArguments(fields, "status"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		issueID, appErr := requiredStringArgument(fields, "issue_id", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		status, appErr := stringArgument(fields, "status", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		issue, err := s.service.SetStatus(ctx, issueID, domain.Status(status))
		return toolResult("Updated status.", issue, err)
	case "issue_set_priority":
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, invalidParams(err)
		}
		_, appErr := usernameArgument(fields)
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := requireArgument(raw, "priority"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		if appErr := rejectNullArguments(fields, "priority"); appErr != nil {
			return toolResult("", nil, appErr)
		}
		issueID, appErr := requiredStringArgument(fields, "issue_id", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		priority, appErr := stringArgument(fields, "priority", "Argument must be a string.")
		if appErr != nil {
			return toolResult("", nil, appErr)
		}
		issue, err := s.service.SetPriority(ctx, issueID, domain.Priority(priority))
		return toolResult("Updated priority.", issue, err)
	default:
		return nil, &rpcError{Code: -32602, Message: "Unknown tool"}
	}
}

func normalizeToolArguments(raw json.RawMessage) (json.RawMessage, *rpcError) {
	clean := strings.TrimSpace(string(raw))
	if clean == "" {
		return json.RawMessage(`{}`), nil
	}
	if clean == "null" || !strings.HasPrefix(clean, "{") {
		return nil, &rpcError{Code: -32602, Message: "Invalid params", Data: "tool arguments must be an object"}
	}
	return raw, nil
}

func (s *Server) readResource(r *http.Request, uri string) (any, *rpcError) {
	ctx := r.Context()
	switch {
	case uri == "tala://board":
		issues, err := s.service.SearchIssues(ctx, domain.IssueFilters{})
		if err != nil {
			return nil, appToRPC(err)
		}
		grouped := emptyBoard()
		for _, issue := range issues {
			grouped[string(issue.Status)] = append(grouped[string(issue.Status)], compactIssue(issue))
		}
		return resourceResult(uri, grouped), nil
	case uri == "tala://planning":
		issues, err := s.service.SearchIssues(ctx, domain.IssueFilters{})
		if err != nil {
			return nil, appToRPC(err)
		}
		details, err := s.issueDetails(ctx, issues)
		if err != nil {
			return nil, appToRPC(err)
		}
		planning := map[string]any{
			"issues": compactIssues(issues),
			"roots": compactIssues(filterIssues(issues, func(issue domain.Issue) bool {
				return issue.ParentIssueID == nil
			})),
			"children_by_parent": childrenByParent(issues),
			"blocked":            blockedContexts(details),
			"blocking":           blockingContexts(details),
		}
		return resourceResult(uri, planning), nil
	default:
		const issuePrefix = "tala://issues/"
		if len(uri) > len(issuePrefix) && uri[:len(issuePrefix)] == issuePrefix {
			id := uri[len(issuePrefix):]
			resourceKind := "detail"
			for _, suffix := range []string{"/tree", "/blockers"} {
				if len(id) > len(suffix) && id[len(id)-len(suffix):] == suffix {
					id = id[:len(id)-len(suffix)]
					resourceKind = strings.TrimPrefix(suffix, "/")
				}
			}
			issue, err := s.service.GetIssue(ctx, id)
			if err != nil {
				return nil, resourceReadError(uri, err)
			}
			switch resourceKind {
			case "tree":
				tree := map[string]any{
					"issue":    compactIssue(issue),
					"parent":   nil,
					"siblings": []domain.Issue{},
					"children": compactIssues(issue.Children),
				}
				if issue.ParentIssueID != nil {
					parent, err := s.service.GetIssue(ctx, *issue.ParentIssueID)
					if err != nil {
						return nil, resourceReadError(uri, err)
					}
					tree["parent"] = compactIssue(parent)
					tree["siblings"] = compactIssues(filterIssues(parent.Children, func(candidate domain.Issue) bool {
						return candidate.ID != issue.ID
					}))
				}
				return resourceResult(uri, tree), nil
			case "blockers":
				blockers := map[string]any{
					"issue":                 compactIssue(issue),
					"blockers":              compactIssues(issue.Blockers),
					"unresolved_blockers":   unresolvedIssues(issue.Blockers),
					"resolved_blockers":     resolvedIssues(issue.Blockers),
					"blocked_by":            compactIssues(issue.BlockedBy),
					"unresolved_blocked_by": unresolvedIssues(issue.BlockedBy),
					"resolved_blocked_by":   resolvedIssues(issue.BlockedBy),
				}
				return resourceResult(uri, blockers), nil
			}
			return resourceResult(uri, issue), nil
		}
		return nil, resourceNotFound(uri, nil)
	}
}

func (s *Server) issueDetails(ctx context.Context, issues []domain.Issue) ([]domain.Issue, error) {
	details := make([]domain.Issue, 0, len(issues))
	for _, issue := range issues {
		detail, err := s.service.GetIssue(ctx, issue.ID)
		if err != nil {
			return nil, err
		}
		details = append(details, detail)
	}
	return details, nil
}

func toolResult(summary string, structured any, err error) (any, *rpcError) {
	if err != nil {
		if appErr, ok := err.(*domain.AppError); ok {
			return toolErrorResult(appErr), nil
		}
		return nil, appToRPC(err)
	}
	return map[string]any{
		"content": []map[string]string{
			{"type": "text", "text": summary},
			{"type": "text", "text": mustJSON(structured)},
		},
		"structuredContent": structured,
		"isError":           false,
	}, nil
}

func toolErrorResult(appErr *domain.AppError) map[string]any {
	return map[string]any{
		"content": []map[string]string{
			{"type": "text", "text": appErr.Message},
			{"type": "text", "text": mustJSON(appErr)},
		},
		"structuredContent": appErr,
		"isError":           true,
	}
}

type dependencyContext struct {
	Issue               domain.Issue   `json:"issue"`
	Blockers            []domain.Issue `json:"blockers"`
	UnresolvedBlockers  []domain.Issue `json:"unresolved_blockers"`
	ResolvedBlockers    []domain.Issue `json:"resolved_blockers"`
	BlockedBy           []domain.Issue `json:"blocked_by"`
	UnresolvedBlockedBy []domain.Issue `json:"unresolved_blocked_by"`
	ResolvedBlockedBy   []domain.Issue `json:"resolved_blocked_by"`
}

func compactIssues(issues []domain.Issue) []domain.Issue {
	out := make([]domain.Issue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, compactIssue(issue))
	}
	return out
}

func compactIssue(issue domain.Issue) domain.Issue {
	return domain.Issue{
		ID:                  issue.ID,
		Title:               issue.Title,
		DescriptionMarkdown: issue.DescriptionMarkdown,
		Status:              issue.Status,
		Priority:            issue.Priority,
		Assignee:            issue.Assignee,
		CreatedBy:           issue.CreatedBy,
		ParentIssueID:       issue.ParentIssueID,
		CreatedAt:           issue.CreatedAt,
		UpdatedAt:           issue.UpdatedAt,
		Tags:                issue.Tags,
		Children:            []domain.Issue{},
		Blockers:            []domain.Issue{},
		BlockedBy:           []domain.Issue{},
		RecentComments:      []domain.Comment{},
		ChildCount:          issue.ChildCount,
		CommentCount:        issue.CommentCount,
		Blocked:             issue.Blocked,
	}
}

func childrenByParent(issues []domain.Issue) map[string][]domain.Issue {
	children := map[string][]domain.Issue{}
	for _, issue := range issues {
		if issue.ParentIssueID == nil {
			continue
		}
		parentID := *issue.ParentIssueID
		children[parentID] = append(children[parentID], compactIssue(issue))
	}
	return children
}

func blockedContexts(issues []domain.Issue) []dependencyContext {
	contexts := []dependencyContext{}
	for _, issue := range issues {
		if !issue.Blocked {
			continue
		}
		contexts = append(contexts, dependencyContext{
			Issue:               compactIssue(issue),
			Blockers:            compactIssues(issue.Blockers),
			UnresolvedBlockers:  unresolvedIssues(issue.Blockers),
			ResolvedBlockers:    resolvedIssues(issue.Blockers),
			BlockedBy:           []domain.Issue{},
			UnresolvedBlockedBy: []domain.Issue{},
			ResolvedBlockedBy:   []domain.Issue{},
		})
	}
	return contexts
}

func blockingContexts(issues []domain.Issue) []dependencyContext {
	contexts := []dependencyContext{}
	for _, issue := range issues {
		if len(issue.BlockedBy) == 0 {
			continue
		}
		contexts = append(contexts, dependencyContext{
			Issue:               compactIssue(issue),
			Blockers:            []domain.Issue{},
			UnresolvedBlockers:  []domain.Issue{},
			ResolvedBlockers:    []domain.Issue{},
			BlockedBy:           compactIssues(issue.BlockedBy),
			UnresolvedBlockedBy: unresolvedIssues(issue.BlockedBy),
			ResolvedBlockedBy:   resolvedIssues(issue.BlockedBy),
		})
	}
	return contexts
}

func unresolvedIssues(issues []domain.Issue) []domain.Issue {
	return compactIssues(filterIssues(issues, func(issue domain.Issue) bool {
		return !domain.TerminalStatus(issue.Status)
	}))
}

func resolvedIssues(issues []domain.Issue) []domain.Issue {
	return compactIssues(filterIssues(issues, func(issue domain.Issue) bool {
		return domain.TerminalStatus(issue.Status)
	}))
}

func filterIssues(issues []domain.Issue, keep func(domain.Issue) bool) []domain.Issue {
	filtered := []domain.Issue{}
	for _, issue := range issues {
		if keep(issue) {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

func emptyBoard() map[string][]domain.Issue {
	return map[string][]domain.Issue{
		string(domain.StatusNew):        {},
		string(domain.StatusInProgress): {},
		string(domain.StatusCompleted):  {},
		string(domain.StatusCanceled):   {},
	}
}

func resourceResult(uri string, data any) map[string]any {
	return map[string]any{
		"contents": []map[string]any{{
			"uri":      uri,
			"mimeType": "application/json",
			"text":     mustJSON(data),
		}},
	}
}

func capabilities() map[string]any {
	return map[string]any{
		"protocolVersion": "2025-06-18",
		"serverInfo":      map[string]string{"name": "tala", "version": "0.1.0"},
		"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
	}
}

func tools() []map[string]any {
	return []map[string]any{
		tool("issue_create", "Create an issue with Markdown description, tags, assignee, priority, and optional parent.", schema(
			props(
				strProp("username", "Required username for the mutation."),
				strProp("title", "Required issue title."),
				strProp("description_markdown", "Markdown source for the issue description."),
				enumProp("priority", []string{"P0", "P1", "P2", "P3", "P4"}),
				nullableStrProp("assignee", "Optional assignee username."),
				arrayProp("tag_names", "Tag names to attach, creating missing tags."),
				nullableStrProp("parent_issue_id", "Optional parent issue ID."),
			),
			[]string{"username", "title"},
		)),
		tool("issue_update", "Update issue fields.", schema(issueMutationProps(
			strProp("title", "Issue title."),
			strProp("description_markdown", "Markdown source for the issue description."),
			enumProp("status", []string{"new", "in_progress", "completed", "canceled"}),
			enumProp("priority", []string{"P0", "P1", "P2", "P3", "P4"}),
			nullableStrProp("assignee", "Optional assignee username; null clears it."),
			arrayProp("tag_names", "Replacement tag names."),
		), []string{"username", "issue_id"})),
		tool("issue_search", "Search and filter issues.", schema(props(
			enumProp("status", []string{"new", "in_progress", "completed", "canceled"}),
			enumProp("priority", []string{"P0", "P1", "P2", "P3", "P4"}),
			strProp("assignee", "Assignee username."),
			strProp("tag", "Tag name."),
			strProp("parent_id", "Parent issue ID."),
			strProp("blocked_by", "Blocker issue ID."),
			strProp("q", "Text query over title, Markdown description, comments, tags, ID, creator, assignee, status, and priority."),
		), nil)),
		tool("issue_get", "Fetch issue detail.", schema(props(strProp("issue_id", "Issue ID.")), []string{"issue_id"})),
		tool("issue_comment", "Append a Markdown comment.", schema(issueMutationProps(strProp("body_markdown", "Required Markdown comment body.")), []string{"username", "issue_id", "body_markdown"})),
		tool("issue_set_parent", "Set or clear an issue parent.", schema(issueMutationProps(nullableStrProp("parent_issue_id", "Parent issue ID, or null to clear.")), []string{"username", "issue_id", "parent_issue_id"})),
		tool("issue_add_blocker", "Add a blocker issue.", schema(issueMutationProps(strProp("blocker_issue_id", "Issue ID that blocks this issue.")), []string{"username", "issue_id", "blocker_issue_id"})),
		tool("issue_remove_blocker", "Remove a blocker issue.", schema(issueMutationProps(strProp("blocker_issue_id", "Blocker issue ID to remove.")), []string{"username", "issue_id", "blocker_issue_id"})),
		tool("issue_assign", "Set or clear assignee.", schema(issueMutationProps(nullableStrProp("assignee", "Assignee username, or null to clear.")), []string{"username", "issue_id", "assignee"})),
		tool("issue_set_status", "Change issue status.", schema(issueMutationProps(enumProp("status", []string{"new", "in_progress", "completed", "canceled"})), []string{"username", "issue_id", "status"})),
		tool("issue_set_priority", "Change issue priority.", schema(issueMutationProps(enumProp("priority", []string{"P0", "P1", "P2", "P3", "P4"})), []string{"username", "issue_id", "priority"})),
	}
}

func tool(name, description string, inputSchema map[string]any) map[string]any {
	return map[string]any{"name": name, "description": description, "inputSchema": inputSchema}
}

func schema(properties map[string]any, required []string) map[string]any {
	out := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

func issueMutationProps(extra ...map[string]any) map[string]any {
	return props(append([]map[string]any{
		strProp("username", "Required username for the mutation."),
		strProp("issue_id", "Issue ID."),
	}, extra...)...)
}

func props(fields ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, field := range fields {
		for name, schema := range field {
			out[name] = schema
		}
	}
	return out
}

func strProp(name, description string) map[string]any {
	return map[string]any{name: map[string]any{"type": "string", "description": description}}
}

func nullableStrProp(name, description string) map[string]any {
	return map[string]any{name: map[string]any{"type": []string{"string", "null"}, "description": description}}
}

func arrayProp(name, description string) map[string]any {
	return map[string]any{name: map[string]any{"type": "array", "items": map[string]string{"type": "string"}, "description": description}}
}

func enumProp(name string, values []string) map[string]any {
	return map[string]any{name: map[string]any{"type": "string", "enum": values}}
}

func resources() []map[string]any {
	return []map[string]any{
		{
			"uri":         "tala://board",
			"name":        "Board",
			"description": "Compact board state grouped by status.",
			"mimeType":    "application/json",
		},
		{
			"uri":         "tala://planning",
			"name":        "Planning",
			"description": "High-level planning context across hierarchy and blockers.",
			"mimeType":    "application/json",
		},
	}
}

func resourceTemplates() []map[string]any {
	return []map[string]any{
		{
			"uriTemplate": "tala://issues/{id}",
			"name":        "Issue detail",
			"description": "Issue detail context including tags, children, blockers, and recent comments.",
			"mimeType":    "application/json",
		},
		{
			"uriTemplate": "tala://issues/{id}/tree",
			"name":        "Issue tree",
			"description": "Parent, siblings, and children for an issue.",
			"mimeType":    "application/json",
		},
		{
			"uriTemplate": "tala://issues/{id}/blockers",
			"name":        "Issue blockers",
			"description": "Blockers and issues blocked by this issue.",
			"mimeType":    "application/json",
		},
	}
}

func appToRPC(err error) *rpcError {
	if appErr, ok := err.(*domain.AppError); ok {
		return &rpcError{Code: -32000, Message: appErr.Message, Data: appErr}
	}
	return &rpcError{Code: -32603, Message: "Internal error"}
}

func resourceReadError(uri string, err error) *rpcError {
	if appErr, ok := err.(*domain.AppError); ok && appErr.Code == domain.CodeNotFound {
		return resourceNotFound(uri, appErr)
	}
	return appToRPC(err)
}

func resourceNotFound(uri string, appErr *domain.AppError) *rpcError {
	data := map[string]any{"uri": uri}
	if appErr != nil {
		data["code"] = appErr.Code
		data["field"] = appErr.Field
	}
	return &rpcError{Code: -32002, Message: "Resource not found", Data: data}
}

func invalidParams(err error) *rpcError {
	return &rpcError{Code: -32602, Message: "Invalid params", Data: err.Error()}
}

func invalidParamsMessage(message string) *rpcError {
	return &rpcError{Code: -32602, Message: "Invalid params", Data: message}
}

func objectParams(raw json.RawMessage) (map[string]json.RawMessage, *rpcError) {
	clean := strings.TrimSpace(string(raw))
	if clean == "" || clean == "null" || !strings.HasPrefix(clean, "{") {
		return nil, invalidParamsMessage("params must be an object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		if err != nil {
			return nil, invalidParams(err)
		}
		return nil, invalidParamsMessage("params must be an object")
	}
	return fields, nil
}

func requiredStringParam(fields map[string]json.RawMessage, name string) (string, *rpcError) {
	raw, ok := fields[name]
	if !ok || string(raw) == "null" {
		return "", invalidParamsMessage(name + " is required")
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", invalidParams(err)
	}
	if strings.TrimSpace(value) == "" {
		return "", invalidParamsMessage(name + " is required")
	}
	return value, nil
}

func requireArgument(raw json.RawMessage, name string) *domain.AppError {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return domain.NewError(domain.CodeValidationError, "Invalid arguments.", "arguments")
	}
	if _, ok := fields[name]; !ok {
		return domain.NewError(domain.CodeValidationError, "Required argument is missing.", name)
	}
	return nil
}

func rejectNullArguments(fields map[string]json.RawMessage, names ...string) *domain.AppError {
	for _, name := range names {
		if raw, ok := fields[name]; ok && string(raw) == "null" {
			return domain.NewError(domain.CodeValidationError, "Argument must not be null.", name)
		}
	}
	return nil
}

func nullableStringArgument(raw json.RawMessage, name, message string) (*string, *domain.AppError) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, domain.NewError(domain.CodeValidationError, "Invalid arguments.", "arguments")
	}
	value, ok := fields[name]
	if !ok || string(value) == "null" {
		return nil, nil
	}
	var parsed string
	if err := json.Unmarshal(value, &parsed); err != nil {
		return nil, domain.NewError(domain.CodeValidationError, message, name)
	}
	return &parsed, nil
}

func stringArgument(fields map[string]json.RawMessage, name, message string) (string, *domain.AppError) {
	var parsed string
	if err := json.Unmarshal(fields[name], &parsed); err != nil {
		return "", domain.NewError(domain.CodeValidationError, message, name)
	}
	return parsed, nil
}

func requiredStringArgument(fields map[string]json.RawMessage, name, message string) (string, *domain.AppError) {
	raw, ok := fields[name]
	if !ok {
		return "", domain.NewError(domain.CodeValidationError, "Required argument is missing.", name)
	}
	if string(raw) == "null" {
		return "", domain.NewError(domain.CodeValidationError, "Argument must not be null.", name)
	}
	return stringArgument(fields, name, message)
}

func optionalStringArgument(fields map[string]json.RawMessage, name, message string) (string, *domain.AppError) {
	raw, ok := fields[name]
	if !ok {
		return "", nil
	}
	if string(raw) == "null" {
		return "", domain.NewError(domain.CodeValidationError, "Argument must not be null.", name)
	}
	return stringArgument(fields, name, message)
}

func optionalStringPointerArgument(fields map[string]json.RawMessage, name, message string) (*string, *domain.AppError) {
	if _, ok := fields[name]; !ok {
		return nil, nil
	}
	parsed, appErr := optionalStringArgument(fields, name, message)
	if appErr != nil {
		return nil, appErr
	}
	return &parsed, nil
}

func issueFilterArguments(fields map[string]json.RawMessage) (domain.IssueFilters, *domain.AppError) {
	status, appErr := optionalStringArgument(fields, "status", "Argument must be a string.")
	if appErr != nil {
		return domain.IssueFilters{}, appErr
	}
	priority, appErr := optionalStringArgument(fields, "priority", "Argument must be a string.")
	if appErr != nil {
		return domain.IssueFilters{}, appErr
	}
	assignee, appErr := optionalStringArgument(fields, "assignee", "Argument must be a string.")
	if appErr != nil {
		return domain.IssueFilters{}, appErr
	}
	tag, appErr := optionalStringArgument(fields, "tag", "Argument must be a string.")
	if appErr != nil {
		return domain.IssueFilters{}, appErr
	}
	parentID, appErr := optionalStringArgument(fields, "parent_id", "Argument must be a string.")
	if appErr != nil {
		return domain.IssueFilters{}, appErr
	}
	blockedBy, appErr := optionalStringArgument(fields, "blocked_by", "Argument must be a string.")
	if appErr != nil {
		return domain.IssueFilters{}, appErr
	}
	query, appErr := optionalStringArgument(fields, "q", "Argument must be a string.")
	if appErr != nil {
		return domain.IssueFilters{}, appErr
	}
	return domain.IssueFilters{
		Status:    status,
		Priority:  priority,
		Assignee:  assignee,
		Tag:       tag,
		ParentID:  parentID,
		BlockedBy: blockedBy,
		Query:     query,
	}, nil
}

func usernameArgument(fields map[string]json.RawMessage) (string, *domain.AppError) {
	raw, ok := fields["username"]
	if !ok || string(raw) == "null" {
		return "", domain.NewError(domain.CodeMissingUsername, "Username is required for this operation.", "username")
	}
	username, appErr := stringArgument(fields, "username", "Username must be a string.")
	if appErr != nil {
		return "", appErr
	}
	if strings.TrimSpace(username) == "" {
		return "", domain.NewError(domain.CodeMissingUsername, "Username is required for this operation.", "username")
	}
	return username, nil
}

func optionalStringSliceArgument(fields map[string]json.RawMessage, name string) ([]string, bool, *domain.AppError) {
	value, ok := fields[name]
	if !ok {
		return nil, false, nil
	}
	if string(value) == "null" {
		return nil, false, domain.NewError(domain.CodeValidationError, "Tag names must be an array.", name)
	}
	var parsed []string
	if err := json.Unmarshal(value, &parsed); err != nil {
		return nil, false, domain.NewError(domain.CodeValidationError, "Tag names must be an array.", name)
	}
	return parsed, true, nil
}

func requireUsername(username string) error {
	if strings.TrimSpace(username) == "" {
		return domain.NewError(domain.CodeMissingUsername, "Username is required for this operation.", "username")
	}
	return nil
}

func allowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := parsed.Hostname()
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func validProtocolVersion(version string) bool {
	switch strings.TrimSpace(version) {
	case "", "2025-03-26", "2025-06-18":
		return true
	default:
		return false
	}
}

func validPostAccept(values []string) bool {
	accept := strings.Join(values, ",")
	if strings.TrimSpace(accept) == "" {
		return false
	}
	hasJSON := false
	hasSSE := false
	for _, part := range strings.Split(accept, ",") {
		mediaType := strings.TrimSpace(strings.Split(part, ";")[0])
		switch mediaType {
		case "application/json":
			hasJSON = true
		case "text/event-stream":
			hasSSE = true
		}
	}
	return hasJSON && hasSSE
}

func mustJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func writeTransportError(w http.ResponseWriter, status int, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response{JSONRPC: "2.0", ID: nil, Error: &rpcError{Code: code, Message: message}})
}
