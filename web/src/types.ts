export type Status = "new" | "in_progress" | "completed" | "canceled";
export type Priority = "P0" | "P1" | "P2" | "P3" | "P4";

export type Tag = { id: string; name: string; color: string | null; created_at: string };
export type Comment = { id: string; author: string; body_markdown: string; created_at: string };
export type Issue = {
  id: string;
  title: string;
  description_markdown: string;
  status: Status;
  priority: Priority;
  assignee?: string | null;
  created_by: string;
  parent_issue_id?: string | null;
  created_at: string;
  updated_at: string;
  tags: Tag[];
  children: Issue[];
  blockers: Issue[];
  blocked_by: Issue[];
  recent_comments: Comment[];
  child_count: number;
  comment_count: number;
  blocked: boolean;
};

export type View = "board" | "hierarchy" | "blockers" | "profile";
export type ActionNotice = { tone: "pending" | "success"; message: string };
export type UploadedImage = { url: string; filename: string; content_type: string; size: number; markdown?: string };
export type IssueFilters = {
  q: string;
  status: string;
  assignee: string;
  priority: string;
  tag: string;
  id: string;
  parent_id: string;
  blocked_by: string;
  blocker_of: string;
  state: string;
  sort: string;
  order: string;
};
