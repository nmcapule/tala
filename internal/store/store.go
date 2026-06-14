package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"tala/internal/domain"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := ensureDatabaseDir(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func ensureDatabaseDir(path string) error {
	path = strings.TrimSpace(path)
	if path == "" || path == ":memory:" || strings.HasPrefix(path, "file:") {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, "migrations/"+entry.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		schema, err := migrationsFS.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, string(schema)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	if err := s.ensureIssueStoryPointsColumn(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureIssueStoryPointsColumn(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(issues)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "story_points" {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `ALTER TABLE issues ADD COLUMN story_points INTEGER CHECK (story_points IN (1, 2, 3, 5, 8, 13, 21))`)
	return err
}

type IssueInput struct {
	Title               string
	DescriptionMarkdown string
	Status              domain.Status
	Priority            domain.Priority
	StoryPoints         *int
	Assignee            *string
	CreatedBy           string
	ParentIssueID       *string
	TagNames            []string
}

type IssueUpdate struct {
	Title               *string
	DescriptionMarkdown *string
	Status              *domain.Status
	Priority            *domain.Priority
	StoryPoints         **int
	Assignee            **string
	TagNames            []string
	ReplaceTags         bool
}

func (s *Store) CreateIssue(ctx context.Context, in IssueInput) (domain.Issue, error) {
	now := time.Now().UTC()
	issue := domain.Issue{
		ID:                  "issue_" + uuid.NewString(),
		Title:               in.Title,
		DescriptionMarkdown: in.DescriptionMarkdown,
		Status:              in.Status,
		Priority:            in.Priority,
		StoryPoints:         in.StoryPoints,
		Assignee:            in.Assignee,
		CreatedBy:           in.CreatedBy,
		ParentIssueID:       in.ParentIssueID,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO issues (id,title,description_markdown,status,priority,story_points,assignee,created_by,parent_issue_id,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			issue.ID, issue.Title, issue.DescriptionMarkdown, issue.Status, issue.Priority, nullableInt(issue.StoryPoints), nullable(issue.Assignee), issue.CreatedBy, nullable(issue.ParentIssueID), formatTime(issue.CreatedAt), formatTime(issue.UpdatedAt))
		if err != nil {
			return err
		}
		return s.replaceIssueTags(ctx, tx, issue.ID, in.TagNames)
	})
	if err != nil {
		return domain.Issue{}, err
	}
	return s.GetIssue(ctx, issue.ID)
}

func (s *Store) UpdateIssue(ctx context.Context, id string, up IssueUpdate) (domain.Issue, error) {
	existing, err := s.GetIssue(ctx, id)
	if err != nil {
		return domain.Issue{}, err
	}
	scalarsChanged := false
	if up.Title != nil {
		scalarsChanged = scalarsChanged || existing.Title != *up.Title
		existing.Title = *up.Title
	}
	if up.DescriptionMarkdown != nil {
		scalarsChanged = scalarsChanged || existing.DescriptionMarkdown != *up.DescriptionMarkdown
		existing.DescriptionMarkdown = *up.DescriptionMarkdown
	}
	if up.Status != nil {
		scalarsChanged = scalarsChanged || existing.Status != *up.Status
		existing.Status = *up.Status
	}
	if up.Priority != nil {
		scalarsChanged = scalarsChanged || existing.Priority != *up.Priority
		existing.Priority = *up.Priority
	}
	if up.StoryPoints != nil {
		scalarsChanged = scalarsChanged || !sameIntPtr(existing.StoryPoints, *up.StoryPoints)
		existing.StoryPoints = *up.StoryPoints
	}
	if up.Assignee != nil {
		scalarsChanged = scalarsChanged || !sameStringPtr(existing.Assignee, *up.Assignee)
		existing.Assignee = *up.Assignee
	}
	tagsChanged := up.ReplaceTags && !sameTagNames(existing.Tags, up.TagNames)
	if !scalarsChanged && !tagsChanged {
		return existing, nil
	}
	existing.UpdatedAt = time.Now().UTC()
	err = s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE issues SET title=?, description_markdown=?, status=?, priority=?, story_points=?, assignee=?, updated_at=? WHERE id=?`,
			existing.Title, existing.DescriptionMarkdown, existing.Status, existing.Priority, nullableInt(existing.StoryPoints), nullable(existing.Assignee), formatTime(existing.UpdatedAt), id)
		if err != nil {
			return err
		}
		if tagsChanged {
			return s.replaceIssueTags(ctx, tx, id, up.TagNames)
		}
		return nil
	})
	if err != nil {
		return domain.Issue{}, err
	}
	return s.GetIssue(ctx, id)
}

func sameStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameTagNames(existing []domain.Tag, requested []string) bool {
	existingNames := make([]string, 0, len(existing))
	for _, tag := range existing {
		existingNames = append(existingNames, tag.Name)
	}
	return sameNormalizedNames(existingNames, requested)
}

func sameNormalizedNames(a, b []string) bool {
	aSet := normalizedNameSet(a)
	bSet := normalizedNameSet(b)
	if len(aSet) != len(bSet) {
		return false
	}
	for name := range aSet {
		if !bSet[name] {
			return false
		}
	}
	return true
}

func normalizedNameSet(names []string) map[string]bool {
	set := map[string]bool{}
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		set[strings.ToLower(name)] = true
	}
	return set
}

func (s *Store) GetIssue(ctx context.Context, id string) (domain.Issue, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,title,description_markdown,status,priority,story_points,assignee,created_by,parent_issue_id,created_at,updated_at FROM issues WHERE id=?`, id)
	issue, err := scanIssue(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Issue{}, domain.NewError(domain.CodeNotFound, "Issue not found.", "id")
		}
		return domain.Issue{}, err
	}
	if err := s.hydrateIssue(ctx, &issue, true); err != nil {
		return domain.Issue{}, err
	}
	return issue, nil
}

func (s *Store) ListIssues(ctx context.Context, filters domain.IssueFilters) ([]domain.Issue, error) {
	clauses := []string{"1=1"}
	args := []any{}
	if filters.Status != "" {
		clauses = append(clauses, "i.status = ?")
		args = append(args, filters.Status)
	}
	if filters.Priority != "" {
		clauses = append(clauses, "i.priority = ?")
		args = append(args, filters.Priority)
	}
	if filters.Assignee != "" {
		clauses = append(clauses, "i.assignee = ?")
		args = append(args, filters.Assignee)
	}
	if filters.ParentID != "" {
		clauses = append(clauses, "i.parent_issue_id = ?")
		args = append(args, filters.ParentID)
	}
	if filters.BlockedBy != "" {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM issue_blockers ib WHERE ib.issue_id = i.id AND ib.blocker_issue_id = ?)")
		args = append(args, filters.BlockedBy)
	}
	if filters.Tag != "" {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM issue_tags it JOIN tags t ON t.id = it.tag_id WHERE it.issue_id = i.id AND lower(t.name) = lower(?))")
		args = append(args, filters.Tag)
	}
	if filters.ID != "" {
		clauses = append(clauses, "i.id = ?")
		args = append(args, filters.ID)
	}
	if filters.BlockerOf != "" {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM issue_blockers ib WHERE ib.blocker_issue_id = i.id AND ib.issue_id = ?)")
		args = append(args, filters.BlockerOf)
	}
	switch filters.State {
	case "open":
		clauses = append(clauses, "i.status IN ('new','in_progress')")
	case "blocked":
		clauses = append(clauses, `EXISTS (
			SELECT 1 FROM issue_blockers ib
			JOIN issues b ON b.id = ib.blocker_issue_id
			WHERE ib.issue_id = i.id AND b.status NOT IN ('completed','canceled')
		)`)
	case "done":
		clauses = append(clauses, "i.status = 'completed'")
	}
	if filters.Query != "" {
		q := "%" + strings.ToLower(filters.Query) + "%"
		clauses = append(clauses, `(lower(i.title) LIKE ?
			OR lower(i.description_markdown) LIKE ?
			OR lower(i.id) LIKE ?
			OR lower(i.status) LIKE ?
			OR lower(i.priority) LIKE ?
			OR lower(coalesce(i.assignee, '')) LIKE ?
			OR lower(i.created_by) LIKE ?
			OR EXISTS (SELECT 1 FROM issue_tags it JOIN tags t ON t.id = it.tag_id WHERE it.issue_id = i.id AND lower(t.name) LIKE ?)
			OR EXISTS (SELECT 1 FROM comments c WHERE c.issue_id = i.id AND lower(c.body_markdown) LIKE ?))`)
		args = append(args, q, q, q, q, q, q, q, q, q)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT i.id,i.title,i.description_markdown,i.status,i.priority,i.story_points,i.assignee,i.created_by,i.parent_issue_id,i.created_at,i.updated_at FROM issues i WHERE `+strings.Join(clauses, " AND ")+issueOrderBy(filters), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := []domain.Issue{}
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range issues {
		if err := s.hydrateIssue(ctx, &issues[i], false); err != nil {
			return nil, err
		}
	}
	return issues, nil
}

func issueOrderBy(filters domain.IssueFilters) string {
	if filters.Sort == "" {
		return ` ORDER BY CASE i.priority WHEN 'P0' THEN 0 WHEN 'P1' THEN 1 WHEN 'P2' THEN 2 WHEN 'P3' THEN 3 ELSE 4 END, i.updated_at DESC, i.id ASC`
	}
	direction := "ASC"
	if filters.Order == "desc" {
		direction = "DESC"
	}
	switch filters.Sort {
	case "priority":
		return ` ORDER BY CASE i.priority WHEN 'P0' THEN 0 WHEN 'P1' THEN 1 WHEN 'P2' THEN 2 WHEN 'P3' THEN 3 ELSE 4 END ` + direction + `, i.updated_at DESC, i.id ASC`
	case "updated_at":
		return ` ORDER BY i.updated_at ` + direction + `, i.id ASC`
	case "created_at":
		return ` ORDER BY i.created_at ` + direction + `, i.id ASC`
	case "title":
		return ` ORDER BY lower(i.title) ` + direction + `, i.id ASC`
	case "status":
		return ` ORDER BY CASE i.status WHEN 'new' THEN 0 WHEN 'in_progress' THEN 1 WHEN 'completed' THEN 2 WHEN 'canceled' THEN 3 ELSE 4 END ` + direction + `, i.id ASC`
	default:
		return ` ORDER BY i.id ASC`
	}
}

func (s *Store) ListTags(ctx context.Context) ([]domain.Tag, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,color,created_at FROM tags ORDER BY lower(name)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []domain.Tag{}
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (s *Store) CreateTag(ctx context.Context, name string, color *string) (domain.Tag, error) {
	tag := domain.Tag{ID: "tag_" + uuid.NewString(), Name: name, Color: color, CreatedAt: time.Now().UTC()}
	_, err := s.db.ExecContext(ctx, `INSERT INTO tags (id,name,color,created_at) VALUES (?,?,?,?)`, tag.ID, tag.Name, nullable(tag.Color), formatTime(tag.CreatedAt))
	if err != nil {
		if isUniqueTagNameError(err) {
			return domain.Tag{}, domain.NewError(domain.CodeConflict, "Tag name already exists.", "name")
		}
		return domain.Tag{}, err
	}
	return tag, nil
}

func (s *Store) UpdateTag(ctx context.Context, id string, name *string, color **string) (domain.Tag, error) {
	tag, err := s.getTag(ctx, id)
	if err != nil {
		return domain.Tag{}, err
	}
	if name != nil {
		tag.Name = *name
	}
	if color != nil {
		tag.Color = *color
	}
	_, err = s.db.ExecContext(ctx, `UPDATE tags SET name=?, color=? WHERE id=?`, tag.Name, nullable(tag.Color), id)
	if err != nil {
		if isUniqueTagNameError(err) {
			return domain.Tag{}, domain.NewError(domain.CodeConflict, "Tag name already exists.", "name")
		}
		return domain.Tag{}, err
	}
	return tag, nil
}

func (s *Store) AddComment(ctx context.Context, issueID, author, body string) (domain.Comment, error) {
	if _, err := s.GetIssue(ctx, issueID); err != nil {
		return domain.Comment{}, err
	}
	comment := domain.Comment{ID: "comment_" + uuid.NewString(), IssueID: issueID, Author: author, BodyMarkdown: body, CreatedAt: time.Now().UTC()}
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO comments (id,issue_id,author,body_markdown,created_at) VALUES (?,?,?,?,?)`, comment.ID, comment.IssueID, comment.Author, comment.BodyMarkdown, formatTime(comment.CreatedAt)); err != nil {
			return err
		}
		return touchIssue(ctx, tx, issueID)
	})
	return comment, err
}

func (s *Store) ListComments(ctx context.Context, issueID string) ([]domain.Comment, error) {
	if _, err := s.GetIssue(ctx, issueID); err != nil {
		return nil, err
	}
	return s.comments(ctx, issueID, 0)
}

func (s *Store) SetParent(ctx context.Context, issueID string, parentID *string) (domain.Issue, error) {
	existing, err := s.GetIssue(ctx, issueID)
	if err != nil {
		return domain.Issue{}, err
	}
	if sameStringPtr(existing.ParentIssueID, parentID) {
		return existing, nil
	}
	_, err = s.db.ExecContext(ctx, `UPDATE issues SET parent_issue_id=?, updated_at=? WHERE id=?`, nullable(parentID), formatTime(time.Now().UTC()), issueID)
	if err != nil {
		return domain.Issue{}, err
	}
	return s.GetIssue(ctx, issueID)
}

func (s *Store) AddBlocker(ctx context.Context, issueID, blockerID string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO issue_blockers (issue_id,blocker_issue_id,created_at) VALUES (?,?,?)`, issueID, blockerID, formatTime(time.Now().UTC()))
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed == 0 {
			return nil
		}
		return touchIssue(ctx, tx, issueID)
	})
}

func (s *Store) RemoveBlocker(ctx context.Context, issueID, blockerID string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `DELETE FROM issue_blockers WHERE issue_id=? AND blocker_issue_id=?`, issueID, blockerID)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed == 0 {
			return nil
		}
		return touchIssue(ctx, tx, issueID)
	})
}

func (s *Store) ParentWouldCycle(ctx context.Context, issueID, parentID string) (bool, error) {
	current := &parentID
	for current != nil {
		if *current == issueID {
			return true, nil
		}
		var next sql.NullString
		err := s.db.QueryRowContext(ctx, `SELECT parent_issue_id FROM issues WHERE id=?`, *current).Scan(&next)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return false, domain.NewError(domain.CodeNotFound, "Parent issue not found.", "parent_issue_id")
			}
			return false, err
		}
		if next.Valid {
			current = &next.String
		} else {
			current = nil
		}
	}
	return false, nil
}

func (s *Store) BlockerWouldCycle(ctx context.Context, issueID, blockerID string) (bool, error) {
	seen := map[string]bool{}
	var walk func(string) (bool, error)
	walk = func(id string) (bool, error) {
		if id == issueID {
			return true, nil
		}
		if seen[id] {
			return false, nil
		}
		seen[id] = true
		rows, err := s.db.QueryContext(ctx, `SELECT blocker_issue_id FROM issue_blockers WHERE issue_id=?`, id)
		if err != nil {
			return false, err
		}
		defer rows.Close()
		for rows.Next() {
			var next string
			if err := rows.Scan(&next); err != nil {
				return false, err
			}
			cycle, err := walk(next)
			if cycle || err != nil {
				return cycle, err
			}
		}
		return false, rows.Err()
	}
	return walk(blockerID)
}

func (s *Store) getTag(ctx context.Context, id string) (domain.Tag, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,color,created_at FROM tags WHERE id=?`, id)
	tag, err := scanTag(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Tag{}, domain.NewError(domain.CodeNotFound, "Tag not found.", "id")
	}
	return tag, err
}

func (s *Store) replaceIssueTags(ctx context.Context, tx *sql.Tx, issueID string, names []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM issue_tags WHERE issue_id=?`, issueID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		tag, err := s.findOrCreateTag(ctx, tx, name)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO issue_tags (issue_id,tag_id) VALUES (?,?)`, issueID, tag.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) findOrCreateTag(ctx context.Context, tx *sql.Tx, name string) (domain.Tag, error) {
	row := tx.QueryRowContext(ctx, `SELECT id,name,color,created_at FROM tags WHERE lower(name)=lower(?)`, name)
	tag, err := scanTag(row)
	if err == nil {
		return tag, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.Tag{}, err
	}
	tag = domain.Tag{ID: "tag_" + uuid.NewString(), Name: name, CreatedAt: time.Now().UTC()}
	_, err = tx.ExecContext(ctx, `INSERT INTO tags (id,name,color,created_at) VALUES (?,?,?,?)`, tag.ID, tag.Name, nil, formatTime(tag.CreatedAt))
	return tag, err
}

func (s *Store) hydrateIssue(ctx context.Context, issue *domain.Issue, detail bool) error {
	var err error
	issue.Tags = []domain.Tag{}
	issue.Children = []domain.Issue{}
	issue.Blockers = []domain.Issue{}
	issue.BlockedBy = []domain.Issue{}
	issue.RecentComments = []domain.Comment{}
	issue.Tags, err = s.issueTags(ctx, issue.ID)
	if err != nil {
		return err
	}
	issue.ChildCount, err = s.count(ctx, `SELECT count(*) FROM issues WHERE parent_issue_id=?`, issue.ID)
	if err != nil {
		return err
	}
	issue.CommentCount, err = s.count(ctx, `SELECT count(*) FROM comments WHERE issue_id=?`, issue.ID)
	if err != nil {
		return err
	}
	issue.Blocked, err = s.isBlocked(ctx, issue.ID)
	if err != nil {
		return err
	}
	issue.StoryPointsTotal, err = s.storyPointsTotal(ctx, issue.ID)
	if err != nil {
		return err
	}
	if detail {
		issue.Children, err = s.relatedIssues(ctx, `SELECT id,title,description_markdown,status,priority,story_points,assignee,created_by,parent_issue_id,created_at,updated_at FROM issues WHERE parent_issue_id=? ORDER BY updated_at DESC, id ASC`, issue.ID)
		if err != nil {
			return err
		}
		issue.Blockers, err = s.relatedIssues(ctx, `SELECT i.id,i.title,i.description_markdown,i.status,i.priority,i.story_points,i.assignee,i.created_by,i.parent_issue_id,i.created_at,i.updated_at FROM issues i JOIN issue_blockers ib ON ib.blocker_issue_id=i.id WHERE ib.issue_id=? ORDER BY i.updated_at DESC, i.id ASC`, issue.ID)
		if err != nil {
			return err
		}
		issue.BlockedBy, err = s.relatedIssues(ctx, `SELECT i.id,i.title,i.description_markdown,i.status,i.priority,i.story_points,i.assignee,i.created_by,i.parent_issue_id,i.created_at,i.updated_at FROM issues i JOIN issue_blockers ib ON ib.issue_id=i.id WHERE ib.blocker_issue_id=? ORDER BY i.updated_at DESC, i.id ASC`, issue.ID)
		if err != nil {
			return err
		}
		issue.RecentComments, err = s.recentComments(ctx, issue.ID, 5)
	}
	return err
}

func (s *Store) issueTags(ctx context.Context, issueID string) ([]domain.Tag, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id,t.name,t.color,t.created_at FROM tags t JOIN issue_tags it ON it.tag_id=t.id WHERE it.issue_id=? ORDER BY lower(t.name)`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []domain.Tag{}
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (s *Store) relatedIssues(ctx context.Context, query, id string) ([]domain.Issue, error) {
	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := []domain.Issue{}
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range issues {
		if err := s.hydrateIssue(ctx, &issues[i], false); err != nil {
			return nil, err
		}
	}
	return issues, nil
}

func (s *Store) comments(ctx context.Context, issueID string, limit int) ([]domain.Comment, error) {
	query := `SELECT id,issue_id,author,body_markdown,created_at FROM comments WHERE issue_id=? ORDER BY created_at ASC, rowid ASC`
	args := []any{issueID}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := []domain.Comment{}
	for rows.Next() {
		var c domain.Comment
		var created string
		if err := rows.Scan(&c.ID, &c.IssueID, &c.Author, &c.BodyMarkdown, &created); err != nil {
			return nil, err
		}
		c.CreatedAt, err = parseTime(created)
		if err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (s *Store) recentComments(ctx context.Context, issueID string, limit int) ([]domain.Comment, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id,issue_id,author,body_markdown,created_at FROM (
  SELECT rowid AS comment_rowid,id,issue_id,author,body_markdown,created_at
  FROM comments
  WHERE issue_id=?
  ORDER BY created_at DESC, rowid DESC
  LIMIT ?
) recent
ORDER BY created_at ASC, comment_rowid ASC`, issueID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanComments(rows)
}

type commentRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanComments(rows commentRows) ([]domain.Comment, error) {
	comments := []domain.Comment{}
	for rows.Next() {
		var c domain.Comment
		var created string
		if err := rows.Scan(&c.ID, &c.IssueID, &c.Author, &c.BodyMarkdown, &created); err != nil {
			return nil, err
		}
		createdAt, err := parseTime(created)
		if err != nil {
			return nil, err
		}
		c.CreatedAt = createdAt
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (s *Store) isBlocked(ctx context.Context, issueID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM issue_blockers ib JOIN issues b ON b.id=ib.blocker_issue_id WHERE ib.issue_id=? AND b.status NOT IN ('completed','canceled')`, issueID).Scan(&count)
	return count > 0, err
}

func (s *Store) storyPointsTotal(ctx context.Context, issueID string) (int, error) {
	var total sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
WITH RECURSIVE descendants(id, story_points) AS (
  SELECT id, story_points FROM issues WHERE id=?
  UNION ALL
  SELECT i.id, i.story_points
  FROM issues i
  JOIN descendants d ON i.parent_issue_id = d.id
)
SELECT coalesce(sum(coalesce(story_points, 0)), 0) FROM descendants`, issueID).Scan(&total)
	if err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	return int(total.Int64), nil
}

func (s *Store) count(ctx context.Context, query, id string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, query, id).Scan(&count)
	return count, err
}

func (s *Store) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func touchIssue(ctx context.Context, tx *sql.Tx, issueID string) error {
	_, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at=? WHERE id=?`, formatTime(time.Now().UTC()), issueID)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanIssue(row scanner) (domain.Issue, error) {
	var issue domain.Issue
	var assignee, parent sql.NullString
	var storyPoints sql.NullInt64
	var created, updated string
	err := row.Scan(&issue.ID, &issue.Title, &issue.DescriptionMarkdown, &issue.Status, &issue.Priority, &storyPoints, &assignee, &issue.CreatedBy, &parent, &created, &updated)
	if err != nil {
		return domain.Issue{}, err
	}
	issue.StoryPoints = intPtrFromNull(storyPoints)
	issue.Assignee = ptrFromNull(assignee)
	issue.ParentIssueID = ptrFromNull(parent)
	issue.CreatedAt, err = parseTime(created)
	if err != nil {
		return domain.Issue{}, err
	}
	issue.UpdatedAt, err = parseTime(updated)
	return issue, err
}

func scanTag(row scanner) (domain.Tag, error) {
	var tag domain.Tag
	var color sql.NullString
	var created string
	err := row.Scan(&tag.ID, &tag.Name, &color, &created)
	if err != nil {
		return domain.Tag{}, err
	}
	tag.Color = ptrFromNull(color)
	tag.CreatedAt, err = parseTime(created)
	return tag, err
}

func nullable(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func ptrFromNull(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func intPtrFromNull(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	value := int(v.Int64)
	return &value
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", raw, err)
	}
	return t, nil
}

func isUniqueTagNameError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "tags_name_unique")
}
