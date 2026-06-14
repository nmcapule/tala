package store

import (
	"context"
	"path/filepath"
	"sort"
	"testing"
	"time"

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

func TestOpenCreatesDatabaseParentDirectory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), ".tala", "tala.db")

	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateIsIdempotentAndSchemaComplete(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if err := st.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	assertForeignKeysEnabled(t, st)
	assertSchemaObjects(t, st, []string{
		"comments",
		"comments_issue_created_idx",
		"issue_blockers",
		"issue_blockers_blocker_idx",
		"issue_tags",
		"issues",
		"issues_assignee_idx",
		"issues_parent_idx",
		"issues_priority_idx",
		"issues_status_idx",
		"tags",
		"tags_name_unique",
	})

	_, err := st.db.ExecContext(ctx, `INSERT INTO issue_tags (issue_id, tag_id) VALUES (?, ?)`, "issue_missing", "tag_missing")
	if err == nil {
		t.Fatal("expected migrated database to enforce foreign keys")
	}
}

func TestStoryPointsPersistAndRollUpThroughDescendants(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	parentPoints := 3
	childPoints := 2
	grandchildPoints := 5
	parent, err := st.CreateIssue(ctx, IssueInput{Title: "Parent", Status: domain.StatusNew, Priority: domain.PriorityP2, StoryPoints: &parentPoints, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := st.CreateIssue(ctx, IssueInput{Title: "Child", Status: domain.StatusNew, Priority: domain.PriorityP2, StoryPoints: &childPoints, CreatedBy: "alex", ParentIssueID: &parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	grandchild, err := st.CreateIssue(ctx, IssueInput{Title: "Grandchild", Status: domain.StatusNew, Priority: domain.PriorityP2, StoryPoints: &grandchildPoints, CreatedBy: "alex", ParentIssueID: &child.ID})
	if err != nil {
		t.Fatal(err)
	}

	parentDetail, err := st.GetIssue(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if parentDetail.StoryPoints == nil || *parentDetail.StoryPoints != parentPoints || parentDetail.StoryPointsTotal != 10 {
		t.Fatalf("expected parent direct 3SP and total 10SP, got direct=%v total=%d", parentDetail.StoryPoints, parentDetail.StoryPointsTotal)
	}
	childDetail, err := st.GetIssue(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if childDetail.StoryPointsTotal != 7 {
		t.Fatalf("expected child total 7SP, got %d", childDetail.StoryPointsTotal)
	}
	grandchildDetail, err := st.GetIssue(ctx, grandchild.ID)
	if err != nil {
		t.Fatal(err)
	}
	if grandchildDetail.StoryPointsTotal != 5 {
		t.Fatalf("expected grandchild total 5SP, got %d", grandchildDetail.StoryPointsTotal)
	}

	same, err := st.UpdateIssue(ctx, parent.ID, IssueUpdate{StoryPoints: &parentDetail.StoryPoints})
	if err != nil {
		t.Fatal(err)
	}
	if !same.UpdatedAt.Equal(parentDetail.UpdatedAt) {
		t.Fatalf("expected no-op story point update to preserve updated_at, got %s want %s", same.UpdatedAt, parentDetail.UpdatedAt)
	}
}

func TestRelationshipNoOpsPreserveUpdatedAt(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	parent, err := st.CreateIssue(ctx, IssueInput{Title: "Parent", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := st.CreateIssue(ctx, IssueInput{Title: "Child", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex", ParentIssueID: &parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := st.CreateIssue(ctx, IssueInput{Title: "Blocker", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}

	sameParent, err := st.SetParent(ctx, child.ID, &parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !sameParent.UpdatedAt.Equal(child.UpdatedAt) {
		t.Fatalf("expected same parent no-op to preserve updated_at, got %s want %s", sameParent.UpdatedAt, child.UpdatedAt)
	}

	time.Sleep(time.Millisecond)
	cleared, err := st.SetParent(ctx, child.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cleared.UpdatedAt.After(sameParent.UpdatedAt) {
		t.Fatalf("expected parent clear to advance updated_at from %s to %s", sameParent.UpdatedAt, cleared.UpdatedAt)
	}

	alreadyCleared, err := st.SetParent(ctx, child.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !alreadyCleared.UpdatedAt.Equal(cleared.UpdatedAt) {
		t.Fatalf("expected already-cleared parent no-op to preserve updated_at, got %s want %s", alreadyCleared.UpdatedAt, cleared.UpdatedAt)
	}

	time.Sleep(time.Millisecond)
	if err := st.AddBlocker(ctx, child.ID, blocker.ID); err != nil {
		t.Fatal(err)
	}
	withBlocker, err := st.GetIssue(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !withBlocker.UpdatedAt.After(alreadyCleared.UpdatedAt) {
		t.Fatalf("expected blocker add to advance updated_at from %s to %s", alreadyCleared.UpdatedAt, withBlocker.UpdatedAt)
	}

	if err := st.AddBlocker(ctx, child.ID, blocker.ID); err != nil {
		t.Fatal(err)
	}
	afterDuplicateBlocker, err := st.GetIssue(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !afterDuplicateBlocker.UpdatedAt.Equal(withBlocker.UpdatedAt) {
		t.Fatalf("expected duplicate blocker no-op to preserve updated_at, got %s want %s", afterDuplicateBlocker.UpdatedAt, withBlocker.UpdatedAt)
	}

	missingBlockerID := "issue_missing_blocker"
	if err := st.RemoveBlocker(ctx, child.ID, missingBlockerID); err != nil {
		t.Fatal(err)
	}
	afterMissingRemove, err := st.GetIssue(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !afterMissingRemove.UpdatedAt.Equal(afterDuplicateBlocker.UpdatedAt) {
		t.Fatalf("expected missing blocker removal no-op to preserve updated_at, got %s want %s", afterMissingRemove.UpdatedAt, afterDuplicateBlocker.UpdatedAt)
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
		"search target",
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

func TestIssueRelationshipStateFiltersAndSorting(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	parent, err := st.CreateIssue(ctx, IssueInput{Title: "Parent", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := st.CreateIssue(ctx, IssueInput{Title: "Charlie child", Status: domain.StatusInProgress, Priority: domain.PriorityP1, CreatedBy: "alex", ParentIssueID: &parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := st.CreateIssue(ctx, IssueInput{Title: "Alpha blocker", Status: domain.StatusNew, Priority: domain.PriorityP0, CreatedBy: "sam"})
	if err != nil {
		t.Fatal(err)
	}
	done, err := st.CreateIssue(ctx, IssueInput{Title: "Bravo done", Status: domain.StatusCompleted, Priority: domain.PriorityP4, CreatedBy: "sam"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddBlocker(ctx, child.ID, blocker.ID); err != nil {
		t.Fatal(err)
	}

	results, err := st.ListIssues(ctx, domain.IssueFilters{ID: parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	assertIssueIDs(t, results, []string{parent.ID})

	results, err = st.ListIssues(ctx, domain.IssueFilters{BlockerOf: child.ID})
	if err != nil {
		t.Fatal(err)
	}
	assertIssueIDs(t, results, []string{blocker.ID})

	results, err = st.ListIssues(ctx, domain.IssueFilters{State: "blocked"})
	if err != nil {
		t.Fatal(err)
	}
	assertIssueIDs(t, results, []string{child.ID})

	results, err = st.ListIssues(ctx, domain.IssueFilters{State: "done"})
	if err != nil {
		t.Fatal(err)
	}
	assertIssueIDs(t, results, []string{done.ID})

	results, err = st.ListIssues(ctx, domain.IssueFilters{State: "open", Sort: "title", Order: "asc"})
	if err != nil {
		t.Fatal(err)
	}
	assertIssueIDs(t, results, []string{blocker.ID, child.ID, parent.ID})

	results, err = st.ListIssues(ctx, domain.IssueFilters{Sort: "title", Order: "desc"})
	if err != nil {
		t.Fatal(err)
	}
	assertIssueIDs(t, results, []string{parent.ID, child.ID, done.ID, blocker.ID})
}

func TestIssueOrderingUsesStableTieBreakers(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	first, err := st.CreateIssue(ctx, IssueInput{Title: "Same title", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.CreateIssue(ctx, IssueInput{Title: "same title", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	third, err := st.CreateIssue(ctx, IssueInput{Title: "SAME TITLE", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE issues SET created_at=?, updated_at=?`, "2026-06-12T00:00:00Z", "2026-06-12T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	want := sortedIDs(first.ID, second.ID, third.ID)

	for _, filters := range []domain.IssueFilters{
		{},
		{Sort: "priority", Order: "asc"},
		{Sort: "priority", Order: "desc"},
		{Sort: "updated_at", Order: "asc"},
		{Sort: "updated_at", Order: "desc"},
		{Sort: "created_at", Order: "asc"},
		{Sort: "created_at", Order: "desc"},
		{Sort: "title", Order: "asc"},
		{Sort: "title", Order: "desc"},
		{Sort: "status", Order: "asc"},
		{Sort: "status", Order: "desc"},
	} {
		results, err := st.ListIssues(ctx, filters)
		if err != nil {
			t.Fatalf("filters %#v failed: %v", filters, err)
		}
		assertIssueIDs(t, results, want)
	}
}

func TestIssueDefaultOrderingUsesLastModifiedDescending(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	oldest, err := st.CreateIssue(ctx, IssueInput{Title: "Oldest", Status: domain.StatusNew, Priority: domain.PriorityP0, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	newest, err := st.CreateIssue(ctx, IssueInput{Title: "Newest", Status: domain.StatusNew, Priority: domain.PriorityP4, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	middle, err := st.CreateIssue(ctx, IssueInput{Title: "Middle", Status: domain.StatusNew, Priority: domain.PriorityP2, CreatedBy: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE issues SET updated_at = CASE id
		WHEN ? THEN '2026-06-12T00:00:00Z'
		WHEN ? THEN '2026-06-14T00:00:00Z'
		WHEN ? THEN '2026-06-13T00:00:00Z'
		ELSE updated_at END`, oldest.ID, newest.ID, middle.ID); err != nil {
		t.Fatal(err)
	}

	results, err := st.ListIssues(ctx, domain.IssueFilters{})
	if err != nil {
		t.Fatal(err)
	}
	assertIssueIDs(t, results, []string{newest.ID, middle.ID, oldest.ID})
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

func assertSchemaObjects(t *testing.T, st *Store, want []string) {
	t.Helper()
	rows, err := st.db.Query(`SELECT name FROM sqlite_master WHERE type IN ('table', 'index') AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	got := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("got schema objects %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got schema objects %v, want %v", got, want)
		}
	}
}

func sortedIDs(ids ...string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}
