package domain

import "time"

type Status string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusCanceled   Status = "canceled"
)

type Priority string

const (
	PriorityP0 Priority = "P0"
	PriorityP1 Priority = "P1"
	PriorityP2 Priority = "P2"
	PriorityP3 Priority = "P3"
	PriorityP4 Priority = "P4"
)

type Issue struct {
	ID                  string    `json:"id"`
	Title               string    `json:"title"`
	DescriptionMarkdown string    `json:"description_markdown"`
	Status              Status    `json:"status"`
	Priority            Priority  `json:"priority"`
	Assignee            *string   `json:"assignee"`
	CreatedBy           string    `json:"created_by"`
	ParentIssueID       *string   `json:"parent_issue_id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	Tags                []Tag     `json:"tags"`
	Children            []Issue   `json:"children"`
	Blockers            []Issue   `json:"blockers"`
	BlockedBy           []Issue   `json:"blocked_by"`
	RecentComments      []Comment `json:"recent_comments"`
	ChildCount          int       `json:"child_count"`
	CommentCount        int       `json:"comment_count"`
	Blocked             bool      `json:"blocked"`
}

type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     *string   `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

type Comment struct {
	ID           string    `json:"id"`
	IssueID      string    `json:"issue_id"`
	Author       string    `json:"author"`
	BodyMarkdown string    `json:"body_markdown"`
	CreatedAt    time.Time `json:"created_at"`
}

type IssueFilters struct {
	Status    string `json:"status"`
	Priority  string `json:"priority"`
	Assignee  string `json:"assignee"`
	Tag       string `json:"tag"`
	ID        string `json:"id"`
	ParentID  string `json:"parent_id"`
	BlockedBy string `json:"blocked_by"`
	BlockerOf string `json:"blocker_of"`
	State     string `json:"state"`
	Query     string `json:"q"`
	Sort      string `json:"sort"`
	Order     string `json:"order"`
}

func ValidStatus(status Status) bool {
	switch status {
	case StatusNew, StatusInProgress, StatusCompleted, StatusCanceled:
		return true
	default:
		return false
	}
}

func ValidPriority(priority Priority) bool {
	switch priority {
	case PriorityP0, PriorityP1, PriorityP2, PriorityP3, PriorityP4:
		return true
	default:
		return false
	}
}

func TerminalStatus(status Status) bool {
	return status == StatusCompleted || status == StatusCanceled
}
