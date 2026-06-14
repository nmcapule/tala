package app

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"tala/internal/domain"
	"tala/internal/store"
)

type Service struct {
	store     *store.Store
	uploadDir string
}

func NewService(st *store.Store) *Service {
	return &Service{store: st}
}

func NewServiceWithUploadDir(st *store.Store, uploadDir string) *Service {
	return &Service{store: st, uploadDir: strings.TrimSpace(uploadDir)}
}

type CreateIssueRequest struct {
	Title               string   `json:"title"`
	DescriptionMarkdown string   `json:"description_markdown"`
	Priority            string   `json:"priority"`
	Assignee            *string  `json:"assignee"`
	TagNames            []string `json:"tag_names"`
	ParentIssueID       *string  `json:"parent_issue_id"`
}

type UpdateIssueRequest struct {
	Title               *string  `json:"title"`
	DescriptionMarkdown *string  `json:"description_markdown"`
	Status              *string  `json:"status"`
	Priority            *string  `json:"priority"`
	Assignee            **string `json:"-"`
	TagNames            []string `json:"-"`
	TagNamesSet         bool     `json:"-"`
}

type CommentRequest struct {
	BodyMarkdown string `json:"body_markdown"`
}

var tagHexColorPattern = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

var tagColorTokens = map[string]bool{
	"background":                true,
	"error":                     true,
	"error-container":           true,
	"inverse-on-surface":        true,
	"inverse-primary":           true,
	"inverse-surface":           true,
	"on-background":             true,
	"on-error":                  true,
	"on-error-container":        true,
	"on-primary":                true,
	"on-primary-container":      true,
	"on-secondary":              true,
	"on-secondary-container":    true,
	"on-surface":                true,
	"on-surface-variant":        true,
	"on-tertiary":               true,
	"on-tertiary-container":     true,
	"outline":                   true,
	"outline-variant":           true,
	"primary":                   true,
	"primary-container":         true,
	"secondary":                 true,
	"secondary-container":       true,
	"surface":                   true,
	"surface-bright":            true,
	"surface-container":         true,
	"surface-container-high":    true,
	"surface-container-highest": true,
	"surface-container-low":     true,
	"surface-container-lowest":  true,
	"surface-dim":               true,
	"surface-tint":              true,
	"surface-variant":           true,
	"tertiary":                  true,
	"tertiary-container":        true,
}

func (r *CommentRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if err := rejectNullFields(fields, "body_markdown"); err != nil {
		return err
	}
	if raw, ok := fields["body_markdown"]; ok {
		body, err := stringField(raw, "body_markdown")
		if err != nil {
			return err
		}
		r.BodyMarkdown = body
	}
	return nil
}

func (r *CreateIssueRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if err := rejectNullFields(fields, "title", "description_markdown", "priority"); err != nil {
		return err
	}
	if rawTitle, ok := fields["title"]; ok {
		title, err := stringField(rawTitle, "title")
		if err != nil {
			return err
		}
		r.Title = title
	}
	if rawDescription, ok := fields["description_markdown"]; ok {
		description, err := stringField(rawDescription, "description_markdown")
		if err != nil {
			return err
		}
		r.DescriptionMarkdown = description
	}
	if rawPriority, ok := fields["priority"]; ok {
		priority, err := stringField(rawPriority, "priority")
		if err != nil {
			return err
		}
		r.Priority = priority
	}
	if rawAssignee, ok := fields["assignee"]; ok {
		assignee, err := nullableStringField(rawAssignee, "assignee")
		if err != nil {
			return err
		}
		r.Assignee = assignee
	}
	if rawParentID, ok := fields["parent_issue_id"]; ok {
		parentID, err := nullableStringField(rawParentID, "parent_issue_id")
		if err != nil {
			return err
		}
		r.ParentIssueID = parentID
	}
	if rawTagNames, ok := fields["tag_names"]; ok {
		tagNames, err := stringSliceField(rawTagNames, "tag_names")
		if err != nil {
			return err
		}
		r.TagNames = tagNames
	}
	return nil
}

func (r *UpdateIssueRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if err := rejectNullFields(fields, "title", "description_markdown", "status", "priority"); err != nil {
		return err
	}
	if rawTitle, ok := fields["title"]; ok {
		title, err := stringField(rawTitle, "title")
		if err != nil {
			return err
		}
		r.Title = &title
	}
	if rawDescription, ok := fields["description_markdown"]; ok {
		description, err := stringField(rawDescription, "description_markdown")
		if err != nil {
			return err
		}
		r.DescriptionMarkdown = &description
	}
	if rawStatus, ok := fields["status"]; ok {
		status, err := stringField(rawStatus, "status")
		if err != nil {
			return err
		}
		r.Status = &status
	}
	if rawPriority, ok := fields["priority"]; ok {
		priority, err := stringField(rawPriority, "priority")
		if err != nil {
			return err
		}
		r.Priority = &priority
	}
	if rawTagNames, ok := fields["tag_names"]; ok {
		tagNames, err := stringSliceField(rawTagNames, "tag_names")
		if err != nil {
			return err
		}
		r.TagNames = tagNames
		r.TagNamesSet = true
	}
	if rawAssignee, ok := fields["assignee"]; ok {
		assignee, err := nullableStringField(rawAssignee, "assignee")
		if err != nil {
			return err
		}
		r.Assignee = &assignee
	}
	return nil
}

func rejectNullFields(fields map[string]json.RawMessage, names ...string) error {
	for _, name := range names {
		if raw, ok := fields[name]; ok && string(raw) == "null" {
			return domain.NewError(domain.CodeValidationError, name+" must not be null.", name)
		}
	}
	return nil
}

func stringField(raw json.RawMessage, name string) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", domain.NewError(domain.CodeValidationError, name+" must be a string.", name)
	}
	return value, nil
}

func nullableStringField(raw json.RawMessage, name string) (*string, error) {
	if string(raw) == "null" {
		return nil, nil
	}
	value, err := stringField(raw, name)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func stringSliceField(raw json.RawMessage, name string) ([]string, error) {
	if string(raw) == "null" {
		return nil, domain.NewError(domain.CodeValidationError, "Tag names must be an array.", name)
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, domain.NewError(domain.CodeValidationError, "Tag names must be an array.", name)
	}
	return values, nil
}

func (s *Service) CreateIssue(ctx context.Context, username string, req CreateIssueRequest) (domain.Issue, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return domain.Issue{}, domain.NewError(domain.CodeMissingUsername, "Username is required for this operation.", "username")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Title is required.", "title")
	}
	priority := domain.Priority(strings.TrimSpace(req.Priority))
	if priority == "" {
		priority = domain.PriorityP2
	}
	if !domain.ValidPriority(priority) {
		return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Unknown priority.", "priority")
	}
	if req.ParentIssueID != nil {
		parent := strings.TrimSpace(*req.ParentIssueID)
		if parent == "" {
			req.ParentIssueID = nil
		} else {
			req.ParentIssueID = &parent
			if _, err := s.store.GetIssue(ctx, parent); err != nil {
				return domain.Issue{}, withNotFoundField(err, "parent_issue_id")
			}
		}
	}
	return s.store.CreateIssue(ctx, store.IssueInput{
		Title:               title,
		DescriptionMarkdown: req.DescriptionMarkdown,
		Status:              domain.StatusNew,
		Priority:            priority,
		Assignee:            cleanOptional(req.Assignee),
		CreatedBy:           username,
		ParentIssueID:       req.ParentIssueID,
		TagNames:            req.TagNames,
	})
}

func (s *Service) UpdateIssue(ctx context.Context, id string, req UpdateIssueRequest) (domain.Issue, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Issue ID is required.", "issue_id")
	}
	if req.Title == nil && req.DescriptionMarkdown == nil && req.Status == nil && req.Priority == nil && req.Assignee == nil && !req.TagNamesSet {
		return s.GetIssue(ctx, id)
	}
	up := store.IssueUpdate{}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Title is required.", "title")
		}
		up.Title = &title
	}
	if req.DescriptionMarkdown != nil {
		up.DescriptionMarkdown = req.DescriptionMarkdown
	}
	if req.Status != nil {
		status := domain.Status(strings.TrimSpace(*req.Status))
		if !domain.ValidStatus(status) {
			return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Unknown status.", "status")
		}
		up.Status = &status
	}
	if req.Priority != nil {
		priority := domain.Priority(strings.TrimSpace(*req.Priority))
		if !domain.ValidPriority(priority) {
			return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Unknown priority.", "priority")
		}
		up.Priority = &priority
	}
	if req.Assignee != nil {
		assignee := cleanOptional(*req.Assignee)
		up.Assignee = &assignee
	}
	if req.TagNamesSet {
		up.ReplaceTags = true
		up.TagNames = req.TagNames
	}
	issue, err := s.store.UpdateIssue(ctx, id, up)
	return issue, withNotFoundField(err, "issue_id")
}

func (s *Service) SearchIssues(ctx context.Context, filters domain.IssueFilters) ([]domain.Issue, error) {
	filters = normalizeIssueFilters(filters)
	if filters.Status != "" && !domain.ValidStatus(domain.Status(filters.Status)) {
		return nil, domain.NewError(domain.CodeValidationError, "Unknown status.", "status")
	}
	if filters.Priority != "" && !domain.ValidPriority(domain.Priority(filters.Priority)) {
		return nil, domain.NewError(domain.CodeValidationError, "Unknown priority.", "priority")
	}
	if filters.State != "" && !validIssueState(filters.State) {
		return nil, domain.NewError(domain.CodeValidationError, "Unknown state.", "state")
	}
	if filters.Sort != "" && !validIssueSort(filters.Sort) {
		return nil, domain.NewError(domain.CodeValidationError, "Unknown sort.", "sort")
	}
	if filters.Order != "" && filters.Order != "asc" && filters.Order != "desc" {
		return nil, domain.NewError(domain.CodeValidationError, "Unknown order.", "order")
	}
	if filters.ID != "" {
		if _, err := s.store.GetIssue(ctx, filters.ID); err != nil {
			return nil, withNotFoundField(err, "id")
		}
	}
	if filters.ParentID != "" {
		if _, err := s.store.GetIssue(ctx, filters.ParentID); err != nil {
			return nil, withNotFoundField(err, "parent_id")
		}
	}
	if filters.BlockedBy != "" {
		if _, err := s.store.GetIssue(ctx, filters.BlockedBy); err != nil {
			return nil, withNotFoundField(err, "blocked_by")
		}
	}
	if filters.BlockerOf != "" {
		if _, err := s.store.GetIssue(ctx, filters.BlockerOf); err != nil {
			return nil, withNotFoundField(err, "blocker_of")
		}
	}
	return s.store.ListIssues(ctx, filters)
}

func (s *Service) GetIssue(ctx context.Context, id string) (domain.Issue, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Issue ID is required.", "issue_id")
	}
	issue, err := s.store.GetIssue(ctx, id)
	return issue, withNotFoundField(err, "issue_id")
}

func (s *Service) AddComment(ctx context.Context, issueID, username string, req CommentRequest) (domain.Comment, error) {
	issueID = strings.TrimSpace(issueID)
	if issueID == "" {
		return domain.Comment{}, domain.NewError(domain.CodeValidationError, "Issue ID is required.", "issue_id")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return domain.Comment{}, domain.NewError(domain.CodeMissingUsername, "Username is required for this operation.", "username")
	}
	body := strings.TrimSpace(req.BodyMarkdown)
	if body == "" {
		return domain.Comment{}, domain.NewError(domain.CodeValidationError, "Comment body is required.", "body_markdown")
	}
	comment, err := s.store.AddComment(ctx, issueID, username, req.BodyMarkdown)
	return comment, withNotFoundField(err, "issue_id")
}

func (s *Service) ListComments(ctx context.Context, issueID string) ([]domain.Comment, error) {
	issueID = strings.TrimSpace(issueID)
	if issueID == "" {
		return nil, domain.NewError(domain.CodeValidationError, "Issue ID is required.", "issue_id")
	}
	comments, err := s.store.ListComments(ctx, issueID)
	return comments, withNotFoundField(err, "issue_id")
}

func (s *Service) ListTags(ctx context.Context) ([]domain.Tag, error) {
	return s.store.ListTags(ctx)
}

func (s *Service) CreateTag(ctx context.Context, name string, color *string) (domain.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Tag{}, domain.NewError(domain.CodeValidationError, "Tag name is required.", "name")
	}
	cleanColor, err := normalizeTagColor(color)
	if err != nil {
		return domain.Tag{}, err
	}
	return s.store.CreateTag(ctx, name, cleanColor)
}

func (s *Service) UpdateTag(ctx context.Context, id string, name *string, color **string) (domain.Tag, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Tag{}, domain.NewError(domain.CodeValidationError, "Tag ID is required.", "tag_id")
	}
	if name != nil {
		clean := strings.TrimSpace(*name)
		if clean == "" {
			return domain.Tag{}, domain.NewError(domain.CodeValidationError, "Tag name is required.", "name")
		}
		name = &clean
	}
	if color != nil {
		clean, err := normalizeTagColor(*color)
		if err != nil {
			return domain.Tag{}, err
		}
		color = &clean
	}
	tag, err := s.store.UpdateTag(ctx, id, name, color)
	return tag, withNotFoundField(err, "tag_id")
}

func (s *Service) SetParent(ctx context.Context, issueID string, parentID *string) (domain.Issue, error) {
	issueID = strings.TrimSpace(issueID)
	if issueID == "" {
		return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Issue ID is required.", "issue_id")
	}
	issue, err := s.store.GetIssue(ctx, issueID)
	if err != nil {
		return domain.Issue{}, withNotFoundField(err, "issue_id")
	}
	if parentID != nil {
		parent := strings.TrimSpace(*parentID)
		if parent == "" {
			parentID = nil
		} else {
			if parent == issueID {
				return domain.Issue{}, domain.NewError(domain.CodeCycleDetected, "An issue cannot be its own parent.", "parent_issue_id")
			}
			if _, err := s.store.GetIssue(ctx, parent); err != nil {
				return domain.Issue{}, withNotFoundField(err, "parent_issue_id")
			}
			cycle, err := s.store.ParentWouldCycle(ctx, issueID, parent)
			if err != nil {
				return domain.Issue{}, err
			}
			if cycle {
				return domain.Issue{}, domain.NewError(domain.CodeCycleDetected, "Setting this parent would create a hierarchy cycle.", "parent_issue_id")
			}
			parentID = &parent
		}
	}
	if sameOptionalString(issue.ParentIssueID, parentID) {
		return issue, nil
	}
	return s.store.SetParent(ctx, issueID, parentID)
}

func sameOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (s *Service) AddBlocker(ctx context.Context, issueID, blockerID string) error {
	issueID = strings.TrimSpace(issueID)
	blockerID = strings.TrimSpace(blockerID)
	if issueID == "" {
		return domain.NewError(domain.CodeValidationError, "Issue ID is required.", "issue_id")
	}
	if blockerID == "" {
		return domain.NewError(domain.CodeValidationError, "Blocker issue ID is required.", "blocker_issue_id")
	}
	if issueID == blockerID {
		return domain.NewError(domain.CodeCycleDetected, "An issue cannot block itself.", "blocker_issue_id")
	}
	if _, err := s.store.GetIssue(ctx, issueID); err != nil {
		return withNotFoundField(err, "issue_id")
	}
	if _, err := s.store.GetIssue(ctx, blockerID); err != nil {
		return withNotFoundField(err, "blocker_issue_id")
	}
	cycle, err := s.store.BlockerWouldCycle(ctx, issueID, blockerID)
	if err != nil {
		return err
	}
	if cycle {
		return domain.NewError(domain.CodeCycleDetected, "Adding this blocker would create a dependency cycle.", "blocker_issue_id")
	}
	return s.store.AddBlocker(ctx, issueID, blockerID)
}

func (s *Service) RemoveBlocker(ctx context.Context, issueID, blockerID string) error {
	issueID = strings.TrimSpace(issueID)
	blockerID = strings.TrimSpace(blockerID)
	if issueID == "" {
		return domain.NewError(domain.CodeValidationError, "Issue ID is required.", "issue_id")
	}
	if blockerID == "" {
		return domain.NewError(domain.CodeValidationError, "Blocker issue ID is required.", "blocker_issue_id")
	}
	if _, err := s.store.GetIssue(ctx, issueID); err != nil {
		return withNotFoundField(err, "issue_id")
	}
	if _, err := s.store.GetIssue(ctx, blockerID); err != nil {
		return withNotFoundField(err, "blocker_issue_id")
	}
	return s.store.RemoveBlocker(ctx, issueID, blockerID)
}

func (s *Service) AssignIssue(ctx context.Context, id string, assignee *string) (domain.Issue, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Issue ID is required.", "issue_id")
	}
	clean := cleanOptional(assignee)
	current, err := s.store.GetIssue(ctx, id)
	if err != nil {
		return domain.Issue{}, withNotFoundField(err, "issue_id")
	}
	if sameOptionalString(current.Assignee, clean) {
		return current, nil
	}
	issue, err := s.store.UpdateIssue(ctx, id, store.IssueUpdate{Assignee: &clean})
	return issue, withNotFoundField(err, "issue_id")
}

func (s *Service) SetStatus(ctx context.Context, id string, status domain.Status) (domain.Issue, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Issue ID is required.", "issue_id")
	}
	status = domain.Status(strings.TrimSpace(string(status)))
	if !domain.ValidStatus(status) {
		return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Unknown status.", "status")
	}
	current, err := s.store.GetIssue(ctx, id)
	if err != nil {
		return domain.Issue{}, withNotFoundField(err, "issue_id")
	}
	if current.Status == status {
		return current, nil
	}
	issue, err := s.store.UpdateIssue(ctx, id, store.IssueUpdate{Status: &status})
	return issue, withNotFoundField(err, "issue_id")
}

func (s *Service) SetPriority(ctx context.Context, id string, priority domain.Priority) (domain.Issue, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Issue ID is required.", "issue_id")
	}
	priority = domain.Priority(strings.TrimSpace(string(priority)))
	if !domain.ValidPriority(priority) {
		return domain.Issue{}, domain.NewError(domain.CodeValidationError, "Unknown priority.", "priority")
	}
	current, err := s.store.GetIssue(ctx, id)
	if err != nil {
		return domain.Issue{}, withNotFoundField(err, "issue_id")
	}
	if current.Priority == priority {
		return current, nil
	}
	issue, err := s.store.UpdateIssue(ctx, id, store.IssueUpdate{Priority: &priority})
	return issue, withNotFoundField(err, "issue_id")
}

func cleanOptional(v *string) *string {
	if v == nil {
		return nil
	}
	clean := strings.TrimSpace(*v)
	if clean == "" {
		return nil
	}
	return &clean
}

func normalizeTagColor(color *string) (*string, *domain.AppError) {
	clean := cleanOptional(color)
	if clean == nil {
		return nil, nil
	}
	if tagHexColorPattern.MatchString(*clean) || tagColorTokens[*clean] {
		return clean, nil
	}
	return nil, domain.NewError(domain.CodeValidationError, "Tag color must be a hex color or known UI color token.", "color")
}

func normalizeIssueFilters(filters domain.IssueFilters) domain.IssueFilters {
	filters.Status = strings.TrimSpace(filters.Status)
	filters.Priority = strings.TrimSpace(filters.Priority)
	filters.Assignee = strings.TrimSpace(filters.Assignee)
	filters.Tag = strings.TrimSpace(filters.Tag)
	filters.ID = strings.TrimSpace(filters.ID)
	filters.ParentID = strings.TrimSpace(filters.ParentID)
	filters.BlockedBy = strings.TrimSpace(filters.BlockedBy)
	filters.BlockerOf = strings.TrimSpace(filters.BlockerOf)
	filters.State = strings.TrimSpace(filters.State)
	filters.Query = strings.TrimSpace(filters.Query)
	filters.Sort = strings.TrimSpace(filters.Sort)
	filters.Order = strings.TrimSpace(strings.ToLower(filters.Order))
	return filters
}

func validIssueState(state string) bool {
	switch state {
	case "open", "blocked", "done":
		return true
	default:
		return false
	}
}

func validIssueSort(sort string) bool {
	switch sort {
	case "priority", "updated_at", "created_at", "title", "status":
		return true
	default:
		return false
	}
}

func withNotFoundField(err error, field string) error {
	var appErr *domain.AppError
	if errors.As(err, &appErr) && appErr.Code == domain.CodeNotFound {
		return domain.NewError(appErr.Code, appErr.Message, field)
	}
	return err
}
