package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tala/internal/app"
	"tala/internal/domain"
	"tala/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "tala.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return New(app.NewService(st))
}

func newTestServerWithUploads(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, ".tala", "tala.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
	})
	uploadDir := filepath.Join(dir, ".tala", "uploads", "images")
	return New(app.NewServiceWithUploadDir(st, uploadDir)), uploadDir
}

func TestMCPInitializeAdvertisesServerCapabilities(t *testing.T) {
	server := newTestServer(t)

	res := rpcRequest(t, server, "http://127.0.0.1:8080", "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"clientInfo":      map[string]any{"name": "tala-test", "version": "0.0.0"},
		"capabilities":    map[string]any{},
	})
	if res.Code != http.StatusOK {
		t.Fatalf("initialize failed: %d %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("MCP-Protocol-Version"); got != "2025-06-18" {
		t.Fatalf("expected protocol version response header, got %q", got)
	}

	var body map[string]any
	decodeRecorder(t, res, &body)
	if body["error"] != nil {
		t.Fatalf("initialize returned rpc error: %#v", body["error"])
	}
	result := body["result"].(map[string]any)
	if result["protocolVersion"] != "2025-06-18" {
		t.Fatalf("expected initialize protocol version 2025-06-18, got %#v", result["protocolVersion"])
	}
	serverInfo := result["serverInfo"].(map[string]any)
	if serverInfo["name"] != "tala" || serverInfo["version"] != "0.1.0" {
		t.Fatalf("unexpected server info: %#v", serverInfo)
	}
	capabilities := result["capabilities"].(map[string]any)
	if _, ok := capabilities["tools"].(map[string]any); !ok {
		t.Fatalf("expected tools capability object, got %#v", capabilities["tools"])
	}
	if _, ok := capabilities["resources"].(map[string]any); !ok {
		t.Fatalf("expected resources capability object, got %#v", capabilities["resources"])
	}
}

func TestMCPOriginToolsAndResources(t *testing.T) {
	server := newTestServer(t)

	getReq := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	getReq.Header.Set("Origin", "http://127.0.0.1:8080")
	getRes := httptest.NewRecorder()
	server.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected GET /mcp to return 405, got %d %s", getRes.Code, getRes.Body.String())
	}
	if allow := getRes.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected Allow: POST for unsupported MCP method, got %q", allow)
	}
	assertTransportError(t, getRes, "MCP endpoint only supports POST")

	forbidden := rpcRequest(t, server, "https://example.com", "tools/list", map[string]any{})
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden origin, got %d", forbidden.Code)
	}
	assertTransportError(t, forbidden, "Forbidden origin")
	forbiddenScheme := rpcRequest(t, server, "file://localhost", "tools/list", map[string]any{})
	if forbiddenScheme.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden non-HTTP local origin, got %d", forbiddenScheme.Code)
	}
	assertTransportError(t, forbiddenScheme, "Forbidden origin")
	secureLocal := rpcRequest(t, server, "https://localhost", "tools/list", map[string]any{})
	if secureLocal.Code != http.StatusOK {
		t.Fatalf("expected HTTPS localhost origin to be allowed, got %d %s", secureLocal.Code, secureLocal.Body.String())
	}

	tools := rpcRequest(t, server, "http://127.0.0.1:8080", "tools/list", map[string]any{})
	if tools.Code != http.StatusOK {
		t.Fatalf("tools/list failed: %d %s", tools.Code, tools.Body.String())
	}
	var toolsBody map[string]any
	decodeRecorder(t, tools, &toolsBody)
	toolList := toolsBody["result"].(map[string]any)["tools"].([]any)
	if len(toolList) != 12 {
		t.Fatalf("expected 12 tools, got %d", len(toolList))
	}
	firstSchema := toolList[0].(map[string]any)["inputSchema"].(map[string]any)
	if firstSchema["type"] != "object" || firstSchema["properties"] == nil {
		t.Fatalf("expected concrete input schema, got %#v", firstSchema)
	}
	assertNullableToolProp(t, toolList, "issue_assign", "assignee")
	assertNullableToolProp(t, toolList, "issue_set_parent", "parent_issue_id")
	assertRequiredToolProp(t, toolList, "issue_assign", "assignee")
	assertRequiredToolProp(t, toolList, "issue_set_parent", "parent_issue_id")
	assertRequiredToolProps(t, toolList, map[string][]string{
		"image_upload":         {"username", "path"},
		"issue_create":         {"username", "title"},
		"issue_update":         {"username", "issue_id"},
		"issue_search":         nil,
		"issue_get":            {"issue_id"},
		"issue_comment":        {"username", "issue_id", "body_markdown"},
		"issue_set_parent":     {"username", "issue_id", "parent_issue_id"},
		"issue_add_blocker":    {"username", "issue_id", "blocker_issue_id"},
		"issue_remove_blocker": {"username", "issue_id", "blocker_issue_id"},
		"issue_assign":         {"username", "issue_id", "assignee"},
		"issue_set_status":     {"username", "issue_id", "status"},
		"issue_set_priority":   {"username", "issue_id", "priority"},
	})
	assertRequiredUsername(t, toolList)
	assertToolSchemasUseScalarTypes(t, toolList)
	assertToolPropertiesHaveDescriptions(t, toolList)
	assertToolPropDescriptionContains(t, toolList, "issue_search", "q", "comments", "tags", "creator", "priority")
	assertToolPropDescriptionContains(t, toolList, "issue_search", "blocker_of", "blocked")
	assertRequiredToolProp(t, toolList, "image_upload", "path")

	resources := rpcRequest(t, server, "http://127.0.0.1:8080", "resources/list", map[string]any{})
	if resources.Code != http.StatusOK {
		t.Fatalf("resources/list failed: %d %s", resources.Code, resources.Body.String())
	}
	var resourcesBody map[string]any
	decodeRecorder(t, resources, &resourcesBody)
	resourceList := resourcesBody["result"].(map[string]any)["resources"].([]any)
	assertResourceURI(t, resourceList, "tala://board")
	assertResourceURI(t, resourceList, "tala://planning")
	for _, item := range resourceList {
		resource := item.(map[string]any)
		if _, ok := resource["uriTemplate"]; ok {
			t.Fatalf("resources/list should only advertise concrete resources, got %#v", resource)
		}
	}

	templates := rpcRequest(t, server, "http://127.0.0.1:8080", "resources/templates/list", map[string]any{})
	if templates.Code != http.StatusOK {
		t.Fatalf("resources/templates/list failed: %d %s", templates.Code, templates.Body.String())
	}
	var templatesBody map[string]any
	decodeRecorder(t, templates, &templatesBody)
	templateList := templatesBody["result"].(map[string]any)["resourceTemplates"].([]any)
	assertResourceTemplate(t, templateList, "tala://issues/{id}")
	assertResourceTemplate(t, templateList, "tala://issues/{id}/tree")
	assertResourceTemplate(t, templateList, "tala://issues/{id}/blockers")

	parent := callTool(t, server, "issue_create", map[string]any{
		"username":             "alex",
		"title":                "Parent issue",
		"description_markdown": "Parent **markdown**",
		"priority":             "P2",
		"tag_names":            []string{"planning"},
	})
	parentID := structuredID(t, parent)
	nullCreateTags := callToolExpectToolError(t, server, "issue_create", map[string]any{
		"username":  "alex",
		"title":     "Null create tags",
		"tag_names": nil,
	})
	assertToolAppError(t, nullCreateTags, domain.CodeValidationError, "tag_names")
	nullCreatePriority := callToolExpectToolError(t, server, "issue_create", map[string]any{
		"username": "alex",
		"title":    "Null create priority",
		"priority": nil,
	})
	assertToolAppError(t, nullCreatePriority, domain.CodeValidationError, "priority")
	for _, tt := range []struct {
		args  map[string]any
		field string
	}{
		{args: map[string]any{"username": "alex", "title": 42}, field: "title"},
		{args: map[string]any{"username": "alex", "title": "Invalid create description", "description_markdown": 42}, field: "description_markdown"},
		{args: map[string]any{"username": "alex", "title": "Invalid create priority", "priority": 42}, field: "priority"},
		{args: map[string]any{"username": "alex", "title": "Invalid create assignee", "assignee": 42}, field: "assignee"},
		{args: map[string]any{"username": "alex", "title": "Invalid create parent", "parent_issue_id": 42}, field: "parent_issue_id"},
		{args: map[string]any{"username": "alex", "title": "Invalid create tags", "tag_names": 42}, field: "tag_names"},
	} {
		invalidCreate := callToolExpectToolError(t, server, "issue_create", tt.args)
		assertToolAppError(t, invalidCreate, domain.CodeValidationError, tt.field)
	}
	child := callTool(t, server, "issue_create", map[string]any{
		"username":        "alex",
		"title":           "Child issue",
		"priority":        "P1",
		"assignee":        "sam",
		"parent_issue_id": parentID,
	})
	childID := structuredID(t, child)
	updated := callTool(t, server, "issue_update", map[string]any{
		"username": "alex",
		"issue_id": childID,
		"assignee": nil,
	})
	updatedIssue := updated["result"].(map[string]any)["structuredContent"].(map[string]any)
	if updatedIssue["assignee"] != nil {
		t.Fatalf("expected MCP issue_update to clear assignee, got %#v", updatedIssue["assignee"])
	}
	tagged := callTool(t, server, "issue_update", map[string]any{
		"username":  "alex",
		"issue_id":  childID,
		"tag_names": []string{"mcp", "api"},
	})
	taggedIssue := tagged["result"].(map[string]any)["structuredContent"].(map[string]any)
	if tags := taggedIssue["tags"].([]any); len(tags) != 2 {
		t.Fatalf("expected MCP issue_update to replace tags, got %#v", tags)
	}
	renamed := callTool(t, server, "issue_update", map[string]any{
		"username": "alex",
		"issue_id": childID,
		"title":    "Child issue renamed",
	})
	renamedIssue := renamed["result"].(map[string]any)["structuredContent"].(map[string]any)
	if tags := renamedIssue["tags"].([]any); len(tags) != 2 {
		t.Fatalf("expected omitted MCP tag_names to preserve tags, got %#v", tags)
	}
	nullTags := callToolExpectToolError(t, server, "issue_update", map[string]any{
		"username":  "alex",
		"issue_id":  childID,
		"tag_names": nil,
	})
	assertToolAppError(t, nullTags, domain.CodeValidationError, "tag_names")
	nullStatus := callToolExpectToolError(t, server, "issue_update", map[string]any{
		"username": "alex",
		"issue_id": childID,
		"status":   nil,
	})
	assertToolAppError(t, nullStatus, domain.CodeValidationError, "status")
	invalidUpdateAssignee := callToolExpectToolError(t, server, "issue_update", map[string]any{
		"username": "alex",
		"issue_id": childID,
		"assignee": 42,
	})
	assertToolAppError(t, invalidUpdateAssignee, domain.CodeValidationError, "assignee")
	invalidUpdateTags := callToolExpectToolError(t, server, "issue_update", map[string]any{
		"username":  "alex",
		"issue_id":  childID,
		"tag_names": 42,
	})
	assertToolAppError(t, invalidUpdateTags, domain.CodeValidationError, "tag_names")
	for _, tt := range []struct {
		args  map[string]any
		field string
	}{
		{args: map[string]any{"username": "alex", "issue_id": childID, "title": 42}, field: "title"},
		{args: map[string]any{"username": "alex", "issue_id": childID, "description_markdown": 42}, field: "description_markdown"},
		{args: map[string]any{"username": "alex", "issue_id": childID, "status": 42}, field: "status"},
		{args: map[string]any{"username": "alex", "issue_id": childID, "priority": 42}, field: "priority"},
	} {
		invalidUpdate := callToolExpectToolError(t, server, "issue_update", tt.args)
		assertToolAppError(t, invalidUpdate, domain.CodeValidationError, tt.field)
	}
	clearedTags := callTool(t, server, "issue_update", map[string]any{
		"username":  "alex",
		"issue_id":  childID,
		"tag_names": []string{},
	})
	clearedTagsIssue := clearedTags["result"].(map[string]any)["structuredContent"].(map[string]any)
	if tags, ok := clearedTagsIssue["tags"].([]any); ok && len(tags) != 0 {
		t.Fatalf("expected empty MCP tag_names array to clear tags, got %#v", tags)
	}
	tagSearch := callTool(t, server, "issue_search", map[string]any{"tag": "mcp"})
	tagSearchResults := tagSearch["result"].(map[string]any)["structuredContent"].([]any)
	if len(tagSearchResults) != 0 {
		t.Fatalf("expected cleared MCP tags to be absent from tag search, got %#v", tagSearchResults)
	}
	missingAssignee := callToolExpectToolError(t, server, "issue_assign", map[string]any{
		"username": "alex",
		"issue_id": childID,
	})
	assertToolAppError(t, missingAssignee, domain.CodeValidationError, "assignee")
	missingParent := callToolExpectToolError(t, server, "issue_set_parent", map[string]any{
		"username": "alex",
		"issue_id": childID,
	})
	assertToolAppError(t, missingParent, domain.CodeValidationError, "parent_issue_id")
	blocker := callTool(t, server, "issue_create", map[string]any{
		"username": "alex",
		"title":    "Blocker issue",
		"priority": "P0",
	})
	blockerID := structuredID(t, blocker)
	callTool(t, server, "issue_add_blocker", map[string]any{
		"username":         "alex",
		"issue_id":         childID,
		"blocker_issue_id": blockerID,
	})
	resolvedBlocker := callTool(t, server, "issue_create", map[string]any{
		"username": "alex",
		"title":    "Resolved blocker issue",
		"priority": "P3",
	})
	resolvedBlockerID := structuredID(t, resolvedBlocker)
	paddedPriority := callTool(t, server, "issue_set_priority", map[string]any{
		"username": "alex",
		"issue_id": childID,
		"priority": " P1 ",
	})
	paddedPriorityIssue := paddedPriority["result"].(map[string]any)["structuredContent"].(map[string]any)
	if paddedPriorityIssue["priority"] != string(domain.PriorityP1) {
		t.Fatalf("expected padded MCP priority to be trimmed, got %#v", paddedPriorityIssue["priority"])
	}
	paddedStatus := callTool(t, server, "issue_set_status", map[string]any{
		"username": "alex",
		"issue_id": resolvedBlockerID,
		"status":   " completed ",
	})
	paddedStatusIssue := paddedStatus["result"].(map[string]any)["structuredContent"].(map[string]any)
	if paddedStatusIssue["status"] != string(domain.StatusCompleted) {
		t.Fatalf("expected padded MCP status to be trimmed, got %#v", paddedStatusIssue["status"])
	}
	callTool(t, server, "issue_add_blocker", map[string]any{
		"username":         "alex",
		"issue_id":         childID,
		"blocker_issue_id": resolvedBlockerID,
	})
	resolvedDependent := callTool(t, server, "issue_create", map[string]any{
		"username": "alex",
		"title":    "Resolved dependent issue",
		"priority": "P4",
	})
	resolvedDependentID := structuredID(t, resolvedDependent)
	callTool(t, server, "issue_add_blocker", map[string]any{
		"username":         "alex",
		"issue_id":         resolvedDependentID,
		"blocker_issue_id": blockerID,
	})
	callTool(t, server, "issue_set_status", map[string]any{
		"username": "alex",
		"issue_id": resolvedDependentID,
		"status":   string(domain.StatusCompleted),
	})

	search := callTool(t, server, "issue_search", map[string]any{"parent_id": parentID})
	searchResults := search["result"].(map[string]any)["structuredContent"].([]any)
	if len(searchResults) != 1 {
		t.Fatalf("expected snake_case parent_id search to find one issue, got %d", len(searchResults))
	}
	paddedSearch := callTool(t, server, "issue_search", map[string]any{"parent_id": " " + parentID + " ", "priority": " P1 ", "q": " Child "})
	paddedSearchResults := paddedSearch["result"].(map[string]any)["structuredContent"].([]any)
	if len(paddedSearchResults) != 1 {
		t.Fatalf("expected padded MCP search filters to find one issue, got %d", len(paddedSearchResults))
	}
	exactSearch := callTool(t, server, "issue_search", map[string]any{"id": childID})
	exactSearchResults := exactSearch["result"].(map[string]any)["structuredContent"].([]any)
	if len(exactSearchResults) != 1 {
		t.Fatalf("expected MCP id search to find one issue, got %d", len(exactSearchResults))
	}
	blockerOfSearch := callTool(t, server, "issue_search", map[string]any{"blocker_of": childID})
	blockerOfSearchResults := blockerOfSearch["result"].(map[string]any)["structuredContent"].([]any)
	if len(blockerOfSearchResults) != 2 {
		t.Fatalf("expected MCP blocker_of search to find blockers, got %d", len(blockerOfSearchResults))
	}
	stateSearch := callTool(t, server, "issue_search", map[string]any{"state": "blocked", "sort": "title", "order": "asc"})
	stateSearchResults := stateSearch["result"].(map[string]any)["structuredContent"].([]any)
	if len(stateSearchResults) == 0 {
		t.Fatal("expected MCP state search to find blocked issues")
	}
	missingParentSearch := callToolExpectToolError(t, server, "issue_search", map[string]any{"parent_id": "issue_missing_parent"})
	assertToolAppError(t, missingParentSearch, domain.CodeNotFound, "parent_id")
	missingBlockedBySearch := callToolExpectToolError(t, server, "issue_search", map[string]any{"blocked_by": "issue_missing_blocker"})
	assertToolAppError(t, missingBlockedBySearch, domain.CodeNotFound, "blocked_by")
	missingIDSearch := callToolExpectToolError(t, server, "issue_search", map[string]any{"id": "issue_missing"})
	assertToolAppError(t, missingIDSearch, domain.CodeNotFound, "id")
	missingBlockerOfSearch := callToolExpectToolError(t, server, "issue_search", map[string]any{"blocker_of": "issue_missing_blocked"})
	assertToolAppError(t, missingBlockerOfSearch, domain.CodeNotFound, "blocker_of")
	invalidGetIssueID := callToolExpectToolError(t, server, "issue_get", map[string]any{"issue_id": 42})
	assertToolAppError(t, invalidGetIssueID, domain.CodeValidationError, "issue_id")
	missingGetIssueID := callToolExpectToolError(t, server, "issue_get", map[string]any{})
	assertToolAppError(t, missingGetIssueID, domain.CodeValidationError, "issue_id")
	invalidSearchQuery := callToolExpectToolError(t, server, "issue_search", map[string]any{"q": 42})
	assertToolAppError(t, invalidSearchQuery, domain.CodeValidationError, "q")
	nullSearchStatus := callToolExpectToolError(t, server, "issue_search", map[string]any{"status": nil})
	assertToolAppError(t, nullSearchStatus, domain.CodeValidationError, "status")
	invalidSearchState := callToolExpectToolError(t, server, "issue_search", map[string]any{"state": "waiting"})
	assertToolAppError(t, invalidSearchState, domain.CodeValidationError, "state")
	invalidSearchSort := callToolExpectToolError(t, server, "issue_search", map[string]any{"sort": "rank"})
	assertToolAppError(t, invalidSearchSort, domain.CodeValidationError, "sort")
	invalidSearchOrder := callToolExpectToolError(t, server, "issue_search", map[string]any{"order": "reverse"})
	assertToolAppError(t, invalidSearchOrder, domain.CodeValidationError, "order")

	board := readResource(t, server, "tala://board")
	var boardData map[string][]map[string]any
	if err := json.Unmarshal([]byte(resourceText(t, board)), &boardData); err != nil {
		t.Fatalf("decode board resource: %v", err)
	}
	for _, status := range []string{"new", "in_progress", "completed", "canceled"} {
		if _, ok := boardData[status]; !ok {
			t.Fatalf("board resource missing %s group: %#v", status, boardData)
		}
	}
	if len(boardData["new"]) != 3 {
		t.Fatalf("expected new board group to contain created issues, got %#v", boardData["new"])
	}
	if !containsIssueID(boardData["completed"], resolvedBlockerID) || !containsIssueID(boardData["completed"], resolvedDependentID) {
		t.Fatalf("expected completed board group to contain resolved issues, got %#v", boardData["completed"])
	}
	for _, issue := range boardData["new"] {
		assertCompactResourceIssueShape(t, issue)
	}

	tree := readResource(t, server, "tala://issues/"+childID+"/tree")
	treeText := resourceText(t, tree)
	if !bytes.Contains([]byte(treeText), []byte("siblings")) || !bytes.Contains([]byte(treeText), []byte("Parent issue")) {
		t.Fatalf("tree resource missing expected context: %s", treeText)
	}
	rootTree := readResource(t, server, "tala://issues/"+parentID+"/tree")
	var rootTreeData struct {
		Parent   *map[string]any        `json:"parent"`
		Siblings []compactResourceIssue `json:"siblings"`
		Children []compactResourceIssue `json:"children"`
	}
	if err := json.Unmarshal([]byte(resourceText(t, rootTree)), &rootTreeData); err != nil {
		t.Fatalf("decode root tree resource: %v", err)
	}
	if rootTreeData.Parent != nil {
		t.Fatalf("expected root tree parent to be null, got %#v", rootTreeData.Parent)
	}
	if rootTreeData.Siblings == nil || len(rootTreeData.Siblings) != 0 {
		t.Fatalf("expected root tree siblings to be an empty array, got %#v", rootTreeData.Siblings)
	}
	if !containsCompactIssueID(rootTreeData.Children, childID) {
		t.Fatalf("expected root tree children to include child issue, got %#v", rootTreeData.Children)
	}

	blockers := readResource(t, server, "tala://issues/"+childID+"/blockers")
	blockerText := resourceText(t, blockers)
	if !bytes.Contains([]byte(blockerText), []byte("Blocker issue")) || !bytes.Contains([]byte(blockerText), []byte("blocked_by")) {
		t.Fatalf("blocker resource missing expected context: %s", blockerText)
	}
	var blockerResource struct {
		UnresolvedBlockers  []compactResourceIssue `json:"unresolved_blockers"`
		ResolvedBlockers    []compactResourceIssue `json:"resolved_blockers"`
		UnresolvedBlockedBy []compactResourceIssue `json:"unresolved_blocked_by"`
		ResolvedBlockedBy   []compactResourceIssue `json:"resolved_blocked_by"`
	}
	if err := json.Unmarshal([]byte(blockerText), &blockerResource); err != nil {
		t.Fatalf("decode blocker resource: %v", err)
	}
	if !containsCompactIssueID(blockerResource.UnresolvedBlockers, blockerID) || containsCompactIssueID(blockerResource.UnresolvedBlockers, resolvedBlockerID) {
		t.Fatalf("blocker resource unresolved split is wrong: %#v", blockerResource.UnresolvedBlockers)
	}
	if !containsCompactIssueID(blockerResource.ResolvedBlockers, resolvedBlockerID) {
		t.Fatalf("blocker resource resolved split missing resolved blocker: %#v", blockerResource.ResolvedBlockers)
	}
	blockingResource := readResource(t, server, "tala://issues/"+blockerID+"/blockers")
	if err := json.Unmarshal([]byte(resourceText(t, blockingResource)), &blockerResource); err != nil {
		t.Fatalf("decode blocking resource: %v", err)
	}
	if !containsCompactIssueID(blockerResource.UnresolvedBlockedBy, childID) || containsCompactIssueID(blockerResource.UnresolvedBlockedBy, resolvedDependentID) {
		t.Fatalf("blocking resource unresolved blocked_by split is wrong: %#v", blockerResource.UnresolvedBlockedBy)
	}
	if !containsCompactIssueID(blockerResource.ResolvedBlockedBy, resolvedDependentID) {
		t.Fatalf("blocking resource resolved blocked_by split missing resolved dependent: %#v", blockerResource.ResolvedBlockedBy)
	}
	resolvedBlockingResource := readResource(t, server, "tala://issues/"+resolvedBlockerID+"/blockers")
	if err := json.Unmarshal([]byte(resourceText(t, resolvedBlockingResource)), &blockerResource); err != nil {
		t.Fatalf("decode resolved blocking resource: %v", err)
	}
	if containsCompactIssueID(blockerResource.UnresolvedBlockedBy, childID) {
		t.Fatalf("resolved blocker should not have active blocked_by relationships: %#v", blockerResource.UnresolvedBlockedBy)
	}
	if !containsCompactIssueID(blockerResource.ResolvedBlockedBy, childID) {
		t.Fatalf("resolved blocker should report open dependents as resolved blocked_by relationships: %#v", blockerResource.ResolvedBlockedBy)
	}

	planningResource := readResource(t, server, "tala://planning")
	var planning struct {
		ChildrenByParent map[string][]map[string]any `json:"children_by_parent"`
		Blocked          []dependencyContext         `json:"blocked"`
		Blocking         []dependencyContext         `json:"blocking"`
	}
	if err := json.Unmarshal([]byte(resourceText(t, planningResource)), &planning); err != nil {
		t.Fatalf("decode planning resource: %v", err)
	}
	if !containsIssueID(planning.ChildrenByParent[parentID], childID) {
		t.Fatalf("planning resource missing child in children_by_parent: %#v", planning.ChildrenByParent)
	}
	if !dependencyHas(planning.Blocked, childID, blockerID) {
		t.Fatalf("planning resource missing blocked dependency context: %#v", planning.Blocked)
	}
	if !dependencyHasResolved(planning.Blocked, childID, resolvedBlockerID) {
		t.Fatalf("planning resource missing resolved blocker context: %#v", planning.Blocked)
	}
	if !dependencyHas(planning.Blocking, blockerID, childID) {
		t.Fatalf("planning resource missing blocking dependency context: %#v", planning.Blocking)
	}
	if !dependencyHasResolvedBlockedBy(planning.Blocking, blockerID, resolvedDependentID) {
		t.Fatalf("planning resource missing resolved blocked_by context: %#v", planning.Blocking)
	}
	if dependencyHas(planning.Blocking, resolvedBlockerID, childID) {
		t.Fatalf("planning resource treated completed blocker as actively blocking: %#v", planning.Blocking)
	}
	if !dependencyHasResolvedBlockedBy(planning.Blocking, resolvedBlockerID, childID) {
		t.Fatalf("planning resource missing completed blocker's resolved dependent context: %#v", planning.Blocking)
	}
	assertStableDependencyContexts(t, planning.Blocked)
	assertStableDependencyContexts(t, planning.Blocking)
	assertResourceSummariesOmitBulkyFields(t, resourceText(t, board))
	assertResourceSummariesOmitBulkyFields(t, treeText)
	assertResourceSummariesOmitBulkyFields(t, blockerText)
	assertResourceSummariesOmitBulkyFields(t, resourceText(t, planningResource))
}

func TestMCPImageUploadTool(t *testing.T) {
	server, uploadDir := newTestServerWithUploads(t)
	imagePath := filepath.Join(t.TempDir(), "browser-screenshot.png")
	if err := os.WriteFile(imagePath, []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	}, 0o644); err != nil {
		t.Fatal(err)
	}

	result := callTool(t, server, "image_upload", map[string]any{
		"username": "agent",
		"path":     imagePath,
		"alt_text": "mobile issue detail",
	})
	uploaded := result["result"].(map[string]any)["structuredContent"].(map[string]any)
	if uploaded["content_type"] != "image/png" {
		t.Fatalf("expected image/png upload, got %#v", uploaded)
	}
	url := uploaded["url"].(string)
	markdown := uploaded["markdown"].(string)
	if !strings.HasPrefix(url, "/uploads/images/") || !strings.Contains(markdown, url) || !strings.Contains(markdown, "mobile issue detail") {
		t.Fatalf("unexpected uploaded image response: %#v", uploaded)
	}
	filename := uploaded["filename"].(string)
	if _, err := os.Stat(filepath.Join(uploadDir, filename)); err != nil {
		t.Fatalf("expected uploaded file on disk: %v", err)
	}
}

func TestMCPImageUploadValidation(t *testing.T) {
	server, _ := newTestServerWithUploads(t)

	missingUser := callToolExpectToolError(t, server, "image_upload", map[string]any{"path": "missing.png"})
	assertToolAppError(t, missingUser, domain.CodeMissingUsername, "username")

	badPath := callToolExpectToolError(t, server, "image_upload", map[string]any{"username": "agent", "path": "missing.png"})
	assertToolAppError(t, badPath, domain.CodeNotFound, "path")

	invalidPath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(invalidPath, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	invalidImage := callToolExpectToolError(t, server, "image_upload", map[string]any{"username": "agent", "path": invalidPath})
	assertToolAppError(t, invalidImage, domain.CodeValidationError, "image")

	badType := callToolExpectToolError(t, server, "image_upload", map[string]any{"username": "agent", "path": 42})
	assertToolAppError(t, badType, domain.CodeValidationError, "path")
}

func TestMCPMutationsRequireUsernameAtRuntime(t *testing.T) {
	server := newTestServer(t)

	body := callToolExpectToolError(t, server, "issue_create", map[string]any{
		"title":    "Missing username",
		"priority": "P2",
	})
	assertToolAppError(t, body, domain.CodeMissingUsername, "username")

	commentBody := callToolExpectToolError(t, server, "issue_comment", map[string]any{"body_markdown": "No username."})
	assertToolAppError(t, commentBody, domain.CodeMissingUsername, "username")

	for _, tt := range []struct {
		name string
		args map[string]any
	}{
		{name: "issue_set_parent", args: map[string]any{"issue_id": "issue_missing"}},
		{name: "issue_assign", args: map[string]any{"issue_id": "issue_missing"}},
	} {
		body := callToolExpectToolError(t, server, tt.name, tt.args)
		assertToolAppError(t, body, domain.CodeMissingUsername, "username")
	}

	for _, tt := range []struct {
		name string
		args map[string]any
	}{
		{name: "issue_create", args: map[string]any{"username": 42, "title": "Bad username"}},
		{name: "issue_update", args: map[string]any{"username": 42, "issue_id": "issue_missing", "title": "Bad username"}},
		{name: "issue_comment", args: map[string]any{"username": 42, "issue_id": "issue_missing", "body_markdown": "Bad username"}},
		{name: "issue_set_parent", args: map[string]any{"username": 42, "issue_id": "issue_missing", "parent_issue_id": nil}},
		{name: "issue_add_blocker", args: map[string]any{"username": 42, "issue_id": "issue_missing", "blocker_issue_id": "issue_other"}},
		{name: "issue_remove_blocker", args: map[string]any{"username": 42, "issue_id": "issue_missing", "blocker_issue_id": "issue_other"}},
		{name: "issue_assign", args: map[string]any{"username": 42, "issue_id": "issue_missing", "assignee": nil}},
		{name: "issue_set_status", args: map[string]any{"username": 42, "issue_id": "issue_missing", "status": "in_progress"}},
		{name: "issue_set_priority", args: map[string]any{"username": 42, "issue_id": "issue_missing", "priority": "P1"}},
	} {
		body := callToolExpectToolError(t, server, tt.name, tt.args)
		assertToolAppError(t, body, domain.CodeValidationError, "username")
	}
}

func TestMCPPreservesMarkdownSourceExactly(t *testing.T) {
	server := newTestServer(t)

	initialMarkdown := "MCP **source**\n\n<script>alert(1)</script>\n\n[unsafe](javascript:alert(1))"
	created := callTool(t, server, "issue_create", map[string]any{
		"username":             "alex",
		"title":                "MCP Markdown exactness",
		"description_markdown": initialMarkdown,
		"priority":             "P2",
	})
	createdIssue := structuredIssue(t, created)
	issueID := createdIssue["id"].(string)
	if createdIssue["description_markdown"] != initialMarkdown {
		t.Fatalf("MCP create changed Markdown source: got %#v want %q", createdIssue["description_markdown"], initialMarkdown)
	}

	updatedMarkdown := "Updated MCP source\n\n```json\n{\"raw\":true}\n```\n<span onclick=\"bad()\">kept as source</span>"
	updated := callTool(t, server, "issue_update", map[string]any{
		"username":             "alex",
		"issue_id":             issueID,
		"description_markdown": updatedMarkdown,
	})
	updatedIssue := structuredIssue(t, updated)
	if updatedIssue["description_markdown"] != updatedMarkdown {
		t.Fatalf("MCP update changed Markdown source: got %#v want %q", updatedIssue["description_markdown"], updatedMarkdown)
	}

	commentMarkdown := "  MCP comment **source** with <img src=x onerror=alert(1)>  "
	commented := callTool(t, server, "issue_comment", map[string]any{
		"username":      "sam",
		"issue_id":      issueID,
		"body_markdown": commentMarkdown,
	})
	comment := commented["result"].(map[string]any)["structuredContent"].(map[string]any)
	if comment["body_markdown"] != commentMarkdown {
		t.Fatalf("MCP comment changed Markdown source: got %#v want %q", comment["body_markdown"], commentMarkdown)
	}

	fetched := callTool(t, server, "issue_get", map[string]any{"issue_id": issueID})
	fetchedIssue := structuredIssue(t, fetched)
	if fetchedIssue["description_markdown"] != updatedMarkdown {
		t.Fatalf("MCP issue_get changed Markdown source: got %#v want %q", fetchedIssue["description_markdown"], updatedMarkdown)
	}
	recentComments := fetchedIssue["recent_comments"].([]any)
	if len(recentComments) != 1 || recentComments[0].(map[string]any)["body_markdown"] != commentMarkdown {
		t.Fatalf("MCP issue_get changed recent comment Markdown source: %#v", recentComments)
	}

	resource := readResource(t, server, "tala://issues/"+issueID)
	var resourceIssue domain.Issue
	if err := json.Unmarshal([]byte(resourceText(t, resource)), &resourceIssue); err != nil {
		t.Fatalf("decode issue resource: %v", err)
	}
	if resourceIssue.DescriptionMarkdown != updatedMarkdown {
		t.Fatalf("MCP issue resource changed Markdown source: got %q want %q", resourceIssue.DescriptionMarkdown, updatedMarkdown)
	}
	if len(resourceIssue.RecentComments) != 1 || resourceIssue.RecentComments[0].BodyMarkdown != commentMarkdown {
		t.Fatalf("MCP issue resource changed recent comment Markdown source: %#v", resourceIssue.RecentComments)
	}
}

func TestMCPStructuredToolResultMirrorsJSONInTextContent(t *testing.T) {
	server := newTestServer(t)

	result := callTool(t, server, "issue_create", map[string]any{
		"username":             "alex",
		"title":                "Structured mirror",
		"description_markdown": "Mirror **source**",
		"priority":             "P2",
	})
	toolResult := result["result"].(map[string]any)
	structured := toolResult["structuredContent"].(map[string]any)
	if toolResult["isError"] != false {
		t.Fatalf("expected successful tool result to include isError:false, got %#v", toolResult["isError"])
	}
	content := toolResult["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("expected summary and serialized structured content, got %#v", content)
	}
	if summary := content[0].(map[string]any)["text"]; summary == "" {
		t.Fatalf("expected first content item to contain a summary, got %#v", content[0])
	}
	rawJSON, ok := content[1].(map[string]any)["text"].(string)
	if !ok || rawJSON == "" {
		t.Fatalf("expected second content item to contain serialized JSON, got %#v", content[1])
	}
	var mirrored map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &mirrored); err != nil {
		t.Fatalf("serialized structured text is not JSON: %v\n%s", err, rawJSON)
	}
	if mirrored["id"] != structured["id"] || mirrored["description_markdown"] != structured["description_markdown"] {
		t.Fatalf("serialized structured text did not mirror structuredContent: text=%#v structured=%#v", mirrored, structured)
	}
}

func TestMCPResourceReadNotFoundUsesResourceErrorCode(t *testing.T) {
	server := newTestServer(t)

	for _, tt := range []struct {
		name string
		uri  string
	}{
		{name: "unknown resource", uri: "tala://missing"},
		{name: "missing issue resource", uri: "tala://issues/issue_missing"},
		{name: "missing issue tree resource", uri: "tala://issues/issue_missing/tree"},
	} {
		res := rpcRequest(t, server, "", "resources/read", map[string]any{"uri": tt.uri})
		if res.Code != http.StatusOK {
			t.Fatalf("%s: expected JSON-RPC response, got HTTP %d: %s", tt.name, res.Code, res.Body.String())
		}
		var body map[string]any
		decodeRecorder(t, res, &body)
		errBody, ok := body["error"].(map[string]any)
		if !ok || int(errBody["code"].(float64)) != -32002 {
			t.Fatalf("%s: expected resource not found error, got %#v", tt.name, body)
		}
		data := errBody["data"].(map[string]any)
		if data["uri"] != tt.uri {
			t.Fatalf("%s: expected resource error data to include uri %q, got %#v", tt.name, tt.uri, data)
		}
	}
}

func TestMCPWrappedAppErrorsRemainStructured(t *testing.T) {
	appErr := domain.NewError(domain.CodeValidationError, "Stable validation message.", "title")
	wrapped := fmt.Errorf("storage wrapper should not leak: %w", appErr)

	result, rpcErr := toolResult("", nil, wrapped)
	if rpcErr != nil {
		t.Fatalf("expected wrapped app error to stay a tool result, got rpc error %#v", rpcErr)
	}
	raw, err := json.Marshal(map[string]any{"result": result})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	assertToolAppError(t, body, domain.CodeValidationError, "title")
	toolBody := body["result"].(map[string]any)
	content := toolBody["content"].([]any)
	summary := content[0].(map[string]any)["text"].(string)
	if summary != appErr.Message || strings.Contains(summary, "storage wrapper") {
		t.Fatalf("tool error summary leaked wrapper text: %q", summary)
	}

	rpcAppErr := appToRPC(wrapped)
	if rpcAppErr.Code != -32000 || rpcAppErr.Message != appErr.Message {
		t.Fatalf("expected wrapped app rpc error to keep app message/code, got %#v", rpcAppErr)
	}
	data := rpcAppErr.Data.(*domain.AppError)
	if data.Code != appErr.Code || data.Field != appErr.Field || data.Message != appErr.Message {
		t.Fatalf("expected wrapped app rpc data to keep app error, got %#v", data)
	}
	internal := appToRPC(fmt.Errorf("driver secret detail"))
	if internal.Code != -32603 || internal.Message != "Internal error" || internal.Data != nil {
		t.Fatalf("expected generic errors to be sanitized, got %#v", internal)
	}
}

func TestMCPRelationshipValidationReportsRequestFields(t *testing.T) {
	server := newTestServer(t)

	issue := callTool(t, server, "issue_create", map[string]any{
		"username": "alex",
		"title":    "Relationship issue",
		"priority": "P2",
	})
	issueID := structuredID(t, issue)
	body := callToolExpectToolError(t, server, "issue_add_blocker", map[string]any{
		"username":         "alex",
		"issue_id":         issueID,
		"blocker_issue_id": " ",
	})
	assertToolAppError(t, body, domain.CodeValidationError, "blocker_issue_id")
	nullComment := callToolExpectToolError(t, server, "issue_comment", map[string]any{
		"username":      "alex",
		"issue_id":      issueID,
		"body_markdown": nil,
	})
	assertToolAppError(t, nullComment, domain.CodeValidationError, "body_markdown")
	for _, tt := range []struct {
		name  string
		args  map[string]any
		field string
	}{
		{name: "issue_update", args: map[string]any{"username": "alex", "issue_id": 42, "title": "Bad issue id"}, field: "issue_id"},
		{name: "issue_comment", args: map[string]any{"username": "alex", "issue_id": 42, "body_markdown": "Bad issue id"}, field: "issue_id"},
		{name: "issue_set_parent", args: map[string]any{"username": "alex", "issue_id": 42, "parent_issue_id": nil}, field: "issue_id"},
		{name: "issue_add_blocker", args: map[string]any{"username": "alex", "issue_id": 42, "blocker_issue_id": issueID}, field: "issue_id"},
		{name: "issue_remove_blocker", args: map[string]any{"username": "alex", "issue_id": 42, "blocker_issue_id": issueID}, field: "issue_id"},
		{name: "issue_assign", args: map[string]any{"username": "alex", "issue_id": 42, "assignee": nil}, field: "issue_id"},
		{name: "issue_set_status", args: map[string]any{"username": "alex", "issue_id": 42, "status": "in_progress"}, field: "issue_id"},
		{name: "issue_set_priority", args: map[string]any{"username": "alex", "issue_id": 42, "priority": "P1"}, field: "issue_id"},
		{name: "issue_set_parent", args: map[string]any{"username": "alex", "issue_id": issueID, "parent_issue_id": 42}, field: "parent_issue_id"},
		{name: "issue_assign", args: map[string]any{"username": "alex", "issue_id": issueID, "assignee": 42}, field: "assignee"},
		{name: "issue_comment", args: map[string]any{"username": "alex", "issue_id": issueID, "body_markdown": 42}, field: "body_markdown"},
		{name: "issue_add_blocker", args: map[string]any{"username": "alex", "issue_id": issueID, "blocker_issue_id": 42}, field: "blocker_issue_id"},
		{name: "issue_remove_blocker", args: map[string]any{"username": "alex", "issue_id": issueID, "blocker_issue_id": 42}, field: "blocker_issue_id"},
		{name: "issue_add_blocker", args: map[string]any{"username": "alex", "issue_id": issueID, "blocker_issue_id": nil}, field: "blocker_issue_id"},
		{name: "issue_remove_blocker", args: map[string]any{"username": "alex", "issue_id": issueID, "blocker_issue_id": nil}, field: "blocker_issue_id"},
		{name: "issue_set_status", args: map[string]any{"username": "alex", "issue_id": issueID, "status": 42}, field: "status"},
		{name: "issue_set_status", args: map[string]any{"username": "alex", "issue_id": issueID, "status": nil}, field: "status"},
		{name: "issue_set_priority", args: map[string]any{"username": "alex", "issue_id": issueID, "priority": 42}, field: "priority"},
		{name: "issue_set_priority", args: map[string]any{"username": "alex", "issue_id": issueID, "priority": nil}, field: "priority"},
	} {
		nullArg := callToolExpectToolError(t, server, tt.name, tt.args)
		assertToolAppError(t, nullArg, domain.CodeValidationError, tt.field)
	}
	for _, tt := range []struct {
		name  string
		args  map[string]any
		field string
	}{
		{name: "issue_comment", args: map[string]any{"username": "alex", "issue_id": issueID}, field: "body_markdown"},
		{name: "issue_add_blocker", args: map[string]any{"username": "alex", "issue_id": issueID}, field: "blocker_issue_id"},
		{name: "issue_remove_blocker", args: map[string]any{"username": "alex", "issue_id": issueID}, field: "blocker_issue_id"},
		{name: "issue_set_status", args: map[string]any{"username": "alex", "issue_id": issueID}, field: "status"},
		{name: "issue_set_priority", args: map[string]any{"username": "alex", "issue_id": issueID}, field: "priority"},
	} {
		missing := callToolExpectToolError(t, server, tt.name, tt.args)
		assertToolAppError(t, missing, domain.CodeValidationError, tt.field)
	}

	assignBody := callToolExpectToolError(t, server, "issue_assign", map[string]any{
		"username": "alex",
		"issue_id": "issue_missing",
		"assignee": "sam",
	})
	assertToolAppError(t, assignBody, domain.CodeNotFound, "issue_id")

	updateBody := callToolExpectToolError(t, server, "issue_update", map[string]any{
		"username": "alex",
		"issue_id": "issue_missing",
		"title":    "Missing",
	})
	assertToolAppError(t, updateBody, domain.CodeNotFound, "issue_id")
}

func assertRequiredUsername(t *testing.T, tools []any) {
	t.Helper()
	mutatingTools := map[string]bool{
		"image_upload":         true,
		"issue_create":         true,
		"issue_update":         true,
		"issue_comment":        true,
		"issue_set_parent":     true,
		"issue_add_blocker":    true,
		"issue_remove_blocker": true,
		"issue_assign":         true,
		"issue_set_status":     true,
		"issue_set_priority":   true,
	}
	seen := map[string]bool{}
	for _, item := range tools {
		tool := item.(map[string]any)
		name := tool["name"].(string)
		if !mutatingTools[name] {
			continue
		}
		seen[name] = true
		schema := tool["inputSchema"].(map[string]any)
		properties := schema["properties"].(map[string]any)
		if _, ok := properties["username"]; !ok {
			t.Fatalf("expected %s to expose username property", name)
		}
		required, ok := schema["required"].([]any)
		if !ok {
			t.Fatalf("expected %s to declare required fields", name)
		}
		found := false
		for _, field := range required {
			found = found || field == "username"
		}
		if !found {
			t.Fatalf("expected %s username to be required for this local server, got %#v", name, required)
		}
	}
	for name := range mutatingTools {
		if !seen[name] {
			t.Fatalf("mutating tool %s not found", name)
		}
	}
}

func assertRequiredToolProps(t *testing.T, tools []any, want map[string][]string) {
	t.Helper()
	seen := map[string]bool{}
	for _, item := range tools {
		tool := item.(map[string]any)
		name := tool["name"].(string)
		expected, ok := want[name]
		if !ok {
			t.Fatalf("unexpected tool %s", name)
		}
		seen[name] = true
		schema := tool["inputSchema"].(map[string]any)
		required := []any{}
		if raw, ok := schema["required"]; ok {
			required = raw.([]any)
		}
		if len(required) != len(expected) {
			t.Fatalf("expected %s required fields %v, got %#v", name, expected, required)
		}
		requiredSet := map[string]bool{}
		for _, field := range required {
			requiredSet[field.(string)] = true
		}
		for _, field := range expected {
			if !requiredSet[field] {
				t.Fatalf("expected %s required fields %v, got %#v", name, expected, required)
			}
		}
	}
	for name := range want {
		if !seen[name] {
			t.Fatalf("expected tool %s not found", name)
		}
	}
}

func assertToolSchemasUseScalarTypes(t *testing.T, tools []any) {
	t.Helper()
	for _, item := range tools {
		tool := item.(map[string]any)
		schema := tool["inputSchema"].(map[string]any)
		assertSchemaTypesAreScalar(t, tool["name"].(string)+".inputSchema", schema)
	}
}

func assertSchemaTypesAreScalar(t *testing.T, path string, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		if schemaType, ok := typed["type"]; ok {
			if _, ok := schemaType.(string); !ok {
				t.Fatalf("expected %s.type to be a scalar string, got %#v", path, schemaType)
			}
		}
		for name, child := range typed {
			assertSchemaTypesAreScalar(t, path+"."+name, child)
		}
	case []any:
		for i, child := range typed {
			assertSchemaTypesAreScalar(t, fmt.Sprintf("%s[%d]", path, i), child)
		}
	}
}

func assertToolPropertiesHaveDescriptions(t *testing.T, tools []any) {
	t.Helper()
	for _, item := range tools {
		tool := item.(map[string]any)
		name := tool["name"].(string)
		schema := tool["inputSchema"].(map[string]any)
		props := schema["properties"].(map[string]any)
		for propName, rawProp := range props {
			prop := rawProp.(map[string]any)
			description, ok := prop["description"].(string)
			if !ok || strings.TrimSpace(description) == "" {
				t.Fatalf("expected %s.%s to include a description, got %#v", name, propName, prop)
			}
		}
	}
}

func assertNullableToolProp(t *testing.T, tools []any, toolName, propName string) {
	t.Helper()
	for _, item := range tools {
		tool := item.(map[string]any)
		if tool["name"] != toolName {
			continue
		}
		schema := tool["inputSchema"].(map[string]any)
		props := schema["properties"].(map[string]any)
		prop := props[propName].(map[string]any)
		options := prop["anyOf"].([]any)
		if len(options) != 2 {
			t.Fatalf("expected %s.%s to accept string or null, got %#v", toolName, propName, prop["anyOf"])
		}
		seen := map[string]bool{}
		for _, option := range options {
			optionSchema := option.(map[string]any)
			seen[optionSchema["type"].(string)] = true
		}
		if !seen["string"] || !seen["null"] {
			t.Fatalf("expected %s.%s to accept string or null, got %#v", toolName, propName, prop["anyOf"])
		}
		return
	}
	t.Fatalf("tool %s not found", toolName)
}

func assertRequiredToolProp(t *testing.T, tools []any, toolName, propName string) {
	t.Helper()
	for _, item := range tools {
		tool := item.(map[string]any)
		if tool["name"] != toolName {
			continue
		}
		schema := tool["inputSchema"].(map[string]any)
		required := schema["required"].([]any)
		for _, field := range required {
			if field == propName {
				return
			}
		}
		t.Fatalf("expected %s.%s to be required, got %#v", toolName, propName, required)
	}
	t.Fatalf("tool %s not found", toolName)
}

func assertToolPropDescriptionContains(t *testing.T, tools []any, toolName, propName string, want ...string) {
	t.Helper()
	for _, item := range tools {
		tool := item.(map[string]any)
		if tool["name"] != toolName {
			continue
		}
		schema := tool["inputSchema"].(map[string]any)
		props := schema["properties"].(map[string]any)
		prop := props[propName].(map[string]any)
		description := prop["description"].(string)
		for _, part := range want {
			if !strings.Contains(description, part) {
				t.Fatalf("expected %s.%s description to mention %q, got %q", toolName, propName, part, description)
			}
		}
		return
	}
	t.Fatalf("tool %s not found", toolName)
}

func assertResourceURI(t *testing.T, resources []any, uri string) {
	t.Helper()
	for _, item := range resources {
		resource := item.(map[string]any)
		if resource["uri"] == uri {
			if resource["mimeType"] != "application/json" {
				t.Fatalf("expected %s to advertise JSON mime type, got %#v", uri, resource)
			}
			return
		}
	}
	t.Fatalf("resource %s not found in %#v", uri, resources)
}

func assertResourceTemplate(t *testing.T, templates []any, uriTemplate string) {
	t.Helper()
	for _, item := range templates {
		template := item.(map[string]any)
		if template["uriTemplate"] == uriTemplate {
			if template["mimeType"] != "application/json" {
				t.Fatalf("expected %s to advertise JSON mime type, got %#v", uriTemplate, template)
			}
			return
		}
	}
	t.Fatalf("resource template %s not found in %#v", uriTemplate, templates)
}

func containsIssueID(issues []map[string]any, id string) bool {
	for _, issue := range issues {
		if issue["id"] == id {
			return true
		}
	}
	return false
}

func assertCompactResourceIssueShape(t *testing.T, issue map[string]any) {
	t.Helper()
	for _, field := range []string{"id", "title", "status", "priority", "assignee", "parent_issue_id", "tags", "child_count", "comment_count", "blocked"} {
		if _, ok := issue[field]; !ok {
			t.Fatalf("compact resource issue missing field %q in %#v", field, issue)
		}
	}
	for _, field := range bulkyIssueFields() {
		if _, ok := issue[field]; ok {
			t.Fatalf("compact resource issue should omit field %q in %#v", field, issue)
		}
	}
	if _, ok := issue["tags"].([]any); !ok {
		t.Fatalf("expected compact resource issue tags to be an array, got %#v", issue["tags"])
	}
}

func assertResourceSummariesOmitBulkyFields(t *testing.T, text string) {
	t.Helper()
	for _, field := range bulkyIssueFields() {
		if bytes.Contains([]byte(text), []byte(`"`+field+`"`)) {
			t.Fatalf("expected compact resource summary to omit %q, got %s", field, text)
		}
	}
}

func bulkyIssueFields() []string {
	return []string{
		"description_markdown",
		"created_by",
		"created_at",
		"updated_at",
		"recent_comments",
	}
}

func dependencyHas(contexts []dependencyContext, issueID, relatedID string) bool {
	for _, context := range contexts {
		if context.Issue.ID != issueID {
			continue
		}
		if containsCompactIssueID(context.UnresolvedBlockers, relatedID) || containsCompactIssueID(context.UnresolvedBlockedBy, relatedID) {
			return true
		}
	}
	return false
}

func dependencyHasResolved(contexts []dependencyContext, issueID, relatedID string) bool {
	for _, context := range contexts {
		if context.Issue.ID == issueID && containsCompactIssueID(context.ResolvedBlockers, relatedID) {
			return true
		}
	}
	return false
}

func dependencyHasResolvedBlockedBy(contexts []dependencyContext, issueID, relatedID string) bool {
	for _, context := range contexts {
		if context.Issue.ID == issueID && containsCompactIssueID(context.ResolvedBlockedBy, relatedID) {
			return true
		}
	}
	return false
}

func assertStableDependencyContexts(t *testing.T, contexts []dependencyContext) {
	t.Helper()
	for _, context := range contexts {
		if context.Blockers == nil {
			t.Fatalf("expected dependency context blockers to be an array, got nil in %#v", context)
		}
		if context.UnresolvedBlockers == nil {
			t.Fatalf("expected dependency context unresolved_blockers to be an array, got nil in %#v", context)
		}
		if context.ResolvedBlockers == nil {
			t.Fatalf("expected dependency context resolved_blockers to be an array, got nil in %#v", context)
		}
		if context.BlockedBy == nil {
			t.Fatalf("expected dependency context blocked_by to be an array, got nil in %#v", context)
		}
		if context.UnresolvedBlockedBy == nil {
			t.Fatalf("expected dependency context unresolved_blocked_by to be an array, got nil in %#v", context)
		}
		if context.ResolvedBlockedBy == nil {
			t.Fatalf("expected dependency context resolved_blocked_by to be an array, got nil in %#v", context)
		}
	}
}

func containsDomainIssueID(issues []domain.Issue, id string) bool {
	for _, issue := range issues {
		if issue.ID == id {
			return true
		}
	}
	return false
}

func containsCompactIssueID(issues []compactResourceIssue, id string) bool {
	for _, issue := range issues {
		if issue.ID == id {
			return true
		}
	}
	return false
}

func TestMCPStreamableHTTPTransportBehavior(t *testing.T) {
	server := newTestServer(t)

	getReq := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	getReq.Header.Set("Accept", "text/event-stream")
	getRes := httptest.NewRecorder()
	server.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected GET without SSE support to return 405, got %d", getRes.Code)
	}
	if got := getRes.Header().Get("MCP-Protocol-Version"); got != "2025-06-18" {
		t.Fatalf("expected protocol version response header, got %q", got)
	}
	if allow := getRes.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected Allow: POST for unsupported MCP method, got %q", allow)
	}
	assertTransportError(t, getRes, "MCP endpoint only supports POST")

	badVersion := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)))
	badVersion.Header.Set("Accept", "application/json, text/event-stream")
	badVersion.Header.Set("MCP-Protocol-Version", "1999-01-01")
	badVersionRes := httptest.NewRecorder()
	server.ServeHTTP(badVersionRes, badVersion)
	if badVersionRes.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid protocol version to return 400, got %d", badVersionRes.Code)
	}
	assertTransportError(t, badVersionRes, "Unsupported MCP protocol version")

	missingAccept := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)))
	missingAcceptRes := httptest.NewRecorder()
	server.ServeHTTP(missingAcceptRes, missingAccept)
	if missingAcceptRes.Code != http.StatusNotAcceptable {
		t.Fatalf("expected missing Accept to return 406, got %d", missingAcceptRes.Code)
	}
	assertTransportError(t, missingAcceptRes, "MCP POST requires Accept: application/json, text/event-stream")

	trailingJSON := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"} {}`)))
	trailingJSON.Header.Set("Accept", "application/json, text/event-stream")
	trailingJSONRes := httptest.NewRecorder()
	server.ServeHTTP(trailingJSONRes, trailingJSON)
	if trailingJSONRes.Code != http.StatusOK {
		t.Fatalf("expected JSON-RPC parse error response to use HTTP 200, got %d", trailingJSONRes.Code)
	}
	var trailingBody response
	decodeRecorder(t, trailingJSONRes, &trailingBody)
	if trailingBody.Error == nil || trailingBody.Error.Code != -32700 {
		t.Fatalf("expected trailing JSON to produce parse error, got %#v", trailingBody)
	}
	var trailingRaw map[string]any
	decodeRecorder(t, trailingJSONRes, &trailingRaw)
	if _, ok := trailingRaw["id"]; !ok || trailingRaw["id"] != nil {
		t.Fatalf("expected parse error response to include id:null, got %#v", trailingRaw)
	}

	nullID := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":null,"method":"tools/list","params":{}}`)))
	nullID.Header.Set("Accept", "application/json, text/event-stream")
	nullIDRes := httptest.NewRecorder()
	server.ServeHTTP(nullIDRes, nullID)
	if nullIDRes.Code != http.StatusOK {
		t.Fatalf("expected null id request to return 200, got %d %s", nullIDRes.Code, nullIDRes.Body.String())
	}
	var nullIDRaw map[string]any
	decodeRecorder(t, nullIDRes, &nullIDRaw)
	if _, ok := nullIDRaw["id"]; !ok || nullIDRaw["id"] != nil {
		t.Fatalf("expected null id response to include id:null, got %#v", nullIDRaw)
	}

	stringID := "agent-request-001"
	stringIDRes := rawRPCRequest(t, server, `{"jsonrpc":"2.0","id":"`+stringID+`","method":"tools/list","params":{}}`)
	if stringIDRes.Code != http.StatusOK {
		t.Fatalf("expected string id request to return 200, got %d %s", stringIDRes.Code, stringIDRes.Body.String())
	}
	var stringIDRaw map[string]any
	decodeRecorder(t, stringIDRes, &stringIDRaw)
	if stringIDRaw["id"] != stringID {
		t.Fatalf("expected string request id to be echoed exactly, got %#v", stringIDRaw)
	}

	largeID := "9007199254740993"
	largeIDRes := rawRPCRequest(t, server, `{"jsonrpc":"2.0","id":`+largeID+`,"method":"tools/list","params":{}}`)
	if largeIDRes.Code != http.StatusOK {
		t.Fatalf("expected large integer id request to return 200, got %d %s", largeIDRes.Code, largeIDRes.Body.String())
	}
	if !bytes.Contains(largeIDRes.Body.Bytes(), []byte(`"id":`+largeID)) {
		t.Fatalf("expected large integer request id to be echoed exactly, got %s", largeIDRes.Body.String())
	}

	for name, payload := range map[string]string{
		"missing jsonrpc": `{"id":1,"method":"tools/list","params":{}}`,
		"wrong jsonrpc":   `{"jsonrpc":"1.0","id":1,"method":"tools/list","params":{}}`,
		"missing method":  `{"jsonrpc":"2.0","id":1,"params":{}}`,
		"batch request":   `[{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}]`,
		"string request":  `"not an object"`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(payload)))
		req.Header.Set("Accept", "application/json, text/event-stream")
		res := httptest.NewRecorder()
		server.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("%s: expected invalid request response to use HTTP 200, got %d", name, res.Code)
		}
		var body response
		decodeRecorder(t, res, &body)
		if body.Error == nil || body.Error.Code != -32600 {
			t.Fatalf("%s: expected invalid request error, got %#v", name, body)
		}
	}

	badParams := rawRPCRequest(t, server, `{"jsonrpc":"2.0","id":"bad-params","method":"resources/read","params":[]}`)
	if badParams.Code != http.StatusOK {
		t.Fatalf("expected invalid params response to use HTTP 200, got %d %s", badParams.Code, badParams.Body.String())
	}
	var badParamsBody response
	decodeRecorder(t, badParams, &badParamsBody)
	if badParamsBody.ID != "bad-params" || badParamsBody.Error == nil || badParamsBody.Error.Code != -32602 {
		t.Fatalf("expected invalid params error with echoed string id, got %#v", badParamsBody)
	}

	notification := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)))
	notification.Header.Set("Accept", "application/json, text/event-stream")
	notificationRes := httptest.NewRecorder()
	server.ServeHTTP(notificationRes, notification)
	if notificationRes.Code != http.StatusAccepted {
		t.Fatalf("expected notification to return 202, got %d %s", notificationRes.Code, notificationRes.Body.String())
	}
	if notificationRes.Body.Len() != 0 {
		t.Fatalf("expected notification response body to be empty, got %q", notificationRes.Body.String())
	}

	for name, payload := range map[string]string{
		"result response":  `{"jsonrpc":"2.0","id":99,"result":{"ok":true}}`,
		"error response":   `{"jsonrpc":"2.0","id":100,"error":{"code":-32000,"message":"client-side failure"}}`,
		"null id response": `{"jsonrpc":"2.0","id":null,"result":{"ok":true}}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(payload)))
		req.Header.Set("Accept", "application/json, text/event-stream")
		res := httptest.NewRecorder()
		server.ServeHTTP(res, req)
		if res.Code != http.StatusAccepted {
			t.Fatalf("%s: expected client JSON-RPC response to return 202, got %d %s", name, res.Code, res.Body.String())
		}
		if res.Body.Len() != 0 {
			t.Fatalf("%s: expected client JSON-RPC response body to be empty, got %q", name, res.Body.String())
		}
	}

	for name, payload := range map[string]string{
		"object request id":       `{"jsonrpc":"2.0","id":{"bad":true},"method":"tools/list","params":{}}`,
		"boolean request id":      `{"jsonrpc":"2.0","id":false,"method":"tools/list","params":{}}`,
		"fractional request id":   `{"jsonrpc":"2.0","id":1.5,"method":"tools/list","params":{}}`,
		"exponent request id":     `{"jsonrpc":"2.0","id":1e3,"method":"tools/list","params":{}}`,
		"request with result":     `{"jsonrpc":"2.0","id":101,"method":"tools/list","result":{"bad":true}}`,
		"response with bad id":    `{"jsonrpc":"2.0","id":{"bad":true},"result":{"ok":true}}`,
		"response fractional id":  `{"jsonrpc":"2.0","id":1.5,"result":{"ok":true}}`,
		"response exponent id":    `{"jsonrpc":"2.0","id":1e3,"result":{"ok":true}}`,
		"response result+error":   `{"jsonrpc":"2.0","id":102,"result":{"ok":true},"error":{"code":-32000,"message":"bad"}}`,
		"notification with error": `{"jsonrpc":"2.0","method":"notifications/initialized","error":{"code":-32000,"message":"bad"}}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(payload)))
		req.Header.Set("Accept", "application/json, text/event-stream")
		res := httptest.NewRecorder()
		server.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("%s: expected invalid JSON-RPC envelope response to use HTTP 200, got %d", name, res.Code)
		}
		var body response
		decodeRecorder(t, res, &body)
		if body.Error == nil || body.Error.Code != -32600 {
			t.Fatalf("%s: expected invalid request error, got %#v", name, body)
		}
	}

	unknownNotification := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"unknown","arguments":{}}}`)))
	unknownNotification.Header.Set("Accept", "application/json, text/event-stream")
	unknownNotificationRes := httptest.NewRecorder()
	server.ServeHTTP(unknownNotificationRes, unknownNotification)
	if unknownNotificationRes.Code != http.StatusAccepted {
		t.Fatalf("expected invalid notification to return 202, got %d %s", unknownNotificationRes.Code, unknownNotificationRes.Body.String())
	}
	if unknownNotificationRes.Body.Len() != 0 {
		t.Fatalf("expected invalid notification response body to be empty, got %q", unknownNotificationRes.Body.String())
	}
}

func TestMCPNonNotificationWithoutIDDoesNotMutate(t *testing.T) {
	server := newTestServer(t)

	createNotification := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{
		"jsonrpc":"2.0",
		"method":"tools/call",
		"params":{
			"name":"issue_create",
			"arguments":{"username":"agent","title":"Invisible notification issue","priority":"P1"}
		}
	}`)))
	createNotification.Header.Set("Accept", "application/json, text/event-stream")
	createNotificationRes := httptest.NewRecorder()
	server.ServeHTTP(createNotificationRes, createNotification)
	if createNotificationRes.Code != http.StatusAccepted {
		t.Fatalf("expected non-notification message without id to return 202, got %d %s", createNotificationRes.Code, createNotificationRes.Body.String())
	}
	if createNotificationRes.Body.Len() != 0 {
		t.Fatalf("expected non-notification message without id response body to be empty, got %q", createNotificationRes.Body.String())
	}

	search := callTool(t, server, "issue_search", map[string]any{"q": "Invisible notification issue"})
	result := search["result"].(map[string]any)
	issues := result["structuredContent"].([]any)
	if len(issues) != 0 {
		t.Fatalf("expected issue_create message without id to be ignored, got %#v", issues)
	}
}

func TestMCPToolCallArgumentsAreOptionalObject(t *testing.T) {
	server := newTestServer(t)

	omittedSearch := rawRPCRequest(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"issue_search"}}`)
	if omittedSearch.Code != http.StatusOK {
		t.Fatalf("expected omitted tool arguments to return 200, got %d %s", omittedSearch.Code, omittedSearch.Body.String())
	}
	var searchBody map[string]any
	decodeRecorder(t, omittedSearch, &searchBody)
	if searchBody["error"] != nil {
		t.Fatalf("expected omitted issue_search arguments to succeed, got %#v", searchBody["error"])
	}
	searchResult := searchBody["result"].(map[string]any)
	if searchResult["isError"] != false {
		t.Fatalf("expected omitted issue_search arguments to be a successful tool result, got %#v", searchResult)
	}
	if _, ok := searchResult["structuredContent"].([]any); !ok {
		t.Fatalf("expected issue_search structured content array, got %#v", searchResult["structuredContent"])
	}

	omittedRequired := rawRPCRequest(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"issue_get"}}`)
	if omittedRequired.Code != http.StatusOK {
		t.Fatalf("expected omitted required arguments to return 200, got %d %s", omittedRequired.Code, omittedRequired.Body.String())
	}
	var requiredBody map[string]any
	decodeRecorder(t, omittedRequired, &requiredBody)
	if requiredBody["error"] != nil {
		t.Fatalf("expected omitted required tool arguments to return a tool error, got rpc error %#v", requiredBody["error"])
	}
	assertToolAppError(t, requiredBody, domain.CodeValidationError, "issue_id")

	nullArguments := rawRPCRequest(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"issue_search","arguments":null}}`)
	if nullArguments.Code != http.StatusOK {
		t.Fatalf("expected null tool arguments to return JSON-RPC error response, got %d %s", nullArguments.Code, nullArguments.Body.String())
	}
	var nullBody response
	decodeRecorder(t, nullArguments, &nullBody)
	if nullBody.Error == nil || nullBody.Error.Code != -32602 {
		t.Fatalf("expected null tool arguments to be invalid params, got %#v", nullBody)
	}
}

func TestMCPRequiredMethodParamsValidation(t *testing.T) {
	server := newTestServer(t)

	for name, payload := range map[string]string{
		"tools/call missing params": `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`,
		"tools/call null params":    `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":null}`,
		"tools/call array params":   `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":[]}`,
		"tools/call missing name":   `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"arguments":{}}}`,
		"tools/call null name":      `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":null,"arguments":{}}}`,
		"resources/read no params":  `{"jsonrpc":"2.0","id":1,"method":"resources/read"}`,
		"resources/read no uri":     `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{}}`,
		"resources/read null uri":   `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":null}}`,
		"resources/read array":      `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":[]}`,
	} {
		res := rawRPCRequest(t, server, payload)
		if res.Code != http.StatusOK {
			t.Fatalf("%s: expected invalid params to return HTTP 200, got %d %s", name, res.Code, res.Body.String())
		}
		var body response
		decodeRecorder(t, res, &body)
		if body.Error == nil || body.Error.Code != -32602 {
			t.Fatalf("%s: expected invalid params error, got %#v", name, body)
		}
	}

	missingResource := rawRPCRequest(t, server, `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"tala://issues/issue_missing"}}`)
	if missingResource.Code != http.StatusOK {
		t.Fatalf("expected missing resource read to return HTTP 200, got %d %s", missingResource.Code, missingResource.Body.String())
	}
	var missingBody response
	decodeRecorder(t, missingResource, &missingBody)
	if missingBody.Error == nil || missingBody.Error.Code != -32002 {
		t.Fatalf("expected non-empty unknown resource URI to remain resource-not-found, got %#v", missingBody)
	}
}

func callTool(t *testing.T, server *Server, name string, args map[string]any) map[string]any {
	t.Helper()
	res := rpcRequest(t, server, "", "tools/call", map[string]any{"name": name, "arguments": args})
	if res.Code != http.StatusOK {
		t.Fatalf("%s failed: %d %s", name, res.Code, res.Body.String())
	}
	var body map[string]any
	decodeRecorder(t, res, &body)
	if body["error"] != nil {
		t.Fatalf("%s returned rpc error: %#v", name, body["error"])
	}
	return body
}

func callToolExpectError(t *testing.T, server *Server, name string, args map[string]any) map[string]any {
	t.Helper()
	res := rpcRequest(t, server, "", "tools/call", map[string]any{"name": name, "arguments": args})
	if res.Code != http.StatusOK {
		t.Fatalf("%s failed: %d %s", name, res.Code, res.Body.String())
	}
	var body map[string]any
	decodeRecorder(t, res, &body)
	if body["error"] == nil {
		t.Fatalf("%s returned success, expected rpc error: %#v", name, body)
	}
	return body
}

func callToolExpectToolError(t *testing.T, server *Server, name string, args map[string]any) map[string]any {
	t.Helper()
	res := rpcRequest(t, server, "", "tools/call", map[string]any{"name": name, "arguments": args})
	if res.Code != http.StatusOK {
		t.Fatalf("%s failed: %d %s", name, res.Code, res.Body.String())
	}
	var body map[string]any
	decodeRecorder(t, res, &body)
	if body["error"] != nil {
		t.Fatalf("%s returned rpc error, expected tool error result: %#v", name, body["error"])
	}
	result, ok := body["result"].(map[string]any)
	if !ok || result["isError"] != true {
		t.Fatalf("%s returned success, expected isError tool result: %#v", name, body)
	}
	return body
}

func assertRPCAppError(t *testing.T, body map[string]any, code domain.ErrorCode, field string) {
	t.Helper()
	errBody := body["error"].(map[string]any)
	data := errBody["data"].(map[string]any)
	if data["code"] != string(code) || data["field"] != field {
		t.Fatalf("expected rpc app error %s/%s, got %#v", code, field, data)
	}
}

func assertToolAppError(t *testing.T, body map[string]any, code domain.ErrorCode, field string) {
	t.Helper()
	result := body["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError:true tool result, got %#v", result)
	}
	content := result["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("expected tool error summary and structured JSON content, got %#v", content)
	}
	data := result["structuredContent"].(map[string]any)
	if data["code"] != string(code) || data["field"] != field {
		t.Fatalf("expected tool app error %s/%s, got %#v", code, field, data)
	}
	var mirrored map[string]any
	if err := json.Unmarshal([]byte(content[1].(map[string]any)["text"].(string)), &mirrored); err != nil {
		t.Fatalf("tool error JSON mirror is not valid JSON: %v", err)
	}
	if mirrored["code"] != string(code) || mirrored["field"] != field {
		t.Fatalf("tool error JSON mirror did not match structured content: text=%#v structured=%#v", mirrored, data)
	}
}

func assertTransportError(t *testing.T, res *httptest.ResponseRecorder, message string) {
	t.Helper()
	if contentType := res.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected JSON transport error response, got %q", contentType)
	}
	var body response
	decodeRecorder(t, res, &body)
	if body.JSONRPC != "2.0" || body.ID != nil || body.Error == nil {
		t.Fatalf("expected JSON-RPC transport error with null id, got %#v", body)
	}
	if body.Error.Code != -32000 || body.Error.Message != message {
		t.Fatalf("expected transport error -32000/%q, got %#v", message, body.Error)
	}
}

func readResource(t *testing.T, server *Server, uri string) map[string]any {
	t.Helper()
	res := rpcRequest(t, server, "", "resources/read", map[string]any{"uri": uri})
	if res.Code != http.StatusOK {
		t.Fatalf("resource read failed: %d %s", res.Code, res.Body.String())
	}
	var body map[string]any
	decodeRecorder(t, res, &body)
	if body["error"] != nil {
		t.Fatalf("resource returned rpc error: %#v", body["error"])
	}
	return body
}

func rpcRequest(t *testing.T, server *Server, origin, method string, params map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	return res
}

func rawRPCRequest(t *testing.T, server *Server, payload string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	return res
}

func structuredID(t *testing.T, body map[string]any) string {
	t.Helper()
	content := structuredIssue(t, body)
	id, ok := content["id"].(string)
	if !ok || id == "" {
		t.Fatalf("missing structured id in %#v", content)
	}
	return id
}

func structuredIssue(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	result := body["result"].(map[string]any)
	content := result["structuredContent"].(map[string]any)
	return content
}

func resourceText(t *testing.T, body map[string]any) string {
	t.Helper()
	result := body["result"].(map[string]any)
	contents := result["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("expected one resource content, got %d", len(contents))
	}
	text, ok := contents[0].(map[string]any)["text"].(string)
	if !ok {
		t.Fatalf("resource text missing in %#v", contents[0])
	}
	return text
}

func decodeRecorder(t *testing.T, res *httptest.ResponseRecorder, dest any) {
	t.Helper()
	if err := json.Unmarshal(res.Body.Bytes(), dest); err != nil {
		t.Fatalf("decode %q: %v", res.Body.String(), err)
	}
}
