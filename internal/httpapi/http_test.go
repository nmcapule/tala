package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tala/internal/app"
	"tala/internal/domain"
	"tala/internal/store"
)

func newTestHandler(t *testing.T) http.Handler {
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
	return New(app.NewService(st), nil).Routes()
}

func newTestHandlerWithStatic(t *testing.T) http.Handler {
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
	static := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<main>static app</main>"))
	})
	return New(app.NewService(st), static).Routes()
}

func newTestHandlerWithUploads(t *testing.T) (http.Handler, string) {
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
	return New(app.NewServiceWithUploadDir(st, uploadDir), nil).Routes(), uploadDir
}

func TestRESTIssueWorkflowAndValidation(t *testing.T) {
	handler := newTestHandler(t)

	missing := doJSON(t, handler, http.MethodPost, "/api/issues", "", map[string]any{"title": "No user"})
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", missing.Code, missing.Body.String())
	}
	var missingBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, missing, &missingBody)
	if missingBody.Error.Code != domain.CodeMissingUsername {
		t.Fatalf("expected missing_username, got %s", missingBody.Error.Code)
	}
	malformedMissingUser := httptest.NewRequest(http.MethodPost, "/api/issues", bytes.NewReader([]byte(`{"title":`)))
	malformedMissingUser.Header.Set("Content-Type", "application/json")
	malformedMissingUserRes := httptest.NewRecorder()
	handler.ServeHTTP(malformedMissingUserRes, malformedMissingUser)
	if malformedMissingUserRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing username to be checked before create body decode, got %d %s", malformedMissingUserRes.Code, malformedMissingUserRes.Body.String())
	}
	nullCreateTags := doJSON(t, handler, http.MethodPost, "/api/issues", "alex", map[string]any{"title": "Null create tags", "tag_names": nil})
	if nullCreateTags.Code != http.StatusBadRequest {
		t.Fatalf("expected create tag_names null validation, got %d %s", nullCreateTags.Code, nullCreateTags.Body.String())
	}
	var nullCreateTagsBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, nullCreateTags, &nullCreateTagsBody)
	if nullCreateTagsBody.Error.Code != domain.CodeValidationError || nullCreateTagsBody.Error.Field != "tag_names" {
		t.Fatalf("expected create tag_names validation error, got %#v", nullCreateTagsBody.Error)
	}
	nullCreatePriority := doJSON(t, handler, http.MethodPost, "/api/issues", "alex", map[string]any{"title": "Null create priority", "priority": nil})
	if nullCreatePriority.Code != http.StatusBadRequest {
		t.Fatalf("expected create priority null validation, got %d %s", nullCreatePriority.Code, nullCreatePriority.Body.String())
	}
	var nullScalarBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, nullCreatePriority, &nullScalarBody)
	if nullScalarBody.Error.Code != domain.CodeValidationError || nullScalarBody.Error.Field != "priority" {
		t.Fatalf("expected create priority validation error, got %#v", nullScalarBody.Error)
	}
	invalidCreateTitle := doJSON(t, handler, http.MethodPost, "/api/issues", "alex", map[string]any{"title": 42})
	if invalidCreateTitle.Code != http.StatusBadRequest {
		t.Fatalf("expected create title type validation, got %d %s", invalidCreateTitle.Code, invalidCreateTitle.Body.String())
	}
	var invalidScalarBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, invalidCreateTitle, &invalidScalarBody)
	if invalidScalarBody.Error.Code != domain.CodeValidationError || invalidScalarBody.Error.Field != "title" {
		t.Fatalf("expected create title validation error, got %#v", invalidScalarBody.Error)
	}
	invalidCreateStoryPoints := doJSON(t, handler, http.MethodPost, "/api/issues", "alex", map[string]any{"title": "Invalid create SP", "story_points": 4})
	if invalidCreateStoryPoints.Code != http.StatusBadRequest {
		t.Fatalf("expected create story_points validation, got %d %s", invalidCreateStoryPoints.Code, invalidCreateStoryPoints.Body.String())
	}
	decodeBody(t, invalidCreateStoryPoints, &invalidScalarBody)
	if invalidScalarBody.Error.Code != domain.CodeValidationError || invalidScalarBody.Error.Field != "story_points" {
		t.Fatalf("expected create story_points validation error, got %#v", invalidScalarBody.Error)
	}

	blocked := createIssue(t, handler, "Blocked issue", "P1", []string{"mcp", "api"})
	blocker := createIssue(t, handler, "Blocker issue", "P2", []string{"frontend"})
	parent := createIssue(t, handler, "Parent issue", "P3", []string{"planning"})

	addBlocker := doJSON(t, handler, http.MethodPost, "/api/issues/"+blocked.ID+"/blockers", "alex", map[string]any{"blocker_issue_id": blocker.ID})
	if addBlocker.Code != http.StatusOK {
		t.Fatalf("add blocker failed: %d %s", addBlocker.Code, addBlocker.Body.String())
	}
	setParent := doJSON(t, handler, http.MethodPut, "/api/issues/"+blocked.ID+"/parent", "alex", map[string]any{"parent_issue_id": parent.ID})
	if setParent.Code != http.StatusOK {
		t.Fatalf("set parent failed: %d %s", setParent.Code, setParent.Body.String())
	}
	omittedParent := doJSON(t, handler, http.MethodPut, "/api/issues/"+blocked.ID+"/parent", "alex", map[string]any{})
	if omittedParent.Code != http.StatusBadRequest {
		t.Fatalf("expected omitted parent_issue_id validation, got %d: %s", omittedParent.Code, omittedParent.Body.String())
	}
	var parentError struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, omittedParent, &parentError)
	if parentError.Error.Code != domain.CodeValidationError || parentError.Error.Field != "parent_issue_id" {
		t.Fatalf("expected parent_issue_id validation error, got %#v", parentError.Error)
	}
	invalidParent := doJSON(t, handler, http.MethodPut, "/api/issues/"+blocked.ID+"/parent", "alex", map[string]any{"parent_issue_id": 42})
	if invalidParent.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid parent_issue_id validation, got %d: %s", invalidParent.Code, invalidParent.Body.String())
	}
	decodeBody(t, invalidParent, &parentError)
	if parentError.Error.Code != domain.CodeValidationError || parentError.Error.Field != "parent_issue_id" {
		t.Fatalf("expected invalid parent_issue_id validation error, got %#v", parentError.Error)
	}
	cycle := doJSON(t, handler, http.MethodPost, "/api/issues/"+blocker.ID+"/blockers", "alex", map[string]any{"blocker_issue_id": blocked.ID})
	if cycle.Code != http.StatusConflict {
		t.Fatalf("expected cycle conflict, got %d: %s", cycle.Code, cycle.Body.String())
	}
	missingBlocker := doJSON(t, handler, http.MethodPost, "/api/issues/"+blocked.ID+"/blockers", "alex", map[string]any{})
	if missingBlocker.Code != http.StatusBadRequest {
		t.Fatalf("expected missing blocker validation, got %d: %s", missingBlocker.Code, missingBlocker.Body.String())
	}
	var blockerError struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, missingBlocker, &blockerError)
	if blockerError.Error.Code != domain.CodeValidationError || blockerError.Error.Field != "blocker_issue_id" {
		t.Fatalf("expected blocker_issue_id validation error, got %#v", blockerError.Error)
	}
	nullBlocker := doJSON(t, handler, http.MethodPost, "/api/issues/"+blocked.ID+"/blockers", "alex", map[string]any{"blocker_issue_id": nil})
	if nullBlocker.Code != http.StatusBadRequest {
		t.Fatalf("expected null blocker validation, got %d: %s", nullBlocker.Code, nullBlocker.Body.String())
	}
	decodeBody(t, nullBlocker, &blockerError)
	if blockerError.Error.Code != domain.CodeValidationError || blockerError.Error.Field != "blocker_issue_id" {
		t.Fatalf("expected null blocker_issue_id validation error, got %#v", blockerError.Error)
	}
	unknownBlocker := doJSON(t, handler, http.MethodPost, "/api/issues/"+blocked.ID+"/blockers", "alex", map[string]any{"blocker_issue_id": "issue_missing"})
	if unknownBlocker.Code != http.StatusNotFound {
		t.Fatalf("expected unknown blocker not_found, got %d: %s", unknownBlocker.Code, unknownBlocker.Body.String())
	}
	decodeBody(t, unknownBlocker, &blockerError)
	if blockerError.Error.Code != domain.CodeNotFound || blockerError.Error.Field != "blocker_issue_id" {
		t.Fatalf("expected blocker_issue_id not_found error, got %#v", blockerError.Error)
	}

	comment := doJSON(t, handler, http.MethodPost, "/api/issues/"+blocked.ID+"/comments", "sam", map[string]any{"body_markdown": "Stored **as Markdown**."})
	if comment.Code != http.StatusCreated {
		t.Fatalf("comment failed: %d %s", comment.Code, comment.Body.String())
	}
	nullComment := doJSON(t, handler, http.MethodPost, "/api/issues/"+blocked.ID+"/comments", "sam", map[string]any{"body_markdown": nil})
	if nullComment.Code != http.StatusBadRequest {
		t.Fatalf("expected null comment validation, got %d %s", nullComment.Code, nullComment.Body.String())
	}
	var nullCommentBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, nullComment, &nullCommentBody)
	if nullCommentBody.Error.Code != domain.CodeValidationError || nullCommentBody.Error.Field != "body_markdown" {
		t.Fatalf("expected body_markdown validation error, got %#v", nullCommentBody.Error)
	}
	invalidComment := doJSON(t, handler, http.MethodPost, "/api/issues/"+blocked.ID+"/comments", "sam", map[string]any{"body_markdown": 42})
	if invalidComment.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid comment validation, got %d %s", invalidComment.Code, invalidComment.Body.String())
	}
	var invalidCommentBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, invalidComment, &invalidCommentBody)
	if invalidCommentBody.Error.Code != domain.CodeValidationError || invalidCommentBody.Error.Field != "body_markdown" {
		t.Fatalf("expected body_markdown type validation error, got %#v", invalidCommentBody.Error)
	}
	missingCommentBody := doJSON(t, handler, http.MethodPost, "/api/issues/"+blocked.ID+"/comments", "sam", map[string]any{})
	if missingCommentBody.Code != http.StatusBadRequest {
		t.Fatalf("expected missing comment body validation, got %d %s", missingCommentBody.Code, missingCommentBody.Body.String())
	}
	var missingCommentBodyError struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, missingCommentBody, &missingCommentBodyError)
	if missingCommentBodyError.Error.Code != domain.CodeValidationError || missingCommentBodyError.Error.Field != "body_markdown" {
		t.Fatalf("expected missing body_markdown validation error, got %#v", missingCommentBodyError.Error)
	}
	missingCommentUser := doJSON(t, handler, http.MethodPost, "/api/issues/"+blocked.ID+"/comments", "", map[string]any{"body_markdown": "No user."})
	if missingCommentUser.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing comment username to return 401, got %d %s", missingCommentUser.Code, missingCommentUser.Body.String())
	}
	decodeBody(t, missingCommentUser, &missingBody)
	if missingBody.Error.Code != domain.CodeMissingUsername || missingBody.Error.Field != "username" {
		t.Fatalf("expected missing comment username error, got %#v", missingBody.Error)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/issues/"+blocked.ID, nil)
	detail := httptest.NewRecorder()
	handler.ServeHTTP(detail, detailReq)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail failed: %d %s", detail.Code, detail.Body.String())
	}
	var got domain.Issue
	decodeBody(t, detail, &got)
	if !got.Blocked || len(got.Blockers) != 1 || len(got.RecentComments) != 1 {
		t.Fatalf("expected hydrated blocked issue, got blocked=%v blockers=%d comments=%d", got.Blocked, len(got.Blockers), len(got.RecentComments))
	}

	filterReq := httptest.NewRequest(http.MethodGet, "/api/issues?tag=mcp&priority=P1&q=blocked", nil)
	filtered := httptest.NewRecorder()
	handler.ServeHTTP(filtered, filterReq)
	if filtered.Code != http.StatusOK {
		t.Fatalf("filter failed: %d %s", filtered.Code, filtered.Body.String())
	}
	var list []domain.Issue
	decodeBody(t, filtered, &list)
	if len(list) != 1 || list[0].ID != blocked.ID {
		t.Fatalf("expected filtered blocked issue, got %#v", list)
	}

	setAssignee := doJSON(t, handler, http.MethodPatch, "/api/issues/"+blocked.ID, "alex", map[string]any{"assignee": "sam", "status": "in_progress"})
	if setAssignee.Code != http.StatusOK {
		t.Fatalf("set assignee failed: %d %s", setAssignee.Code, setAssignee.Body.String())
	}
	var assigned domain.Issue
	decodeBody(t, setAssignee, &assigned)
	if assigned.Assignee == nil || *assigned.Assignee != "sam" {
		t.Fatalf("expected assignee to be set, got %#v", assigned.Assignee)
	}
	setStoryPoints := doJSON(t, handler, http.MethodPatch, "/api/issues/"+blocked.ID, "alex", map[string]any{"story_points": 8})
	if setStoryPoints.Code != http.StatusOK {
		t.Fatalf("expected REST to allow 8SP direct estimate, got %d %s", setStoryPoints.Code, setStoryPoints.Body.String())
	}
	decodeBody(t, setStoryPoints, &assigned)
	if assigned.StoryPoints == nil || *assigned.StoryPoints != 8 || assigned.StoryPointsTotal != 8 {
		t.Fatalf("expected 8SP direct and total from REST update, got direct=%v total=%d", assigned.StoryPoints, assigned.StoryPointsTotal)
	}
	invalidStoryPoints := doJSON(t, handler, http.MethodPatch, "/api/issues/"+blocked.ID, "alex", map[string]any{"story_points": 4})
	if invalidStoryPoints.Code != http.StatusBadRequest {
		t.Fatalf("expected update story_points validation, got %d %s", invalidStoryPoints.Code, invalidStoryPoints.Body.String())
	}
	decodeBody(t, invalidStoryPoints, &invalidScalarBody)
	if invalidScalarBody.Error.Code != domain.CodeValidationError || invalidScalarBody.Error.Field != "story_points" {
		t.Fatalf("expected update story_points validation error, got %#v", invalidScalarBody.Error)
	}
	clearStoryPoints := doJSON(t, handler, http.MethodPatch, "/api/issues/"+blocked.ID, "alex", map[string]any{"story_points": nil})
	if clearStoryPoints.Code != http.StatusOK {
		t.Fatalf("clear story_points failed: %d %s", clearStoryPoints.Code, clearStoryPoints.Body.String())
	}
	decodeBody(t, clearStoryPoints, &assigned)
	if assigned.StoryPoints != nil || assigned.StoryPointsTotal != 0 {
		t.Fatalf("expected story_points to clear, got direct=%v total=%d", assigned.StoryPoints, assigned.StoryPointsTotal)
	}
	assertIssueFilter(t, handler, "/api/issues?status=in_progress", blocked.ID)
	assertIssueFilter(t, handler, "/api/issues?assignee=sam", blocked.ID)
	assertIssueFilter(t, handler, "/api/issues?id="+blocked.ID, blocked.ID)
	assertIssueFilter(t, handler, "/api/issues?parent_id="+parent.ID, blocked.ID)
	assertIssueFilter(t, handler, "/api/issues?blocked_by="+blocker.ID, blocked.ID)
	assertIssueFilter(t, handler, "/api/issues?blocker_of="+blocked.ID, blocker.ID)
	assertIssueFilter(t, handler, "/api/issues?state=blocked", blocked.ID)
	assertIssueFilter(t, handler, "/api/issues?state=open&q=Blocked&sort=title&order=desc", blocked.ID)
	assertIssueFilter(t, handler, "/api/issues?q=Stored+%2A%2Aas+Markdown%2A%2A", blocked.ID)
	assertIssueFilter(t, handler, "/api/issues?q=api", blocked.ID)
	assertIssueFilter(t, handler, "/api/issues?status=+in_progress+&priority=+P1+&assignee=+sam+&tag=+mcp+&parent_id=+"+parent.ID+"+&blocked_by=+"+blocker.ID+"+&q=+Blocked+", blocked.ID)
	missingParentFilter := httptest.NewRequest(http.MethodGet, "/api/issues?parent_id=issue_missing_parent", nil)
	missingParentFilterRes := httptest.NewRecorder()
	handler.ServeHTTP(missingParentFilterRes, missingParentFilter)
	if missingParentFilterRes.Code != http.StatusNotFound {
		t.Fatalf("expected missing parent filter not_found, got %d: %s", missingParentFilterRes.Code, missingParentFilterRes.Body.String())
	}
	var filterError struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, missingParentFilterRes, &filterError)
	if filterError.Error.Code != domain.CodeNotFound || filterError.Error.Field != "parent_id" {
		t.Fatalf("expected parent_id not_found filter error, got %#v", filterError.Error)
	}
	missingBlockedByFilter := httptest.NewRequest(http.MethodGet, "/api/issues?blocked_by=issue_missing_blocker", nil)
	missingBlockedByFilterRes := httptest.NewRecorder()
	handler.ServeHTTP(missingBlockedByFilterRes, missingBlockedByFilter)
	if missingBlockedByFilterRes.Code != http.StatusNotFound {
		t.Fatalf("expected missing blocked_by filter not_found, got %d: %s", missingBlockedByFilterRes.Code, missingBlockedByFilterRes.Body.String())
	}
	decodeBody(t, missingBlockedByFilterRes, &filterError)
	if filterError.Error.Code != domain.CodeNotFound || filterError.Error.Field != "blocked_by" {
		t.Fatalf("expected blocked_by not_found filter error, got %#v", filterError.Error)
	}
	missingIDFilter := httptest.NewRequest(http.MethodGet, "/api/issues?id=issue_missing", nil)
	missingIDFilterRes := httptest.NewRecorder()
	handler.ServeHTTP(missingIDFilterRes, missingIDFilter)
	if missingIDFilterRes.Code != http.StatusNotFound {
		t.Fatalf("expected missing id filter not_found, got %d: %s", missingIDFilterRes.Code, missingIDFilterRes.Body.String())
	}
	decodeBody(t, missingIDFilterRes, &filterError)
	if filterError.Error.Code != domain.CodeNotFound || filterError.Error.Field != "id" {
		t.Fatalf("expected id not_found filter error, got %#v", filterError.Error)
	}
	missingBlockerOfFilter := httptest.NewRequest(http.MethodGet, "/api/issues?blocker_of=issue_missing_blocked", nil)
	missingBlockerOfFilterRes := httptest.NewRecorder()
	handler.ServeHTTP(missingBlockerOfFilterRes, missingBlockerOfFilter)
	if missingBlockerOfFilterRes.Code != http.StatusNotFound {
		t.Fatalf("expected missing blocker_of filter not_found, got %d: %s", missingBlockerOfFilterRes.Code, missingBlockerOfFilterRes.Body.String())
	}
	decodeBody(t, missingBlockerOfFilterRes, &filterError)
	if filterError.Error.Code != domain.CodeNotFound || filterError.Error.Field != "blocker_of" {
		t.Fatalf("expected blocker_of not_found filter error, got %#v", filterError.Error)
	}
	invalidStatusFilter := httptest.NewRequest(http.MethodGet, "/api/issues?status=shipped", nil)
	invalidStatusFilterRes := httptest.NewRecorder()
	handler.ServeHTTP(invalidStatusFilterRes, invalidStatusFilter)
	if invalidStatusFilterRes.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid status filter validation, got %d: %s", invalidStatusFilterRes.Code, invalidStatusFilterRes.Body.String())
	}
	decodeBody(t, invalidStatusFilterRes, &filterError)
	if filterError.Error.Code != domain.CodeValidationError || filterError.Error.Field != "status" {
		t.Fatalf("expected status validation filter error, got %#v", filterError.Error)
	}
	invalidPriorityFilter := httptest.NewRequest(http.MethodGet, "/api/issues?priority=P9", nil)
	invalidPriorityFilterRes := httptest.NewRecorder()
	handler.ServeHTTP(invalidPriorityFilterRes, invalidPriorityFilter)
	if invalidPriorityFilterRes.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid priority filter validation, got %d: %s", invalidPriorityFilterRes.Code, invalidPriorityFilterRes.Body.String())
	}
	decodeBody(t, invalidPriorityFilterRes, &filterError)
	if filterError.Error.Code != domain.CodeValidationError || filterError.Error.Field != "priority" {
		t.Fatalf("expected priority validation filter error, got %#v", filterError.Error)
	}
	invalidStateFilter := httptest.NewRequest(http.MethodGet, "/api/issues?state=waiting", nil)
	invalidStateFilterRes := httptest.NewRecorder()
	handler.ServeHTTP(invalidStateFilterRes, invalidStateFilter)
	if invalidStateFilterRes.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid state filter validation, got %d: %s", invalidStateFilterRes.Code, invalidStateFilterRes.Body.String())
	}
	decodeBody(t, invalidStateFilterRes, &filterError)
	if filterError.Error.Code != domain.CodeValidationError || filterError.Error.Field != "state" {
		t.Fatalf("expected state validation filter error, got %#v", filterError.Error)
	}
	invalidSortFilter := httptest.NewRequest(http.MethodGet, "/api/issues?sort=rank", nil)
	invalidSortFilterRes := httptest.NewRecorder()
	handler.ServeHTTP(invalidSortFilterRes, invalidSortFilter)
	if invalidSortFilterRes.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid sort filter validation, got %d: %s", invalidSortFilterRes.Code, invalidSortFilterRes.Body.String())
	}
	decodeBody(t, invalidSortFilterRes, &filterError)
	if filterError.Error.Code != domain.CodeValidationError || filterError.Error.Field != "sort" {
		t.Fatalf("expected sort validation filter error, got %#v", filterError.Error)
	}
	invalidOrderFilter := httptest.NewRequest(http.MethodGet, "/api/issues?order=reverse", nil)
	invalidOrderFilterRes := httptest.NewRecorder()
	handler.ServeHTTP(invalidOrderFilterRes, invalidOrderFilter)
	if invalidOrderFilterRes.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid order filter validation, got %d: %s", invalidOrderFilterRes.Code, invalidOrderFilterRes.Body.String())
	}
	decodeBody(t, invalidOrderFilterRes, &filterError)
	if filterError.Error.Code != domain.CodeValidationError || filterError.Error.Field != "order" {
		t.Fatalf("expected order validation filter error, got %#v", filterError.Error)
	}

	clearAssignee := doJSON(t, handler, http.MethodPatch, "/api/issues/"+blocked.ID, "alex", map[string]any{"assignee": nil})
	if clearAssignee.Code != http.StatusOK {
		t.Fatalf("clear assignee failed: %d %s", clearAssignee.Code, clearAssignee.Body.String())
	}
	var cleared domain.Issue
	decodeBody(t, clearAssignee, &cleared)
	if cleared.Assignee != nil {
		t.Fatalf("expected assignee to clear, got %q", *cleared.Assignee)
	}
	renameWithTags := doJSON(t, handler, http.MethodPatch, "/api/issues/"+blocked.ID, "alex", map[string]any{"title": "Blocked issue renamed"})
	if renameWithTags.Code != http.StatusOK {
		t.Fatalf("rename with tags failed: %d %s", renameWithTags.Code, renameWithTags.Body.String())
	}
	var renamedWithTags domain.Issue
	decodeBody(t, renameWithTags, &renamedWithTags)
	if len(renamedWithTags.Tags) != 2 {
		t.Fatalf("expected omitted tag_names to preserve existing tags, got %#v", renamedWithTags.Tags)
	}
	nullTags := doJSON(t, handler, http.MethodPatch, "/api/issues/"+blocked.ID, "alex", map[string]any{"tag_names": nil})
	if nullTags.Code != http.StatusBadRequest {
		t.Fatalf("expected tag_names null validation, got %d %s", nullTags.Code, nullTags.Body.String())
	}
	var nullTagsBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, nullTags, &nullTagsBody)
	if nullTagsBody.Error.Code != domain.CodeValidationError || nullTagsBody.Error.Field != "tag_names" {
		t.Fatalf("expected tag_names validation error, got %#v", nullTagsBody.Error)
	}
	nullStatus := doJSON(t, handler, http.MethodPatch, "/api/issues/"+blocked.ID, "alex", map[string]any{"status": nil})
	if nullStatus.Code != http.StatusBadRequest {
		t.Fatalf("expected status null validation, got %d %s", nullStatus.Code, nullStatus.Body.String())
	}
	decodeBody(t, nullStatus, &nullScalarBody)
	if nullScalarBody.Error.Code != domain.CodeValidationError || nullScalarBody.Error.Field != "status" {
		t.Fatalf("expected status validation error, got %#v", nullScalarBody.Error)
	}
	invalidAssignee := doJSON(t, handler, http.MethodPatch, "/api/issues/"+blocked.ID, "alex", map[string]any{"assignee": 42})
	if invalidAssignee.Code != http.StatusBadRequest {
		t.Fatalf("expected assignee type validation, got %d %s", invalidAssignee.Code, invalidAssignee.Body.String())
	}
	decodeBody(t, invalidAssignee, &invalidScalarBody)
	if invalidScalarBody.Error.Code != domain.CodeValidationError || invalidScalarBody.Error.Field != "assignee" {
		t.Fatalf("expected assignee validation error, got %#v", invalidScalarBody.Error)
	}
	clearTags := doJSON(t, handler, http.MethodPatch, "/api/issues/"+blocked.ID, "alex", map[string]any{"tag_names": []string{}})
	if clearTags.Code != http.StatusOK {
		t.Fatalf("clear tags failed: %d %s", clearTags.Code, clearTags.Body.String())
	}
	var withoutTags domain.Issue
	decodeBody(t, clearTags, &withoutTags)
	if len(withoutTags.Tags) != 0 {
		t.Fatalf("expected empty tag_names array to clear tags, got %#v", withoutTags.Tags)
	}
	renameOnly := doJSON(t, handler, http.MethodPatch, "/api/issues/"+blocked.ID, "alex", map[string]any{"title": "Blocked issue renamed again"})
	if renameOnly.Code != http.StatusOK {
		t.Fatalf("rename only failed: %d %s", renameOnly.Code, renameOnly.Body.String())
	}
	var renamed domain.Issue
	decodeBody(t, renameOnly, &renamed)
	if len(renamed.Tags) != 0 {
		t.Fatalf("expected omitted tag_names to preserve current empty tag set, got %#v", renamed.Tags)
	}

	remove := httptest.NewRequest(http.MethodDelete, "/api/issues/"+blocked.ID+"/blockers/"+blocker.ID, nil)
	remove.Header.Set("X-Tala-Username", "alex")
	removeRes := httptest.NewRecorder()
	handler.ServeHTTP(removeRes, remove)
	if removeRes.Code != http.StatusOK {
		t.Fatalf("remove blocker failed: %d %s", removeRes.Code, removeRes.Body.String())
	}
	clearParent := doJSON(t, handler, http.MethodPut, "/api/issues/"+blocked.ID+"/parent", "alex", map[string]any{"parent_issue_id": nil})
	if clearParent.Code != http.StatusOK {
		t.Fatalf("clear parent failed: %d %s", clearParent.Code, clearParent.Body.String())
	}
	var clearedParent domain.Issue
	decodeBody(t, clearParent, &clearedParent)
	if clearedParent.ParentIssueID != nil {
		t.Fatalf("expected parent to clear, got %q", *clearedParent.ParentIssueID)
	}
}

func TestRESTValidationMatrix(t *testing.T) {
	handler := newTestHandler(t)

	parent := createIssue(t, handler, "Matrix parent", "P2", nil)
	issue := createIssue(t, handler, "Matrix child", "P2", nil)
	blocker := createIssue(t, handler, "Matrix blocker", "P2", nil)

	for _, tt := range []struct {
		name     string
		method   string
		path     string
		username string
		body     any
		field    string
	}{
		{name: "create missing title", method: http.MethodPost, path: "/api/issues", username: "alex", body: map[string]any{"priority": "P2"}, field: "title"},
		{name: "create whitespace title", method: http.MethodPost, path: "/api/issues", username: "alex", body: map[string]any{"title": "   ", "priority": "P2"}, field: "title"},
		{name: "create wrong parent type", method: http.MethodPost, path: "/api/issues", username: "alex", body: map[string]any{"title": "Bad parent type", "parent_issue_id": 42}, field: "parent_issue_id"},
		{name: "create wrong tag array type", method: http.MethodPost, path: "/api/issues", username: "alex", body: map[string]any{"title": "Bad tags", "tag_names": "api"}, field: "tag_names"},
		{name: "update whitespace title", method: http.MethodPatch, path: "/api/issues/" + issue.ID, username: "alex", body: map[string]any{"title": "   "}, field: "title"},
		{name: "update wrong priority type", method: http.MethodPatch, path: "/api/issues/" + issue.ID, username: "alex", body: map[string]any{"priority": 2}, field: "priority"},
		{name: "update wrong tag element type", method: http.MethodPatch, path: "/api/issues/" + issue.ID, username: "alex", body: map[string]any{"tag_names": []any{"api", 42}}, field: "tag_names"},
		{name: "comment whitespace body", method: http.MethodPost, path: "/api/issues/" + issue.ID + "/comments", username: "alex", body: map[string]any{"body_markdown": "   "}, field: "body_markdown"},
		{name: "tag create missing name", method: http.MethodPost, path: "/api/tags", username: "alex", body: map[string]any{"color": "#b5f4d8"}, field: "name"},
		{name: "tag create whitespace name", method: http.MethodPost, path: "/api/tags", username: "alex", body: map[string]any{"name": "   "}, field: "name"},
		{name: "tag update whitespace name", method: http.MethodPatch, path: "/api/tags/" + createTag(t, handler, "matrix-tag").ID, username: "alex", body: map[string]any{"name": "   "}, field: "name"},
		{name: "parent missing field", method: http.MethodPut, path: "/api/issues/" + issue.ID + "/parent", username: "alex", body: map[string]any{}, field: "parent_issue_id"},
		{name: "blocker whitespace id", method: http.MethodPost, path: "/api/issues/" + issue.ID + "/blockers", username: "alex", body: map[string]any{"blocker_issue_id": "   "}, field: "blocker_issue_id"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := doJSON(t, handler, tt.method, tt.path, tt.username, tt.body)
			assertRESTError(t, res, http.StatusBadRequest, domain.CodeValidationError, tt.field)
		})
	}

	setParent := doJSON(t, handler, http.MethodPut, "/api/issues/"+issue.ID+"/parent", "alex", map[string]any{"parent_issue_id": " " + parent.ID + " "})
	if setParent.Code != http.StatusOK {
		t.Fatalf("expected whitespace parent id to be trimmed, got %d %s", setParent.Code, setParent.Body.String())
	}
	clearParent := doJSON(t, handler, http.MethodPut, "/api/issues/"+issue.ID+"/parent", "alex", map[string]any{"parent_issue_id": "   "})
	if clearParent.Code != http.StatusOK {
		t.Fatalf("expected whitespace parent id to clear parent, got %d %s", clearParent.Code, clearParent.Body.String())
	}
	addBlocker := doJSON(t, handler, http.MethodPost, "/api/issues/"+issue.ID+"/blockers", "alex", map[string]any{"blocker_issue_id": " " + blocker.ID + " "})
	if addBlocker.Code != http.StatusOK {
		t.Fatalf("expected whitespace blocker id to be trimmed, got %d %s", addBlocker.Code, addBlocker.Body.String())
	}
	assertIssueFilter(t, handler, "/api/issues?status=+new+&priority=+P2+&parent_id=+&blocked_by=+"+blocker.ID+"+&sort=+title+&order=+asc+", issue.ID)
}

func TestRESTErrorResponsesStayStructuredAndSanitized(t *testing.T) {
	appErr := domain.NewError(domain.CodeValidationError, "Stable validation message.", "title")
	wrapped := fmt.Errorf("repository wrapper should not leak: %w", appErr)
	appRes := httptest.NewRecorder()
	writeError(appRes, wrapped)
	assertRESTError(t, appRes, http.StatusBadRequest, domain.CodeValidationError, "title")
	var appBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, appRes, &appBody)
	if appBody.Error.Message != appErr.Message || strings.Contains(appBody.Error.Message, "repository wrapper") {
		t.Fatalf("REST app error leaked wrapper text: %#v", appBody.Error)
	}

	internalRes := httptest.NewRecorder()
	writeError(internalRes, fmt.Errorf("driver secret detail"))
	assertRESTError(t, internalRes, http.StatusInternalServerError, domain.CodeInternal, "")
	var internalBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, internalRes, &internalBody)
	if internalBody.Error.Message != "Internal server error." || strings.Contains(internalBody.Error.Message, "driver secret") {
		t.Fatalf("REST internal error was not sanitized: %#v", internalBody.Error)
	}
}

func TestRESTImageUploadAndServing(t *testing.T) {
	handler, uploadDir := newTestHandlerWithUploads(t)

	upload := doMultipartImage(t, handler, "alex", "screenshot.png", []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	})
	if upload.Code != http.StatusCreated {
		t.Fatalf("upload failed: %d %s", upload.Code, upload.Body.String())
	}
	var uploaded domain.UploadedImage
	decodeBody(t, upload, &uploaded)
	if uploaded.URL == "" || uploaded.Filename == "" || uploaded.ContentType != "image/png" || uploaded.Size == 0 || !strings.Contains(uploaded.Markdown, uploaded.URL) {
		t.Fatalf("unexpected upload response: %#v", uploaded)
	}
	if !strings.HasPrefix(uploaded.URL, "/uploads/images/") {
		t.Fatalf("unexpected upload URL: %q", uploaded.URL)
	}
	if !strings.HasPrefix(filepath.Clean(filepath.Join(uploadDir, uploaded.Filename)), filepath.Clean(uploadDir)) {
		t.Fatalf("uploaded filename escaped upload dir: %q", uploaded.Filename)
	}

	req := httptest.NewRequest(http.MethodGet, uploaded.URL, nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("serve upload failed: %d %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("expected image/png content type, got %q", got)
	}
	if got := res.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff, got %q", got)
	}
}

func TestRESTImageUploadValidation(t *testing.T) {
	handler, _ := newTestHandlerWithUploads(t)

	missingUser := doMultipartImage(t, handler, "", "screenshot.png", []byte{0x89, 0x50, 0x4e, 0x47})
	if missingUser.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing username, got %d %s", missingUser.Code, missingUser.Body.String())
	}

	invalid := doMultipartImage(t, handler, "alex", "notes.txt", []byte("not an image"))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid image rejection, got %d %s", invalid.Code, invalid.Body.String())
	}
	var invalidBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, invalid, &invalidBody)
	if invalidBody.Error.Code != domain.CodeValidationError || invalidBody.Error.Field != "image" {
		t.Fatalf("expected image validation error, got %#v", invalidBody.Error)
	}

	missingReq := httptest.NewRequest(http.MethodPost, "/api/uploads/images", strings.NewReader(""))
	missingReq.Header.Set("X-Tala-Username", "alex")
	missingReq.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	missingRes := httptest.NewRecorder()
	handler.ServeHTTP(missingRes, missingReq)
	if missingRes.Code != http.StatusBadRequest {
		t.Fatalf("expected missing file rejection, got %d %s", missingRes.Code, missingRes.Body.String())
	}

	traversal := httptest.NewRequest(http.MethodGet, "/uploads/images/../tala.db", nil)
	traversalRes := httptest.NewRecorder()
	handler.ServeHTTP(traversalRes, traversal)
	if traversalRes.Code != http.StatusNotFound {
		t.Fatalf("expected traversal request to return 404, got %d %s", traversalRes.Code, traversalRes.Body.String())
	}
}

func TestRESTTagColorCanBeCleared(t *testing.T) {
	handler := newTestHandler(t)

	nullCreateName := doJSON(t, handler, http.MethodPost, "/api/tags", "alex", map[string]any{"name": nil})
	if nullCreateName.Code != http.StatusBadRequest {
		t.Fatalf("expected create tag null name validation, got %d %s", nullCreateName.Code, nullCreateName.Body.String())
	}
	var nullCreateNameBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, nullCreateName, &nullCreateNameBody)
	if nullCreateNameBody.Error.Code != domain.CodeValidationError || nullCreateNameBody.Error.Field != "name" {
		t.Fatalf("expected create tag null name validation error, got %#v", nullCreateNameBody.Error)
	}
	invalidCreateColor := doJSON(t, handler, http.MethodPost, "/api/tags", "alex", map[string]any{"name": "bad color", "color": 42})
	if invalidCreateColor.Code != http.StatusBadRequest {
		t.Fatalf("expected create tag invalid color validation, got %d %s", invalidCreateColor.Code, invalidCreateColor.Body.String())
	}
	var invalidCreateColorBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, invalidCreateColor, &invalidCreateColorBody)
	if invalidCreateColorBody.Error.Code != domain.CodeValidationError || invalidCreateColorBody.Error.Field != "color" {
		t.Fatalf("expected create tag invalid color validation error, got %#v", invalidCreateColorBody.Error)
	}
	invalidCreateColorValue := doJSON(t, handler, http.MethodPost, "/api/tags", "alex", map[string]any{"name": "bad color value", "color": "very-red"})
	if invalidCreateColorValue.Code != http.StatusBadRequest {
		t.Fatalf("expected create tag invalid color value validation, got %d %s", invalidCreateColorValue.Code, invalidCreateColorValue.Body.String())
	}
	decodeBody(t, invalidCreateColorValue, &invalidCreateColorBody)
	if invalidCreateColorBody.Error.Code != domain.CodeValidationError || invalidCreateColorBody.Error.Field != "color" {
		t.Fatalf("expected create tag color validation error, got %#v", invalidCreateColorBody.Error)
	}
	tokenCreate := doJSON(t, handler, http.MethodPost, "/api/tags", "alex", map[string]any{"name": "token tag", "color": "secondary-container"})
	if tokenCreate.Code != http.StatusCreated {
		t.Fatalf("expected token tag color to be accepted, got %d %s", tokenCreate.Code, tokenCreate.Body.String())
	}
	var tokenTag domain.Tag
	decodeBody(t, tokenCreate, &tokenTag)
	if tokenTag.Color == nil || *tokenTag.Color != "secondary-container" {
		t.Fatalf("expected token color to be preserved, got %#v", tokenTag.Color)
	}

	create := doJSON(t, handler, http.MethodPost, "/api/tags", "alex", map[string]any{"name": "docs", "color": "#b5f4d8"})
	if create.Code != http.StatusCreated {
		t.Fatalf("create tag failed: %d %s", create.Code, create.Body.String())
	}
	var tag domain.Tag
	decodeBody(t, create, &tag)
	if tag.Color == nil {
		t.Fatal("expected initial tag color")
	}

	update := doJSON(t, handler, http.MethodPatch, "/api/tags/"+tag.ID, "alex", map[string]any{"name": "documentation", "color": nil})
	if update.Code != http.StatusOK {
		t.Fatalf("update tag failed: %d %s", update.Code, update.Body.String())
	}
	decodeBody(t, update, &tag)
	if tag.Name != "documentation" || tag.Color != nil {
		t.Fatalf("expected renamed tag with cleared color, got %#v", tag)
	}
	nullName := doJSON(t, handler, http.MethodPatch, "/api/tags/"+tag.ID, "alex", map[string]any{"name": nil})
	if nullName.Code != http.StatusBadRequest {
		t.Fatalf("expected null tag name validation, got %d %s", nullName.Code, nullName.Body.String())
	}
	var nullNameBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, nullName, &nullNameBody)
	if nullNameBody.Error.Code != domain.CodeValidationError || nullNameBody.Error.Field != "name" {
		t.Fatalf("expected null tag name validation error, got %#v", nullNameBody.Error)
	}
	invalidName := doJSON(t, handler, http.MethodPatch, "/api/tags/"+tag.ID, "alex", map[string]any{"name": 42})
	if invalidName.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid tag name validation, got %d %s", invalidName.Code, invalidName.Body.String())
	}
	var invalidNameBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, invalidName, &invalidNameBody)
	if invalidNameBody.Error.Code != domain.CodeValidationError || invalidNameBody.Error.Field != "name" {
		t.Fatalf("expected invalid tag name validation error, got %#v", invalidNameBody.Error)
	}
	trimColor := doJSON(t, handler, http.MethodPatch, "/api/tags/"+tag.ID, "alex", map[string]any{"color": "  #ffd7bd  "})
	if trimColor.Code != http.StatusOK {
		t.Fatalf("trim color failed: %d %s", trimColor.Code, trimColor.Body.String())
	}
	decodeBody(t, trimColor, &tag)
	if tag.Color == nil || *tag.Color != "#ffd7bd" {
		t.Fatalf("expected color to be trimmed, got %#v", tag.Color)
	}
	tokenColor := doJSON(t, handler, http.MethodPatch, "/api/tags/"+tag.ID, "alex", map[string]any{"color": " tertiary-container "})
	if tokenColor.Code != http.StatusOK {
		t.Fatalf("token color update failed: %d %s", tokenColor.Code, tokenColor.Body.String())
	}
	decodeBody(t, tokenColor, &tag)
	if tag.Color == nil || *tag.Color != "tertiary-container" {
		t.Fatalf("expected token color to be trimmed and preserved, got %#v", tag.Color)
	}
	invalidUpdateColorValue := doJSON(t, handler, http.MethodPatch, "/api/tags/"+tag.ID, "alex", map[string]any{"color": "chartreuse-ish"})
	if invalidUpdateColorValue.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid update color value validation, got %d %s", invalidUpdateColorValue.Code, invalidUpdateColorValue.Body.String())
	}
	var invalidUpdateColorBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, invalidUpdateColorValue, &invalidUpdateColorBody)
	if invalidUpdateColorBody.Error.Code != domain.CodeValidationError || invalidUpdateColorBody.Error.Field != "color" {
		t.Fatalf("expected update color validation error, got %#v", invalidUpdateColorBody.Error)
	}
	clearBlankColor := doJSON(t, handler, http.MethodPatch, "/api/tags/"+tag.ID, "alex", map[string]any{"color": "   "})
	if clearBlankColor.Code != http.StatusOK {
		t.Fatalf("clear blank color failed: %d %s", clearBlankColor.Code, clearBlankColor.Body.String())
	}
	decodeBody(t, clearBlankColor, &tag)
	if tag.Color != nil {
		t.Fatalf("expected blank color to clear, got %#v", tag.Color)
	}

	duplicate := doJSON(t, handler, http.MethodPost, "/api/tags", "alex", map[string]any{"name": "Documentation"})
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("expected duplicate tag conflict, got %d %s", duplicate.Code, duplicate.Body.String())
	}
	var duplicateBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, duplicate, &duplicateBody)
	if duplicateBody.Error.Code != domain.CodeConflict {
		t.Fatalf("expected conflict error, got %s", duplicateBody.Error.Code)
	}

	other := doJSON(t, handler, http.MethodPost, "/api/tags", "alex", map[string]any{"name": "api"})
	if other.Code != http.StatusCreated {
		t.Fatalf("create second tag failed: %d %s", other.Code, other.Body.String())
	}
	var otherTag domain.Tag
	decodeBody(t, other, &otherTag)
	renameConflict := doJSON(t, handler, http.MethodPatch, "/api/tags/"+otherTag.ID, "alex", map[string]any{"name": "documentation"})
	if renameConflict.Code != http.StatusConflict {
		t.Fatalf("expected rename conflict, got %d %s", renameConflict.Code, renameConflict.Body.String())
	}

	missingTag := doJSON(t, handler, http.MethodPatch, "/api/tags/tag_missing", "alex", map[string]any{"name": "missing"})
	if missingTag.Code != http.StatusNotFound {
		t.Fatalf("expected missing tag not_found, got %d %s", missingTag.Code, missingTag.Body.String())
	}
	var missingTagBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, missingTag, &missingTagBody)
	if missingTagBody.Error.Code != domain.CodeNotFound || missingTagBody.Error.Field != "tag_id" {
		t.Fatalf("expected tag_id not_found error, got %#v", missingTagBody.Error)
	}
}

func TestRESTPreservesMarkdownSourceExactly(t *testing.T) {
	handler := newTestHandler(t)

	initialMarkdown := "Lead text\n\n- keep **bold**\n- keep <script>alert(1)</script>\n\n[unsafe](javascript:alert(1))"
	create := doJSON(t, handler, http.MethodPost, "/api/issues", "alex", map[string]any{
		"title":                "Markdown exactness",
		"description_markdown": initialMarkdown,
		"priority":             "P2",
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", create.Code, create.Body.String())
	}
	var issue domain.Issue
	decodeBody(t, create, &issue)
	if issue.DescriptionMarkdown != initialMarkdown {
		t.Fatalf("create response changed Markdown source: got %q want %q", issue.DescriptionMarkdown, initialMarkdown)
	}

	updatedMarkdown := "Updated source\n\n```go\nfmt.Println(\"raw\")\n```\n<span onclick=\"bad()\">kept as source</span>"
	update := doJSON(t, handler, http.MethodPatch, "/api/issues/"+issue.ID, "alex", map[string]any{
		"description_markdown": updatedMarkdown,
	})
	if update.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", update.Code, update.Body.String())
	}
	decodeBody(t, update, &issue)
	if issue.DescriptionMarkdown != updatedMarkdown {
		t.Fatalf("update response changed Markdown source: got %q want %q", issue.DescriptionMarkdown, updatedMarkdown)
	}

	commentMarkdown := "  Comment **source** with <img src=x onerror=alert(1)>  "
	comment := doJSON(t, handler, http.MethodPost, "/api/issues/"+issue.ID+"/comments", "sam", map[string]any{
		"body_markdown": commentMarkdown,
	})
	if comment.Code != http.StatusCreated {
		t.Fatalf("comment failed: %d %s", comment.Code, comment.Body.String())
	}
	var createdComment domain.Comment
	decodeBody(t, comment, &createdComment)
	if createdComment.BodyMarkdown != commentMarkdown {
		t.Fatalf("comment response changed Markdown source: got %q want %q", createdComment.BodyMarkdown, commentMarkdown)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/issues/"+issue.ID, nil)
	detail := httptest.NewRecorder()
	handler.ServeHTTP(detail, detailReq)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail failed: %d %s", detail.Code, detail.Body.String())
	}
	decodeBody(t, detail, &issue)
	if issue.DescriptionMarkdown != updatedMarkdown {
		t.Fatalf("detail changed Markdown source: got %q want %q", issue.DescriptionMarkdown, updatedMarkdown)
	}
	if len(issue.RecentComments) != 1 || issue.RecentComments[0].BodyMarkdown != commentMarkdown {
		t.Fatalf("detail changed recent comment Markdown source: %#v", issue.RecentComments)
	}

	commentsReq := httptest.NewRequest(http.MethodGet, "/api/issues/"+issue.ID+"/comments", nil)
	commentsRes := httptest.NewRecorder()
	handler.ServeHTTP(commentsRes, commentsReq)
	if commentsRes.Code != http.StatusOK {
		t.Fatalf("list comments failed: %d %s", commentsRes.Code, commentsRes.Body.String())
	}
	var comments []domain.Comment
	decodeBody(t, commentsRes, &comments)
	if len(comments) != 1 || comments[0].BodyMarkdown != commentMarkdown {
		t.Fatalf("list comments changed Markdown source: %#v", comments)
	}
}

func TestRESTAPIFallbacksReturnJSONWithStaticHandler(t *testing.T) {
	handler := newTestHandlerWithStatic(t)

	apiMissing := httptest.NewRequest(http.MethodGet, "/api/not-a-route", nil)
	apiMissingRes := httptest.NewRecorder()
	handler.ServeHTTP(apiMissingRes, apiMissing)
	if apiMissingRes.Code != http.StatusNotFound {
		t.Fatalf("expected API miss to return 404, got %d %s", apiMissingRes.Code, apiMissingRes.Body.String())
	}
	if contentType := apiMissingRes.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected API miss to return JSON, got %q", contentType)
	}
	var missingBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, apiMissingRes, &missingBody)
	if missingBody.Error.Code != domain.CodeNotFound || missingBody.Error.Field != "path" {
		t.Fatalf("expected API not_found error, got %#v", missingBody.Error)
	}

	methodMismatch := httptest.NewRequest(http.MethodPost, "/api/tags/missing", nil)
	methodMismatchRes := httptest.NewRecorder()
	handler.ServeHTTP(methodMismatchRes, methodMismatch)
	if methodMismatchRes.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected API method mismatch to return 405, got %d %s", methodMismatchRes.Code, methodMismatchRes.Body.String())
	}
	if contentType := methodMismatchRes.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected API method mismatch to return JSON, got %q", contentType)
	}
	var mismatchBody struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, methodMismatchRes, &mismatchBody)
	if mismatchBody.Error.Code != domain.CodeValidationError || mismatchBody.Error.Field != "method" {
		t.Fatalf("expected API method validation error, got %#v", mismatchBody.Error)
	}

	appRoute := httptest.NewRequest(http.MethodGet, "/board/deep-link", nil)
	appRouteRes := httptest.NewRecorder()
	handler.ServeHTTP(appRouteRes, appRoute)
	if appRouteRes.Code != http.StatusOK || !bytes.Contains(appRouteRes.Body.Bytes(), []byte("static app")) {
		t.Fatalf("expected app route to fall through to static handler, got %d %s", appRouteRes.Code, appRouteRes.Body.String())
	}
}

func TestRESTUnsupportedDeleteAndCommentMutationRoutes(t *testing.T) {
	handler := newTestHandler(t)

	issue := createIssue(t, handler, "No deletes", "P2", nil)
	comment := doJSON(t, handler, http.MethodPost, "/api/issues/"+issue.ID+"/comments", "alex", map[string]any{"body_markdown": "Append-only comment."})
	if comment.Code != http.StatusCreated {
		t.Fatalf("create comment failed: %d %s", comment.Code, comment.Body.String())
	}
	var createdComment domain.Comment
	decodeBody(t, comment, &createdComment)

	for _, tt := range []struct {
		method   string
		path     string
		wantCode int
		wantErr  domain.ErrorCode
		wantFld  string
	}{
		{method: http.MethodDelete, path: "/api/issues/" + issue.ID, wantCode: http.StatusMethodNotAllowed, wantErr: domain.CodeValidationError, wantFld: "method"},
		{method: http.MethodPatch, path: "/api/issues/" + issue.ID + "/comments/" + createdComment.ID, wantCode: http.StatusNotFound, wantErr: domain.CodeNotFound, wantFld: "path"},
		{method: http.MethodDelete, path: "/api/issues/" + issue.ID + "/comments/" + createdComment.ID, wantCode: http.StatusNotFound, wantErr: domain.CodeNotFound, wantFld: "path"},
	} {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		req.Header.Set("X-Tala-Username", "alex")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != tt.wantCode {
			t.Fatalf("%s %s: expected %d, got %d %s", tt.method, tt.path, tt.wantCode, res.Code, res.Body.String())
		}
		if contentType := res.Header().Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("%s %s: expected JSON error response, got %q", tt.method, tt.path, contentType)
		}
		var body struct {
			Error domain.AppError `json:"error"`
		}
		decodeBody(t, res, &body)
		if body.Error.Code != tt.wantErr || body.Error.Field != tt.wantFld {
			t.Fatalf("%s %s: expected %s/%s error, got %#v", tt.method, tt.path, tt.wantErr, tt.wantFld, body.Error)
		}
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/issues/"+issue.ID, nil)
	detailRes := httptest.NewRecorder()
	handler.ServeHTTP(detailRes, detailReq)
	if detailRes.Code != http.StatusOK {
		t.Fatalf("expected issue to remain after unsupported delete, got %d %s", detailRes.Code, detailRes.Body.String())
	}
	commentsReq := httptest.NewRequest(http.MethodGet, "/api/issues/"+issue.ID+"/comments", nil)
	commentsRes := httptest.NewRecorder()
	handler.ServeHTTP(commentsRes, commentsReq)
	if commentsRes.Code != http.StatusOK {
		t.Fatalf("expected comments to remain readable, got %d %s", commentsRes.Code, commentsRes.Body.String())
	}
	var comments []domain.Comment
	decodeBody(t, commentsRes, &comments)
	if len(comments) != 1 || comments[0].ID != createdComment.ID || comments[0].BodyMarkdown != createdComment.BodyMarkdown {
		t.Fatalf("expected append-only comment to remain unchanged, got %#v", comments)
	}
}

func TestRESTUnknownIssueReadRoutesReturnNotFound(t *testing.T) {
	handler := newTestHandler(t)

	for _, tt := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/issues/issue_missing"},
		{method: http.MethodGet, path: "/api/issues/issue_missing/comments"},
	} {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusNotFound {
			t.Fatalf("%s %s: expected 404, got %d %s", tt.method, tt.path, res.Code, res.Body.String())
		}
		if contentType := res.Header().Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("%s %s: expected JSON error response, got %q", tt.method, tt.path, contentType)
		}
		var body struct {
			Error domain.AppError `json:"error"`
		}
		decodeBody(t, res, &body)
		if body.Error.Code != domain.CodeNotFound || body.Error.Field != "issue_id" {
			t.Fatalf("%s %s: expected issue_id not_found error, got %#v", tt.method, tt.path, body.Error)
		}
	}
}

func TestRESTEmptyCollectionsEncodeAsArrays(t *testing.T) {
	handler := newTestHandler(t)

	for _, path := range []string{"/api/issues", "/api/tags"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("expected %s to return 200, got %d %s", path, res.Code, res.Body.String())
		}
		if strings.TrimSpace(res.Body.String()) != "[]" {
			t.Fatalf("expected %s to encode an empty array, got %q", path, res.Body.String())
		}
	}

	issue := createIssue(t, handler, "No comments yet", "P2", nil)
	commentsReq := httptest.NewRequest(http.MethodGet, "/api/issues/"+issue.ID+"/comments", nil)
	commentsRes := httptest.NewRecorder()
	handler.ServeHTTP(commentsRes, commentsReq)
	if commentsRes.Code != http.StatusOK {
		t.Fatalf("expected comments to return 200, got %d %s", commentsRes.Code, commentsRes.Body.String())
	}
	if strings.TrimSpace(commentsRes.Body.String()) != "[]" {
		t.Fatalf("expected empty comments to encode an empty array, got %q", commentsRes.Body.String())
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/issues/"+issue.ID, nil)
	detailRes := httptest.NewRecorder()
	handler.ServeHTTP(detailRes, detailReq)
	if detailRes.Code != http.StatusOK {
		t.Fatalf("expected issue detail to return 200, got %d %s", detailRes.Code, detailRes.Body.String())
	}
	var detail map[string]any
	decodeBody(t, detailRes, &detail)
	for _, field := range []string{"tags", "children", "blockers", "blocked_by", "recent_comments"} {
		value, ok := detail[field].([]any)
		if !ok {
			t.Fatalf("expected issue detail field %q to encode as an array, got %#v", field, detail[field])
		}
		if len(value) != 0 {
			t.Fatalf("expected issue detail field %q to be empty, got %#v", field, value)
		}
	}
}

func TestRESTWhitespaceUsernameRejectedOnMutations(t *testing.T) {
	handler := newTestHandler(t)

	create := doJSON(t, handler, http.MethodPost, "/api/tags", "   ", map[string]any{"name": "docs"})
	if create.Code != http.StatusUnauthorized {
		t.Fatalf("expected whitespace username to be rejected, got %d %s", create.Code, create.Body.String())
	}
	var body struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, create, &body)
	if body.Error.Code != domain.CodeMissingUsername || body.Error.Field != "username" {
		t.Fatalf("expected missing username error, got %#v", body.Error)
	}
}

func TestRESTRejectsTrailingJSONInRequestBody(t *testing.T) {
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/issues", bytes.NewReader([]byte(`{"title":"Trailing JSON","priority":"P2"} {}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tala-Username", "alex")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected trailing JSON to be rejected, got %d %s", res.Code, res.Body.String())
	}
	var body struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, res, &body)
	if body.Error.Code != domain.CodeValidationError || body.Error.Field != "body" {
		t.Fatalf("expected body validation error, got %#v", body.Error)
	}

	tag := doJSON(t, handler, http.MethodPost, "/api/tags", "alex", map[string]any{"name": "docs"})
	if tag.Code != http.StatusCreated {
		t.Fatalf("create tag failed: %d %s", tag.Code, tag.Body.String())
	}
	var created domain.Tag
	decodeBody(t, tag, &created)
	tagReq := httptest.NewRequest(http.MethodPatch, "/api/tags/"+created.ID, bytes.NewReader([]byte(`{"name":"documentation"} {}`)))
	tagReq.Header.Set("Content-Type", "application/json")
	tagReq.Header.Set("X-Tala-Username", "alex")
	tagRes := httptest.NewRecorder()
	handler.ServeHTTP(tagRes, tagReq)
	if tagRes.Code != http.StatusBadRequest {
		t.Fatalf("expected trailing JSON tag update to be rejected, got %d %s", tagRes.Code, tagRes.Body.String())
	}
	decodeBody(t, tagRes, &body)
	if body.Error.Code != domain.CodeValidationError || body.Error.Field != "body" {
		t.Fatalf("expected tag update body validation error, got %#v", body.Error)
	}
}

func TestRESTRejectsNonObjectJSONRequestBodies(t *testing.T) {
	handler := newTestHandler(t)
	issue := createIssue(t, handler, "Body shape", "P2", nil)
	tag := doJSON(t, handler, http.MethodPost, "/api/tags", "alex", map[string]any{"name": "body-shape"})
	if tag.Code != http.StatusCreated {
		t.Fatalf("create tag failed: %d %s", tag.Code, tag.Body.String())
	}
	var createdTag domain.Tag
	decodeBody(t, tag, &createdTag)

	for _, tt := range []struct {
		name     string
		method   string
		path     string
		username string
		body     string
	}{
		{name: "create issue null", method: http.MethodPost, path: "/api/issues", username: "alex", body: `null`},
		{name: "update issue array", method: http.MethodPatch, path: "/api/issues/" + issue.ID, username: "alex", body: `[]`},
		{name: "comment string", method: http.MethodPost, path: "/api/issues/" + issue.ID + "/comments", username: "alex", body: `"comment"`},
		{name: "set parent array", method: http.MethodPut, path: "/api/issues/" + issue.ID + "/parent", username: "alex", body: `[]`},
		{name: "add blocker null", method: http.MethodPost, path: "/api/issues/" + issue.ID + "/blockers", username: "alex", body: `null`},
		{name: "create tag string", method: http.MethodPost, path: "/api/tags", username: "alex", body: `"tag"`},
		{name: "update tag array", method: http.MethodPatch, path: "/api/tags/" + createdTag.ID, username: "alex", body: `[]`},
	} {
		req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte(tt.body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tala-Username", tt.username)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected non-object body to be rejected, got %d %s", tt.name, res.Code, res.Body.String())
		}
		var body struct {
			Error domain.AppError `json:"error"`
		}
		decodeBody(t, res, &body)
		if body.Error.Code != domain.CodeValidationError || body.Error.Field != "body" {
			t.Fatalf("%s: expected body validation error, got %#v", tt.name, body.Error)
		}
	}
}

func createIssue(t *testing.T, handler http.Handler, title, priority string, tags []string) domain.Issue {
	t.Helper()
	body := map[string]any{
		"title":                title,
		"description_markdown": "Description for " + title,
		"priority":             priority,
	}
	if tags != nil {
		body["tag_names"] = tags
	}
	res := doJSON(t, handler, http.MethodPost, "/api/issues", "alex", body)
	if res.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", res.Code, res.Body.String())
	}
	var issue domain.Issue
	decodeBody(t, res, &issue)
	return issue
}

func createTag(t *testing.T, handler http.Handler, name string) domain.Tag {
	t.Helper()
	res := doJSON(t, handler, http.MethodPost, "/api/tags", "alex", map[string]any{"name": name})
	if res.Code != http.StatusCreated {
		t.Fatalf("create tag failed: %d %s", res.Code, res.Body.String())
	}
	var tag domain.Tag
	decodeBody(t, res, &tag)
	return tag
}

func assertRESTError(t *testing.T, res *httptest.ResponseRecorder, wantStatus int, wantCode domain.ErrorCode, wantField string) {
	t.Helper()
	if res.Code != wantStatus {
		t.Fatalf("expected HTTP %d, got %d %s", wantStatus, res.Code, res.Body.String())
	}
	var body struct {
		Error domain.AppError `json:"error"`
	}
	decodeBody(t, res, &body)
	if body.Error.Code != wantCode || body.Error.Field != wantField {
		t.Fatalf("expected %s/%s error, got %#v", wantCode, wantField, body.Error)
	}
}

func assertIssueFilter(t *testing.T, handler http.Handler, path, wantID string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("filter %s failed: %d %s", path, res.Code, res.Body.String())
	}
	var issues []domain.Issue
	decodeBody(t, res, &issues)
	if len(issues) != 1 || issues[0].ID != wantID {
		t.Fatalf("filter %s returned %#v, want only %s", path, issues, wantID)
	}
}

func doJSON(t *testing.T, handler http.Handler, method, path, username string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if username != "" {
		req.Header.Set("X-Tala-Username", username)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func doMultipartImage(t *testing.T, handler http.Handler, username, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/uploads/images", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if username != "" {
		req.Header.Set("X-Tala-Username", username)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func decodeBody(t *testing.T, res *httptest.ResponseRecorder, dest any) {
	t.Helper()
	if err := json.Unmarshal(res.Body.Bytes(), dest); err != nil {
		t.Fatalf("decode body %q: %v", res.Body.String(), err)
	}
}
