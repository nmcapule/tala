package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tala/internal/domain"
	"tala/internal/store"
)

func newTestService(t *testing.T) *Service {
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
	return NewService(st)
}

func TestCreateIssueRequiresUsernameAndPreservesMarkdown(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, err := svc.CreateIssue(ctx, "", CreateIssueRequest{Title: "Missing username", Priority: "P2"})
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeMissingUsername {
		t.Fatalf("expected missing username error, got %v", err)
	}

	markdown := "Use **Markdown** and keep <script>alert(1)</script> as source."
	issue, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{
		Title:               "Markdown storage",
		DescriptionMarkdown: markdown,
		Priority:            "P1",
		TagNames:            []string{"mcp", "api"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if issue.DescriptionMarkdown != markdown {
		t.Fatalf("markdown was not preserved: %q", issue.DescriptionMarkdown)
	}
	if len(issue.Tags) != 2 {
		t.Fatalf("expected tags to be created, got %d", len(issue.Tags))
	}
}

func TestRejectsRelationshipCycles(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	parent, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Parent", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Child", Priority: "P2", ParentIssueID: &parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SetParent(ctx, parent.ID, &child.ID)
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeCycleDetected {
		t.Fatalf("expected parent cycle error, got %v", err)
	}

	blocked, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Blocked", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Blocker", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddBlocker(ctx, blocked.ID, blocker.ID); err != nil {
		t.Fatal(err)
	}
	err = svc.AddBlocker(ctx, blocker.ID, blocked.ID)
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeCycleDetected {
		t.Fatalf("expected blocker cycle error, got %v", err)
	}
}

func TestRelationshipValidationReportsRequestFields(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	issue, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Issue", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	missingParent := "issue_missing"
	_, err = svc.SetParent(ctx, issue.ID, &missingParent)
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeNotFound || appErr.Field != "parent_issue_id" {
		t.Fatalf("expected parent_issue_id not_found error, got %#v", err)
	}

	err = svc.AddBlocker(ctx, issue.ID, " ")
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "blocker_issue_id" {
		t.Fatalf("expected blocker_issue_id validation error, got %#v", err)
	}
	err = svc.AddBlocker(ctx, issue.ID, "issue_missing")
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeNotFound || appErr.Field != "blocker_issue_id" {
		t.Fatalf("expected blocker_issue_id not_found error, got %#v", err)
	}
	err = svc.RemoveBlocker(ctx, issue.ID, "")
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "blocker_issue_id" {
		t.Fatalf("expected remove blocker_issue_id validation error, got %#v", err)
	}

	_, err = svc.AssignIssue(ctx, " ", nil)
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "issue_id" {
		t.Fatalf("expected assign issue_id validation error, got %#v", err)
	}
	_, err = svc.GetIssue(ctx, " ")
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "issue_id" {
		t.Fatalf("expected get issue_id validation error, got %#v", err)
	}
	_, err = svc.UpdateIssue(ctx, "issue_missing", UpdateIssueRequest{Title: stringPtr("Missing")})
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeNotFound || appErr.Field != "issue_id" {
		t.Fatalf("expected update issue_id not_found error, got %#v", err)
	}
	_, err = svc.AddComment(ctx, "issue_missing", "alex", CommentRequest{BodyMarkdown: "Missing issue"})
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeNotFound || appErr.Field != "issue_id" {
		t.Fatalf("expected add comment issue_id not_found error, got %#v", err)
	}
	_, err = svc.ListComments(ctx, " ")
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "issue_id" {
		t.Fatalf("expected list comments issue_id validation error, got %#v", err)
	}
	_, err = svc.SetStatus(ctx, "issue_missing", domain.StatusInProgress)
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeNotFound || appErr.Field != "issue_id" {
		t.Fatalf("expected set status issue_id not_found error, got %#v", err)
	}
	_, err = svc.SetPriority(ctx, "issue_missing", domain.PriorityP0)
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeNotFound || appErr.Field != "issue_id" {
		t.Fatalf("expected set priority issue_id not_found error, got %#v", err)
	}

	updated, err := svc.SetStatus(ctx, issue.ID, domain.Status(" in_progress "))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != domain.StatusInProgress {
		t.Fatalf("expected padded status to be trimmed, got %q", updated.Status)
	}
	updated, err = svc.SetPriority(ctx, issue.ID, domain.Priority(" P0 "))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Priority != domain.PriorityP0 {
		t.Fatalf("expected padded priority to be trimmed, got %q", updated.Priority)
	}
}

func TestIssueFieldValidationAndTerminalStatusNoCascade(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	_, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Bad priority", Priority: "urgent"})
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "priority" {
		t.Fatalf("expected create priority validation error, got %#v", err)
	}
	_, err = svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "   ", Priority: "P2"})
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "title" {
		t.Fatalf("expected create title validation error, got %#v", err)
	}

	parent, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Parent", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Child", Priority: "P2", ParentIssueID: &parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Dependent", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddBlocker(ctx, dependent.ID, parent.ID); err != nil {
		t.Fatal(err)
	}

	_, err = svc.UpdateIssue(ctx, parent.ID, UpdateIssueRequest{Status: stringPtr("shipped")})
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "status" {
		t.Fatalf("expected update status validation error, got %#v", err)
	}
	_, err = svc.UpdateIssue(ctx, parent.ID, UpdateIssueRequest{Priority: stringPtr("P9")})
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "priority" {
		t.Fatalf("expected update priority validation error, got %#v", err)
	}
	_, err = svc.UpdateIssue(ctx, parent.ID, UpdateIssueRequest{Title: stringPtr("   ")})
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "title" {
		t.Fatalf("expected update title validation error, got %#v", err)
	}

	if _, err := svc.SetStatus(ctx, parent.ID, domain.StatusCompleted); err != nil {
		t.Fatal(err)
	}
	childDetail, err := svc.GetIssue(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if childDetail.Status != domain.StatusNew {
		t.Fatalf("expected completing parent not to update child status, got %q", childDetail.Status)
	}
	dependentDetail, err := svc.GetIssue(ctx, dependent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if dependentDetail.Status != domain.StatusNew || dependentDetail.Blocked {
		t.Fatalf("expected completed blocker to unblock without changing dependent status, got status=%q blocked=%v", dependentDetail.Status, dependentDetail.Blocked)
	}

	if _, err := svc.SetStatus(ctx, parent.ID, domain.StatusCanceled); err != nil {
		t.Fatal(err)
	}
	childDetail, err = svc.GetIssue(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if childDetail.Status != domain.StatusNew {
		t.Fatalf("expected canceling parent not to update child status, got %q", childDetail.Status)
	}
}

func TestCommentsOrderingAndRecentComments(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	issue, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Commented", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"one", "two", "three", "four", "five", "six"} {
		if _, err := svc.AddComment(ctx, issue.ID, "sam", CommentRequest{BodyMarkdown: body}); err != nil {
			t.Fatal(err)
		}
	}

	comments, err := svc.ListComments(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 6 || comments[0].BodyMarkdown != "one" || comments[5].BodyMarkdown != "six" {
		t.Fatalf("expected all comments oldest-first, got %#v", comments)
	}

	detail, err := svc.GetIssue(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.RecentComments) != 5 {
		t.Fatalf("expected five recent comments, got %d", len(detail.RecentComments))
	}
	if detail.RecentComments[0].BodyMarkdown != "two" || detail.RecentComments[4].BodyMarkdown != "six" {
		t.Fatalf("expected latest five comments in oldest-first display order, got %#v", detail.RecentComments)
	}
}

func TestBlockedStateIgnoresCompletedOrCanceledBlockers(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	blocked, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Blocked", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Blocker", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddBlocker(ctx, blocked.ID, blocker.ID); err != nil {
		t.Fatal(err)
	}
	detail, err := svc.GetIssue(ctx, blocked.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.Blocked {
		t.Fatal("expected issue to be blocked by unresolved blocker")
	}
	if _, err := svc.SetStatus(ctx, blocker.ID, domain.StatusCompleted); err != nil {
		t.Fatal(err)
	}
	detail, err = svc.GetIssue(ctx, blocked.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Blocked {
		t.Fatal("expected completed blocker not to block issue")
	}
	if _, err := svc.SetStatus(ctx, blocker.ID, domain.StatusCanceled); err != nil {
		t.Fatal(err)
	}
	detail, err = svc.GetIssue(ctx, blocked.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Blocked {
		t.Fatal("expected canceled blocker not to block issue")
	}
}

func TestIssueUpdatedAtTracksCommentAndBlockerMutations(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	issue, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Timestamped issue", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Timestamp blocker", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	createdAt := issue.UpdatedAt

	time.Sleep(time.Millisecond)
	if _, err := svc.AddComment(ctx, issue.ID, "sam", CommentRequest{BodyMarkdown: "Timestamp comment"}); err != nil {
		t.Fatal(err)
	}
	withComment, err := svc.GetIssue(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !withComment.UpdatedAt.After(createdAt) {
		t.Fatalf("expected comment mutation to advance updated_at from %s to %s", createdAt, withComment.UpdatedAt)
	}

	time.Sleep(time.Millisecond)
	if err := svc.AddBlocker(ctx, issue.ID, blocker.ID); err != nil {
		t.Fatal(err)
	}
	withBlocker, err := svc.GetIssue(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !withBlocker.UpdatedAt.After(withComment.UpdatedAt) {
		t.Fatalf("expected blocker mutation to advance updated_at from %s to %s", withComment.UpdatedAt, withBlocker.UpdatedAt)
	}

	if err := svc.AddBlocker(ctx, issue.ID, blocker.ID); err != nil {
		t.Fatal(err)
	}
	afterDuplicate, err := svc.GetIssue(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !afterDuplicate.UpdatedAt.Equal(withBlocker.UpdatedAt) {
		t.Fatalf("expected duplicate blocker no-op to preserve updated_at, got %s want %s", afterDuplicate.UpdatedAt, withBlocker.UpdatedAt)
	}

	time.Sleep(time.Millisecond)
	if err := svc.RemoveBlocker(ctx, issue.ID, blocker.ID); err != nil {
		t.Fatal(err)
	}
	afterRemove, err := svc.GetIssue(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !afterRemove.UpdatedAt.After(afterDuplicate.UpdatedAt) {
		t.Fatalf("expected blocker removal to advance updated_at from %s to %s", afterDuplicate.UpdatedAt, afterRemove.UpdatedAt)
	}
}

func TestEmptyIssueUpdateDoesNotAdvanceUpdatedAt(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	issue, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "No-op update", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := svc.UpdateIssue(ctx, issue.ID, UpdateIssueRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !unchanged.UpdatedAt.Equal(issue.UpdatedAt) {
		t.Fatalf("expected empty update to preserve updated_at, got %s want %s", unchanged.UpdatedAt, issue.UpdatedAt)
	}

	time.Sleep(time.Millisecond)
	nextTitle := "Changed by real update"
	changed, err := svc.UpdateIssue(ctx, issue.ID, UpdateIssueRequest{Title: &nextTitle})
	if err != nil {
		t.Fatal(err)
	}
	if !changed.UpdatedAt.After(unchanged.UpdatedAt) {
		t.Fatalf("expected real update to advance updated_at from %s to %s", unchanged.UpdatedAt, changed.UpdatedAt)
	}
}

func TestNoOpIssueUpdateFieldsAndTagsDoNotAdvanceUpdatedAt(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	assignee := "sam"
	issue, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{
		Title:               "No-op patch",
		DescriptionMarkdown: "Keep this Markdown.",
		Priority:            "P2",
		Assignee:            &assignee,
		TagNames:            []string{"mcp"},
	})
	if err != nil {
		t.Fatal(err)
	}

	sameTitle := " No-op patch "
	sameDescription := "Keep this Markdown."
	sameStatus := string(domain.StatusNew)
	samePriority := string(domain.PriorityP2)
	sameAssignee := " sam "
	sameAssigneePtr := &sameAssignee
	unchanged, err := svc.UpdateIssue(ctx, issue.ID, UpdateIssueRequest{
		Title:               &sameTitle,
		DescriptionMarkdown: &sameDescription,
		Status:              &sameStatus,
		Priority:            &samePriority,
		Assignee:            &sameAssigneePtr,
		TagNames:            []string{" MCP ", "mcp", " "},
		TagNamesSet:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !unchanged.UpdatedAt.Equal(issue.UpdatedAt) {
		t.Fatalf("expected unchanged patch to preserve updated_at, got %s want %s", unchanged.UpdatedAt, issue.UpdatedAt)
	}
	if len(unchanged.Tags) != 1 || unchanged.Tags[0].Name != "mcp" {
		t.Fatalf("expected equivalent tag update to preserve existing tags, got %#v", unchanged.Tags)
	}

	time.Sleep(time.Millisecond)
	changed, err := svc.UpdateIssue(ctx, issue.ID, UpdateIssueRequest{
		TagNames:    []string{"mcp", "api"},
		TagNamesSet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed.UpdatedAt.After(unchanged.UpdatedAt) {
		t.Fatalf("expected real tag update to advance updated_at from %s to %s", unchanged.UpdatedAt, changed.UpdatedAt)
	}
	if len(changed.Tags) != 2 {
		t.Fatalf("expected tag change to apply, got %#v", changed.Tags)
	}
}

func TestNoOpParentUpdateDoesNotAdvanceUpdatedAt(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	parent, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Parent", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Child", Priority: "P2", ParentIssueID: &parent.ID})
	if err != nil {
		t.Fatal(err)
	}

	sameParent, err := svc.SetParent(ctx, child.ID, &parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !sameParent.UpdatedAt.Equal(child.UpdatedAt) {
		t.Fatalf("expected same parent no-op to preserve updated_at, got %s want %s", sameParent.UpdatedAt, child.UpdatedAt)
	}

	time.Sleep(time.Millisecond)
	cleared, err := svc.SetParent(ctx, child.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cleared.UpdatedAt.After(sameParent.UpdatedAt) {
		t.Fatalf("expected parent clear to advance updated_at from %s to %s", sameParent.UpdatedAt, cleared.UpdatedAt)
	}

	alreadyCleared, err := svc.SetParent(ctx, child.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !alreadyCleared.UpdatedAt.Equal(cleared.UpdatedAt) {
		t.Fatalf("expected already-cleared parent no-op to preserve updated_at, got %s want %s", alreadyCleared.UpdatedAt, cleared.UpdatedAt)
	}
}

func TestNoOpSingleFieldMutationsDoNotAdvanceUpdatedAt(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	assignee := "sam"
	issue, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "No-op single fields", Priority: "P2", Assignee: &assignee})
	if err != nil {
		t.Fatal(err)
	}

	sameAssignee, err := svc.AssignIssue(ctx, issue.ID, &assignee)
	if err != nil {
		t.Fatal(err)
	}
	if !sameAssignee.UpdatedAt.Equal(issue.UpdatedAt) {
		t.Fatalf("expected same assignee no-op to preserve updated_at, got %s want %s", sameAssignee.UpdatedAt, issue.UpdatedAt)
	}

	time.Sleep(time.Millisecond)
	changedAssigneeValue := "lee"
	changedAssignee, err := svc.AssignIssue(ctx, issue.ID, &changedAssigneeValue)
	if err != nil {
		t.Fatal(err)
	}
	if !changedAssignee.UpdatedAt.After(sameAssignee.UpdatedAt) {
		t.Fatalf("expected assignee change to advance updated_at from %s to %s", sameAssignee.UpdatedAt, changedAssignee.UpdatedAt)
	}

	sameStatus, err := svc.SetStatus(ctx, issue.ID, domain.StatusNew)
	if err != nil {
		t.Fatal(err)
	}
	if !sameStatus.UpdatedAt.Equal(changedAssignee.UpdatedAt) {
		t.Fatalf("expected same status no-op to preserve updated_at, got %s want %s", sameStatus.UpdatedAt, changedAssignee.UpdatedAt)
	}

	time.Sleep(time.Millisecond)
	changedStatus, err := svc.SetStatus(ctx, issue.ID, domain.StatusInProgress)
	if err != nil {
		t.Fatal(err)
	}
	if !changedStatus.UpdatedAt.After(sameStatus.UpdatedAt) {
		t.Fatalf("expected status change to advance updated_at from %s to %s", sameStatus.UpdatedAt, changedStatus.UpdatedAt)
	}

	samePriority, err := svc.SetPriority(ctx, issue.ID, domain.PriorityP2)
	if err != nil {
		t.Fatal(err)
	}
	if !samePriority.UpdatedAt.Equal(changedStatus.UpdatedAt) {
		t.Fatalf("expected same priority no-op to preserve updated_at, got %s want %s", samePriority.UpdatedAt, changedStatus.UpdatedAt)
	}

	time.Sleep(time.Millisecond)
	changedPriority, err := svc.SetPriority(ctx, issue.ID, domain.PriorityP1)
	if err != nil {
		t.Fatal(err)
	}
	if !changedPriority.UpdatedAt.After(samePriority.UpdatedAt) {
		t.Fatalf("expected priority change to advance updated_at from %s to %s", samePriority.UpdatedAt, changedPriority.UpdatedAt)
	}
}

func TestSearchIssueFiltersTrimWhitespace(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	parent, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Parent", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Blocker", Priority: "P0"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{
		Title:               "Whitespace filter target",
		DescriptionMarkdown: "Raw Markdown filter text",
		Priority:            "P1",
		Assignee:            stringPtr("sam"),
		TagNames:            []string{"mcp"},
		ParentIssueID:       &parent.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddBlocker(ctx, child.ID, blocker.ID); err != nil {
		t.Fatal(err)
	}
	results, err := svc.SearchIssues(ctx, domain.IssueFilters{
		Status:    " new ",
		Priority:  " P1 ",
		Assignee:  " sam ",
		Tag:       " mcp ",
		ParentID:  " " + parent.ID + " ",
		BlockedBy: " " + blocker.ID + " ",
		Query:     " Markdown filter ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != child.ID {
		t.Fatalf("expected padded filters to match child issue, got %#v", results)
	}
	_, err = svc.SearchIssues(ctx, domain.IssueFilters{ParentID: " issue_missing_parent "})
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeNotFound || appErr.Field != "parent_id" {
		t.Fatalf("expected parent_id not_found error, got %#v", err)
	}
	_, err = svc.SearchIssues(ctx, domain.IssueFilters{BlockedBy: " issue_missing_blocker "})
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeNotFound || appErr.Field != "blocked_by" {
		t.Fatalf("expected blocked_by not_found error, got %#v", err)
	}
	_, err = svc.SearchIssues(ctx, domain.IssueFilters{Status: " shipped "})
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "status" {
		t.Fatalf("expected status validation error, got %#v", err)
	}
	_, err = svc.SearchIssues(ctx, domain.IssueFilters{Priority: " P9 "})
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "priority" {
		t.Fatalf("expected priority validation error, got %#v", err)
	}
}

func TestParentClearingAndTagColorUpdates(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	parent, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Parent", Priority: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateIssue(ctx, "alex", CreateIssueRequest{Title: "Child", Priority: "P2", ParentIssueID: &parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentIssueID == nil {
		t.Fatal("expected child parent to be set")
	}
	child, err = svc.SetParent(ctx, child.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentIssueID != nil {
		t.Fatalf("expected parent to clear, got %q", *child.ParentIssueID)
	}

	color := "#b5f4d8"
	tag, err := svc.CreateTag(ctx, "docs", &color)
	if err != nil {
		t.Fatal(err)
	}
	nextName := "documentation"
	cleared := (*string)(nil)
	tag, err = svc.UpdateTag(ctx, tag.ID, &nextName, &cleared)
	if err != nil {
		t.Fatal(err)
	}
	if tag.Name != nextName || tag.Color != nil {
		t.Fatalf("expected renamed tag with cleared color, got %#v", tag)
	}

	_, err = svc.UpdateTag(ctx, " ", &nextName, nil)
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeValidationError || appErr.Field != "tag_id" {
		t.Fatalf("expected tag_id validation error, got %#v", err)
	}
	_, err = svc.UpdateTag(ctx, "tag_missing", &nextName, nil)
	if appErr, ok := err.(*domain.AppError); !ok || appErr.Code != domain.CodeNotFound || appErr.Field != "tag_id" {
		t.Fatalf("expected tag_id not_found error, got %#v", err)
	}

	paddedColor := "  #ffd7bd  "
	paddedColorUpdate := &paddedColor
	tag, err = svc.UpdateTag(ctx, tag.ID, nil, &paddedColorUpdate)
	if err != nil {
		t.Fatal(err)
	}
	if tag.Color == nil || *tag.Color != "#ffd7bd" {
		t.Fatalf("expected tag color to be trimmed, got %#v", tag.Color)
	}
	blankColor := "   "
	blankColorUpdate := &blankColor
	tag, err = svc.UpdateTag(ctx, tag.ID, nil, &blankColorUpdate)
	if err != nil {
		t.Fatal(err)
	}
	if tag.Color != nil {
		t.Fatalf("expected blank tag color update to clear color, got %#v", tag.Color)
	}
}

func stringPtr(value string) *string {
	return &value
}
