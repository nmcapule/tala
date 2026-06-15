CREATE TABLE IF NOT EXISTS issues (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  description_markdown TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('new', 'in_progress', 'completed', 'canceled')),
  priority TEXT NOT NULL CHECK (priority IN ('P0', 'P1', 'P2', 'P3', 'P4')),
  story_points INTEGER CHECK (story_points IN (1, 2, 3, 5, 8, 13, 21)),
  assignee TEXT,
  created_by TEXT NOT NULL,
  parent_issue_id TEXT REFERENCES issues(id),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tags (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  color TEXT,
  created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS tags_name_unique ON tags (lower(name));

CREATE TABLE IF NOT EXISTS issue_tags (
  issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (issue_id, tag_id)
);

CREATE TABLE IF NOT EXISTS comments (
  id TEXT PRIMARY KEY,
  issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  author TEXT NOT NULL,
  body_markdown TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS issue_blockers (
  issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  blocker_issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  PRIMARY KEY (issue_id, blocker_issue_id),
  CHECK (issue_id <> blocker_issue_id)
);

CREATE INDEX IF NOT EXISTS issues_status_idx ON issues (status);
CREATE INDEX IF NOT EXISTS issues_priority_idx ON issues (priority);
CREATE INDEX IF NOT EXISTS issues_assignee_idx ON issues (assignee);
CREATE INDEX IF NOT EXISTS issues_parent_idx ON issues (parent_issue_id);
CREATE INDEX IF NOT EXISTS comments_issue_created_idx ON comments (issue_id, created_at);
CREATE INDEX IF NOT EXISTS issue_blockers_blocker_idx ON issue_blockers (blocker_issue_id);
