package store

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"tala/internal/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "tala.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return st
}

func TestOpenCanReopenExistingDatabase(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "tala.db")

	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	assertForeignKeysEnabled(t, st)
	tagColor := "secondary-container"
	tag, err := st.CreateTag(ctx, "persistence", &tagColor)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := st.CreateIssue(ctx, IssueInput{
		Title:               "Persisted parent",
		DescriptionMarkdown: "Parent **markdown**",
		Status:              domain.StatusInProgress,
		Priority:            domain.PriorityP1,
		CreatedBy:           "alex",
		TagNames:            []string{tag.Name},
	})
	if err != nil {
		t.Fatal(err)
	}
	child, err := st.CreateIssue(ctx, IssueInput{
		Title:         "Persisted child",
		Status:        domain.StatusNew,
		Priority:      domain.PriorityP2,
		CreatedBy:     "sam",
		ParentIssueID: &parent.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := st.CreateIssue(ctx, IssueInput{
		Title:     "Persisted blocker",
		Status:    domain.StatusNew,
		Priority:  domain.PriorityP0,
		CreatedBy: "alex",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddBlocker(ctx, parent.ID, blocker.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddComment(ctx, parent.ID, "alex", "Still here after reopen."); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Fatal(err)
		}
	})
	assertForeignKeysEnabled(t, reopened)

	detail, err := reopened.GetIssue(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != parent.Title || detail.DescriptionMarkdown != parent.DescriptionMarkdown || detail.Status != parent.Status || detail.Priority != parent.Priority {
		t.Fatalf("reopened issue mismatch: %#v", detail)
	}
	assertIssueIDs(t, detail.Children, []string{child.ID})
	assertIssueIDs(t, detail.Blockers, []string{blocker.ID})
	assertCommentBodies(t, detail.RecentComments, []string{"Still here after reopen."})
	if len(detail.Tags) != 1 || detail.Tags[0].ID != tag.ID || detail.Tags[0].Name != tag.Name || detail.Tags[0].Color == nil || *detail.Tags[0].Color != tagColor {
		t.Fatalf("reopened tags mismatch: %#v", detail.Tags)
	}

	tags, err := reopened.ListTags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].ID != tag.ID || tags[0].Name != tag.Name {
		t.Fatalf("reopened tag list mismatch: %#v", tags)
	}
	_, err = reopened.db.ExecContext(ctx, `INSERT INTO issues (id,title,description_markdown,status,priority,created_by,parent_issue_id,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		"issue_invalid_parent", "Invalid parent", "", domain.StatusNew, domain.PriorityP2, "alex", "issue_missing_parent", "2026-06-13T00:00:00Z", "2026-06-13T00:00:00Z")
	if err == nil {
		t.Fatal("expected reopened database to enforce foreign keys")
	}
}

func TestRecentCommentsUseInsertionOrderWhenTimestampsMatch(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	issue, err := st.CreateIssue(ctx, IssueInput{
		Title:               "Comment ordering",
		DescriptionMarkdown: "",
		Status:              domain.StatusNew,
		Priority:            domain.PriorityP2,
		CreatedBy:           "alex",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, body := range []string{"one", "two", "three", "four", "five", "six"} {
		if _, err := st.AddComment(ctx, issue.ID, "sam", body); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE comments SET created_at=? WHERE issue_id=?`, "2026-06-12T00:00:00Z", issue.ID); err != nil {
		t.Fatal(err)
	}

	comments, err := st.ListComments(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertCommentBodies(t, comments, []string{"one", "two", "three", "four", "five", "six"})

	detail, err := st.GetIssue(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertCommentBodies(t, detail.RecentComments, []string{"two", "three", "four", "five", "six"})
}

func TestIssueAndRelationshipOrderingUseIDTieBreaker(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	parent, err := st.CreateIssue(ctx, IssueInput{Title: "Parent", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	childA, err := st.CreateIssue(ctx, IssueInput{Title: "Child A", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex", ParentIssueID: &parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	childB, err := st.CreateIssue(ctx, IssueInput{Title: "Child B", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex", ParentIssueID: &parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := st.CreateIssue(ctx, IssueInput{Title: "Blocked", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	blockerA, err := st.CreateIssue(ctx, IssueInput{Title: "Blocker A", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	blockerB, err := st.CreateIssue(ctx, IssueInput{Title: "Blocker B", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	dependentA, err := st.CreateIssue(ctx, IssueInput{Title: "Dependent A", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	dependentB, err := st.CreateIssue(ctx, IssueInput{Title: "Dependent B", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{
		{blocked.ID, blockerA.ID},
		{blocked.ID, blockerB.ID},
		{dependentA.ID, blockerA.ID},
		{dependentB.ID, blockerA.ID},
	} {
		if err := st.AddBlocker(ctx, pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE issues SET updated_at=?`, "2026-06-12T00:00:00Z"); err != nil {
		t.Fatal(err)
	}

	parentDetail, err := st.GetIssue(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertIssueIDs(t, parentDetail.Children, sortedIDs(childA.ID, childB.ID))

	blockedDetail, err := st.GetIssue(ctx, blocked.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertIssueIDs(t, blockedDetail.Blockers, sortedIDs(blockerA.ID, blockerB.ID))

	blockerDetail, err := st.GetIssue(ctx, blockerA.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertIssueIDs(t, blockerDetail.BlockedBy, sortedIDs(blocked.ID, dependentA.ID, dependentB.ID))

	listed, err := st.ListIssues(ctx, domain.IssueFilters{ParentID: parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	assertIssueIDs(t, listed, sortedIDs(childA.ID, childB.ID))
}

func TestIssueQuerySearchIncludesCommentsTagsAndMetadata(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	assignee := "sam"
	issue, err := st.CreateIssue(ctx, IssueInput{
		Title:               "Search target",
		DescriptionMarkdown: "Description has raw **Markdown**",
		Status:              domain.StatusInProgress,
		Priority:            domain.PriorityP1,
		Assignee:            &assignee,
		CreatedBy:           "creator-alpha",
		TagNames:            []string{"frontend"},
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateIssue(ctx, IssueInput{
		Title:     "Other issue",
		Status:    domain.StatusNew,
		Priority:  domain.PriorityP4,
		CreatedBy: "alex",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddComment(ctx, issue.ID, "sam", "Comment-only searchable phrase"); err != nil {
		t.Fatal(err)
	}

	for _, query := range []string{
		"raw **markdown**",
		"comment-only searchable",
		"frontend",
		"in_progress",
		"P1",
		"sam",
		"creator-alpha",
		issue.ID[len("issue_"):],
	} {
		results, err := st.ListIssues(ctx, domain.IssueFilters{Query: query})
		if err != nil {
			t.Fatalf("query %q failed: %v", query, err)
		}
		assertIssueIDs(t, results, []string{issue.ID})
	}

	results, err := st.ListIssues(ctx, domain.IssueFilters{Query: "Other issue"})
	if err != nil {
		t.Fatal(err)
	}
	assertIssueIDs(t, results, []string{other.ID})
}

func assertCommentBodies(t *testing.T, comments []domain.Comment, want []string) {
	t.Helper()
	if len(comments) != len(want) {
		t.Fatalf("got %d comments, want %d: %#v", len(comments), len(want), comments)
	}
	for i, body := range want {
		if comments[i].BodyMarkdown != body {
			t.Fatalf("comment %d body = %q, want %q; all comments: %#v", i, comments[i].BodyMarkdown, body, comments)
		}
	}
}

func assertIssueIDs(t *testing.T, issues []domain.Issue, want []string) {
	t.Helper()
	if len(issues) != len(want) {
		t.Fatalf("got %d issues, want %d: %#v", len(issues), len(want), issues)
	}
	for i, id := range want {
		if issues[i].ID != id {
			t.Fatalf("issue %d id = %q, want %q; all issues: %#v", i, issues[i].ID, id, issues)
		}
	}
}

func assertForeignKeysEnabled(t *testing.T, st *Store) {
	t.Helper()
	var enabled int
	if err := st.db.QueryRow(`PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("foreign key enforcement disabled: PRAGMA foreign_keys=%d", enabled)
	}
}

func sortedIDs(ids ...string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}
