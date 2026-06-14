import React, { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  AlertCircle,
  ArrowLeft,
  Blocks,
  Check,
  ChevronDown,
  ChevronRight,
  CircleDot,
  ClipboardList,
  Clock,
  Filter,
  GitBranch,
  Link2,
  ImagePlus,
  LoaderCircle,
  MessageSquare,
  Plus,
  RefreshCw,
  Search,
  Send,
  Settings,
  User,
  X
} from "lucide-react";
import ReactMarkdown from "react-markdown";
import rehypeSanitize from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import "./styles.css";

type Status = "new" | "in_progress" | "completed" | "canceled";
type Priority = "P0" | "P1" | "P2" | "P3" | "P4";

type Tag = { id: string; name: string; color: string | null; created_at: string };
type Comment = { id: string; author: string; body_markdown: string; created_at: string };
type Issue = {
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

type View = "board" | "hierarchy" | "blockers" | "profile";
type ActionNotice = { tone: "pending" | "success"; message: string };
type UploadedImage = { url: string; filename: string; content_type: string; size: number; markdown?: string };
type IssueFilters = {
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
const statuses: Status[] = ["new", "in_progress", "completed", "canceled"];
const priorities: Priority[] = ["P0", "P1", "P2", "P3", "P4"];
const filterKeys = ["q", "status", "assignee", "priority", "tag", "id", "parent_id", "blocked_by", "blocker_of", "state", "sort", "order"] as const;
const states = ["open", "blocked", "done"];
const sortOptions = [
  { value: "", label: "Default" },
  { value: "priority", label: "Priority" },
  { value: "updated_at", label: "Last update" },
  { value: "created_at", label: "Created" },
  { value: "title", label: "Title" },
  { value: "status", label: "Status" },
];
const tagColorTokens: Record<string, string> = {
  surface: "#f7f3ec",
  "surface-dim": "#ded7cc",
  "surface-bright": "#fffdf8",
  "surface-container-lowest": "#ffffff",
  "surface-container-low": "#f1eadf",
  "surface-container": "#e8dfd2",
  "surface-container-high": "#ddd2c2",
  "surface-container-highest": "#d1c3b0",
  "on-surface": "#191714",
  "on-surface-variant": "#5a534a",
  "inverse-surface": "#2d2923",
  "inverse-on-surface": "#f7f3ec",
  outline: "#8b8173",
  "outline-variant": "#c9bdac",
  "surface-tint": "#2d2a7a",
  primary: "#191714",
  "on-primary": "#ffffff",
  "primary-container": "#2d2a7a",
  "on-primary-container": "#f0efff",
  "inverse-primary": "#c7c4ff",
  secondary: "#006c4e",
  "on-secondary": "#ffffff",
  "secondary-container": "#b5f4d8",
  "on-secondary-container": "#003826",
  tertiary: "#8f3300",
  "on-tertiary": "#ffffff",
  "tertiary-container": "#ffd7bd",
  "on-tertiary-container": "#411300",
  error: "#b42318",
  "on-error": "#ffffff",
  "error-container": "#ffe0dc",
  "on-error-container": "#7a120c",
  background: "#f7f3ec",
  "on-background": "#191714",
  "surface-variant": "#e8dfd2",
};
const tagColorChoices = [
  "secondary-container",
  "tertiary-container",
  "error-container",
  "primary-container",
  "surface-container",
  "surface-container-highest",
  "outline",
  "secondary",
];

function App() {
  const initialView = viewFromLocation();
  const initialIssueID = issueIDFromLocation();
  const [username, setUsername] = useState(storedUsername);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [allIssues, setAllIssues] = useState<Issue[]>([]);
  const [selectedIssue, setSelectedIssue] = useState<Issue | null>(null);
  const [selectedID, setSelectedID] = useState<string | null>(initialIssueID);
  const [view, setView] = useState<View>(initialView);
  const [filterOpen, setFilterOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [filters, setFilters] = useState<IssueFilters>(() => filtersFromLocation());
  const [tags, setTags] = useState<Tag[]>([]);
  const [error, setError] = useState("");
  const [permalinkCopied, setPermalinkCopied] = useState(false);
  const [boardLoading, setBoardLoading] = useState(0);
  const [contextLoading, setContextLoading] = useState(0);
  const [tagsLoading, setTagsLoading] = useState(0);
  const [selectedLoading, setSelectedLoading] = useState(Boolean(initialIssueID));
  const [sheetLoading, setSheetLoading] = useState<"filters" | "create" | null>(null);
  const lastListView = useRef<View>(initialView);
  const showBoardLoading = useDelayedBusy(boardLoading > 0);
  const showContextLoading = useDelayedBusy(contextLoading > 0);
  const showTagsLoading = useDelayedBusy(tagsLoading > 0);
  const showSelectedLoading = useDelayedBusy(selectedLoading);
  const showSheetLoading = useDelayedBusy(Boolean(sheetLoading));

  async function trackLoading(setter: React.Dispatch<React.SetStateAction<number>>, fn: () => Promise<void>) {
    setter((count) => count + 1);
    try {
      await fn();
    } finally {
      setter((count) => Math.max(0, count - 1));
    }
  }

  async function refreshBoard() {
    await trackLoading(setBoardLoading, async () => {
      const params = new URLSearchParams();
      filterKeys.forEach((key) => {
        const value = filters[key];
        if (value) params.set(key, value);
      });
      const res = await fetch(`/api/issues?${params}`);
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || "Unable to load issues.");
      setIssues(data || []);
    });
  }

  async function refreshIssueContext() {
    await trackLoading(setContextLoading, async () => {
      const res = await fetch("/api/issues");
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || "Unable to load issues.");
      const summaries = (data || []) as Issue[];
      const details = await Promise.all(summaries.map(async (issue) => {
        const detailRes = await fetch(`/api/issues/${issue.id}`);
        const detail = await detailRes.json();
        if (!detailRes.ok) throw new Error(detail.error?.message || "Unable to load issue context.");
        return detail as Issue;
      }));
      setAllIssues(details);
    });
  }

  async function refreshTags() {
    await trackLoading(setTagsLoading, async () => {
      const res = await fetch("/api/tags");
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || "Unable to load tags.");
      setTags(data || []);
    });
  }

  async function refresh() {
    await Promise.all([refreshBoard(), refreshIssueContext(), refreshTags()]);
  }

  function resetFilters() {
    applyFilters(emptyFilters());
  }

  function applyFilters(nextFilters: IssueFilters) {
    setFilters(nextFilters);
    if (selectedID) {
      setSelectedID(null);
      setSelectedIssue(null);
    }
    const nextPath = pathWithFilters("board", nextFilters);
    if (window.location.pathname + window.location.search !== nextPath) {
      window.history.pushState(null, "", nextPath);
    }
    setView("board");
    lastListView.current = "board";
  }

  function openIssue(issueID: string) {
    setSelectedID(issueID);
    setPermalinkCopied(false);
    const nextPath = issuePath(issueID);
    if (window.location.pathname + window.location.search !== nextPath) {
      window.history.pushState(null, "", nextPath);
    }
  }

  function closeIssue() {
    setSelectedID(null);
    setPermalinkCopied(false);
    const nextPath = pathWithFilters(lastListView.current, filters);
    if (window.location.pathname !== nextPath) {
      window.history.pushState(null, "", nextPath);
    }
  }

  async function copyPermalink() {
    if (!selectedID) return;
    const url = new URL(issuePath(selectedID), window.location.origin);
    await writeClipboard(url.toString());
    setPermalinkCopied(true);
    window.setTimeout(() => setPermalinkCopied(false), 1800);
  }

  function showView(nextView: View) {
    setSelectedID(null);
    setSelectedIssue(null);
    setPermalinkCopied(false);
    setView(nextView);
    lastListView.current = nextView;
    const nextPath = pathWithFilters(nextView, filters);
    if (window.location.pathname + window.location.search !== nextPath) {
      window.history.pushState(null, "", nextPath);
    }
  }

  async function openFilterSheet() {
    setSheetLoading("filters");
    try {
      await Promise.all([refreshIssueContext(), refreshTags()]);
      setFilterOpen(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load filter context.");
    } finally {
      setSheetLoading(null);
    }
  }

  async function openCreateSheet() {
    setSheetLoading("create");
    try {
      await refreshIssueContext();
      setCreateOpen(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load issue context.");
    } finally {
      setSheetLoading(null);
    }
  }

  useEffect(() => {
    refreshBoard().catch((err) => setError(err.message));
  }, [filters.q, filters.status, filters.assignee, filters.priority, filters.tag, filters.id, filters.parent_id, filters.blocked_by, filters.blocker_of, filters.state, filters.sort, filters.order]);

  useEffect(() => {
    refreshIssueContext().catch((err) => setError(err.message));
  }, []);

  useEffect(() => {
    if (selectedID || view === "board" || view === "profile") return;
    refreshIssueContext().catch((err) => setError(err.message));
  }, [selectedID, view]);

  useEffect(() => {
    refreshTags().catch((err) => setError(err instanceof Error ? err.message : "Unable to load tags."));
  }, [allIssues.length, view]);

  useEffect(() => {
    function syncRoute() {
      const issueID = issueIDFromLocation();
      setSelectedID(issueID);
      setPermalinkCopied(false);
      if (!issueID) {
        const nextView = viewFromLocation();
        setView(nextView);
        lastListView.current = nextView;
        setFilters(filtersFromLocation());
      }
    }
    window.addEventListener("popstate", syncRoute);
    return () => window.removeEventListener("popstate", syncRoute);
  }, []);

  useEffect(() => {
    if (!selectedID) return;
    window.requestAnimationFrame(() => window.scrollTo({ top: 0, left: 0 }));
  }, [selectedID]);

  useEffect(() => {
    if (!selectedID) {
      setSelectedIssue(null);
      return;
    }
    let canceled = false;
    setSelectedIssue((current) => current?.id === selectedID ? current : null);
    setSelectedLoading(true);
    fetch(`/api/issues/${selectedID}`)
      .then(async (res) => {
        const data = await res.json();
        if (!res.ok) throw new Error(data.error?.message || "Unable to load issue.");
        if (!canceled) setSelectedIssue(data);
      })
      .catch((err) => {
        if (!canceled) setError(err.message);
      })
      .finally(() => {
        if (!canceled) setSelectedLoading(false);
      });
    return () => {
      canceled = true;
      setSelectedLoading(false);
    };
  }, [selectedID]);

  async function refreshAll() {
    await refresh();
    if (selectedID) {
      const res = await fetch(`/api/issues/${selectedID}`);
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || "Unable to load issue.");
      setSelectedIssue(data);
    }
  }

  async function retryGlobalLoad() {
    setError("");
    try {
      await refreshAll();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to reload data.");
    }
  }

  const selected = selectedID
    ? selectedIssue?.id === selectedID
      ? selectedIssue
      : allIssues.find((issue) => issue.id === selectedID) || issues.find((issue) => issue.id === selectedID) || null
    : null;

  if (!username) {
    return <Login onLogin={(name) => {
      localStorage.setItem("tala.username", name);
      setUsername(name);
    }} />;
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        {selected ? (
          <button className="icon-button" onClick={closeIssue} aria-label="Back"><ArrowLeft size={20} /></button>
        ) : (
          <div className="avatar"><User size={18} /></div>
        )}
        <div>
          <h1>{selected ? "Issue detail" : viewTitle(view)}</h1>
          <p>{selected ? selected.id.slice(0, 12) : "Local issue coordination"}</p>
        </div>
        <div className="top-actions">
          {selected && <button className="icon-button" onClick={() => copyPermalink().catch((err) => setError(err instanceof Error ? err.message : "Unable to copy permalink."))} aria-label={permalinkCopied ? "Copied permalink" : "Copy permalink"} title={permalinkCopied ? "Copied" : "Copy permalink"}><Link2 size={20} /></button>}
          {!selected && <button className="icon-button" disabled={Boolean(sheetLoading)} onClick={openFilterSheet} aria-label="Filters"><Filter size={20} /></button>}
          {!selected && <button className="icon-button primary" disabled={Boolean(sheetLoading)} onClick={openCreateSheet} aria-label="Create issue"><Plus size={20} /></button>}
        </div>
      </header>

      {error && <RequestError message={error} onRetry={retryGlobalLoad} onDismiss={() => setError("")} />}
      {showSheetLoading && <div className="content loading-content"><LoadingStatus message={sheetLoading === "create" ? "Loading issue context..." : "Loading filter context..."} /></div>}

      <main className="content">
        {selected ? (
          <IssueDetail issue={selected} detailLoading={showSelectedLoading} username={username} issues={allIssues} onOpenIssue={openIssue} onApplyFilters={applyFilters} onRefresh={refreshAll} onTagsChanged={refreshTags} onClose={closeIssue} />
        ) : selectedID ? (
          selectedLoading ? (showSelectedLoading ? <LoadingStatus message="Loading issue detail..." /> : null) : <EmptyState title="Issue not found" description="The selected issue could not be loaded." />
        ) : view === "board" ? (
          <Board issues={issues} totalIssues={allIssues.length} filters={filters} hasFilters={hasActiveFilters(filters)} loading={boardLoading > 0 || contextLoading > 0} showLoading={showBoardLoading || showContextLoading} username={username} onOpen={openIssue} onRefresh={refresh} onResetFilters={resetFilters} onApplyFilters={applyFilters} />
        ) : view === "hierarchy" ? (
          contextLoading > 0 ? (showContextLoading ? <LoadingStatus message="Loading hierarchy..." /> : null) : <Hierarchy issues={allIssues} onOpen={openIssue} />
        ) : view === "blockers" ? (
          contextLoading > 0 ? (showContextLoading ? <LoadingStatus message="Loading blockers..." /> : null) : <Blockers issues={allIssues} onOpen={openIssue} />
        ) : (
          <>
            {showTagsLoading && <LoadingStatus message="Loading tag context..." />}
            <Profile username={username} onTagsChanged={refreshAll} onLogout={() => {
            localStorage.removeItem("tala.username");
            setUsername("");
          }} />
          </>
        )}
      </main>

      <nav className="bottom-nav">
        <NavButton active={!selected && view === "board"} icon={<ClipboardList />} label="Board" onClick={() => showView("board")} />
        <NavButton active={!selected && view === "hierarchy"} icon={<GitBranch />} label="Hierarchy" onClick={() => showView("hierarchy")} />
        <NavButton active={!selected && view === "blockers"} icon={<Blocks />} label="Blockers" onClick={() => showView("blockers")} />
        <NavButton active={!selected && view === "profile"} icon={<Settings />} label="Profile" onClick={() => showView("profile")} />
      </nav>

      {filterOpen && <FilterSheet filters={filters} tags={tags} issues={allIssues} onApplyFilters={applyFilters} onClose={() => setFilterOpen(false)} />}
      {createOpen && <CreateSheet username={username} issues={allIssues} onClose={() => setCreateOpen(false)} onCreated={(issue) => {
        setCreateOpen(false);
        openIssue(issue.id);
        refreshAll();
      }} />}
    </div>
  );
}

function Login({ onLogin }: { onLogin: (username: string) => void }) {
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  return (
    <main className="login">
      <div className="brand-row"><span className="mark">T</span><strong>Tala</strong></div>
      <section className="login-panel">
        <h1>Welcome to Tala</h1>
        <p>Use a local username for issue edits, comments, and agent coordination.</p>
        <label>Username</label>
        <input value={name} onChange={(event) => setName(event.target.value)} placeholder="e.g. jdoe_ops" />
        {error && <div className="field-error"><AlertCircle size={15} />{error}</div>}
        <button className="button primary" onClick={() => {
          const trimmed = name.trim();
          if (!trimmed) return setError("Username is required.");
          onLogin(trimmed);
        }}>Continue</button>
      </section>
      <div className="login-features">
        <span><GitBranch size={16} />Hierarchy planning</span>
        <span><Blocks size={16} />Blocker tracking</span>
      </div>
    </main>
  );
}

function RequestError({ message, onRetry, onDismiss, compact = false }: { message: string; onRetry?: () => void; onDismiss?: () => void; compact?: boolean }) {
  return <div className={`${compact ? "field-error" : "error-banner"} request-error`}>
    <AlertCircle size={compact ? 15 : 16} />
    <span>{message}</span>
    <div className="request-error-actions">
      {onRetry && <button type="button" onClick={onRetry}>Retry</button>}
      {onDismiss && <button type="button" onClick={onDismiss}>Dismiss</button>}
    </div>
  </div>;
}

function ActionStatus({ notice }: { notice: ActionNotice }) {
  return <div className={`action-status ${notice.tone}`} role="status" aria-live="polite">
    {notice.tone === "success" ? <Check size={15} /> : <Clock size={15} />}
    <span>{notice.message}</span>
  </div>;
}

function LoadingStatus({ message, compact = false }: { message: string; compact?: boolean }) {
  return <div className={`loading-status ${compact ? "compact" : ""}`} role="status" aria-live="polite">
    <LoaderCircle size={compact ? 15 : 17} className="loading-spinner" />
    <span>{message}</span>
  </div>;
}

function Board({ issues, totalIssues, filters, hasFilters, loading, showLoading, username, onOpen, onRefresh, onResetFilters, onApplyFilters }: { issues: Issue[]; totalIssues: number; filters: IssueFilters; hasFilters: boolean; loading: boolean; showLoading: boolean; username: string; onOpen: (id: string) => void; onRefresh: () => Promise<void>; onResetFilters: () => void; onApplyFilters: (filters: IssueFilters) => void }) {
  const [draggingID, setDraggingID] = useState("");
  const [actionError, setActionError] = useState("");
  const [actionBusy, setActionBusy] = useState(false);
  const [collapsed, setCollapsed] = useState<Record<Status, boolean>>({
    new: false,
    in_progress: false,
    completed: false,
    canceled: false,
  });
  const showActionLoading = useDelayedBusy(actionBusy);
  async function moveIssue(issueID: string, status: Status) {
    const issue = issues.find((item) => item.id === issueID);
    if (!issue || issue.status === status || actionBusy) return;
    setActionError("");
    setActionBusy(true);
    try {
      await api(`/api/issues/${issue.id}`, { method: "PATCH", body: { status }, username });
      await onRefresh();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Unable to update issue status.");
    } finally {
      setActionBusy(false);
    }
  }
  function applyStateFilter(state: string) {
    onApplyFilters({ ...emptyFilters(), state, sort: filters.sort, order: filters.order });
  }
  async function retryBoardRefresh() {
    setActionError("");
    try {
      await onRefresh();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Unable to refresh board.");
    }
  }
  return (
    <div className="board">
      {actionError && <RequestError message={actionError} onRetry={retryBoardRefresh} onDismiss={() => setActionError("")} compact />}
      {showActionLoading && <LoadingStatus message="Updating issue status..." compact />}
      {showLoading && <LoadingStatus message="Loading issues..." />}
      {issues.length === 0 && !loading && (
        totalIssues > 0 && hasFilters ? (
          <section className="empty-state board-empty"><h2>No matching issues</h2><p>Reset filters to return to the full triage board.</p><button className="button" onClick={onResetFilters}>Reset filters</button></section>
        ) : (
          <section className="empty-state board-empty"><h2>No issues yet</h2><p>Create the first issue to start triage.</p></section>
        )
      )}
      <div className="board-stats">
        <Stat label="Open" value={issues.filter((i) => i.status === "new" || i.status === "in_progress").length} active={filters.state === "open"} onClick={() => applyStateFilter("open")} />
        <Stat label="Blocked" value={issues.filter((i) => i.blocked).length} tone="danger" active={filters.state === "blocked"} onClick={() => applyStateFilter("blocked")} />
        <Stat label="Done" value={issues.filter((i) => i.status === "completed").length} tone="good" active={filters.state === "done"} onClick={() => applyStateFilter("done")} />
      </div>
      {statuses.map((status) => {
        const statusIssues = issues.filter((issue) => issue.status === status);
        const isCollapsed = collapsed[status];
        return (
        <section key={status} className={`status-section ${draggingID ? "drop-enabled" : ""}`} onDragOver={(event) => {
          if (!draggingID) return;
          event.preventDefault();
          event.dataTransfer.dropEffect = "move";
        }} onDrop={async (event) => {
          event.preventDefault();
          const issueID = event.dataTransfer.getData("text/plain") || draggingID;
          setDraggingID("");
          await moveIssue(issueID, status);
        }}>
          <div className="status-heading">
            <h2>{statusLabel(status)} <span>{statusIssues.length}</span></h2>
            <button className="icon-button small" onClick={() => setCollapsed((current) => ({ ...current, [status]: !current[status] }))} aria-label={`${isCollapsed ? "Expand" : "Collapse"} ${statusLabel(status)}`} title={isCollapsed ? "Expand" : "Collapse"}>
              {isCollapsed ? <ChevronRight size={16} /> : <ChevronDown size={16} />}
            </button>
          </div>
          {!isCollapsed && <div className="issue-list">
            {statusIssues.length === 0 ? (
              <EmptyState title={`No ${statusLabel(status).toLowerCase()} issues`} description="Drop issues here or change a card status to fill this lane." compact />
            ) : statusIssues.map((issue) => (
              <IssueCard key={issue.id} issue={issue} dragging={draggingID === issue.id} onDragStart={(event) => {
                setDraggingID(issue.id);
                event.dataTransfer.effectAllowed = "move";
                event.dataTransfer.setData("text/plain", issue.id);
              }} onDragEnd={() => setDraggingID("")} onOpen={() => onOpen(issue.id)} onStatus={async (next) => {
                await moveIssue(issue.id, next);
              }} statusDisabled={actionBusy} />
            ))}
          </div>}
        </section>
        );
      })}
    </div>
  );
}

function IssueCard({ issue, dragging, statusDisabled, onDragStart, onDragEnd, onOpen, onStatus }: { issue: Issue; dragging: boolean; statusDisabled: boolean; onDragStart: React.DragEventHandler<HTMLElement>; onDragEnd: React.DragEventHandler<HTMLElement>; onOpen: () => void; onStatus: (status: Status) => Promise<void> }) {
  return (
    <article className={`issue-card ${issue.blocked ? "blocked" : ""} ${dragging ? "dragging" : ""}`} draggable onDragStart={onDragStart} onDragEnd={onDragEnd}>
      <button className="card-main" onClick={onOpen}>
        <div className="card-title-row"><h3>{issue.title}</h3><span>{shortID(issue.id)}</span></div>
        <div className="meta-row">
          <Badge tone={issue.priority === "P0" || issue.priority === "P1" ? "danger" : "neutral"}>{issue.priority}</Badge>
          <Badge tone={isResolved(issue) ? "good" : issue.blocked ? "danger" : "neutral"}>{statusLabel(issue.status)}</Badge>
          <Badge>{issue.assignee || "Unassigned"}</Badge>
          {issue.blocked && <Badge tone="danger">Blocked</Badge>}
        </div>
        <TagRow tags={issue.tags || []} limit={3} />
        <div className="card-footer">
          <span title={`Created by ${issue.created_by}`}><User size={14} />{issue.created_by}</span>
          <span title={`Updated ${formatDateTime(issue.updated_at)}`}><Clock size={14} />{formatDateTime(issue.updated_at)}</span>
          <span aria-label={`${issue.child_count} child issues`}><GitBranch size={14} />{issue.child_count}</span>
          <span aria-label={`${issue.comment_count} comments`}><MessageSquare size={14} />{issue.comment_count}</span>
        </div>
      </button>
      <select value={issue.status} disabled={statusDisabled} onChange={(event) => onStatus(event.target.value as Status)} aria-label="Status">
        {statuses.map((status) => <option value={status} key={status}>{statusLabel(status)}</option>)}
      </select>
    </article>
  );
}

function IssueDetail({ issue, detailLoading, username, issues, onOpenIssue, onApplyFilters, onRefresh, onTagsChanged, onClose }: { issue: Issue; detailLoading: boolean; username: string; issues: Issue[]; onOpenIssue: (id: string) => void; onApplyFilters: (filters: IssueFilters) => void; onRefresh: () => Promise<void>; onTagsChanged: () => Promise<void>; onClose: () => void }) {
  const [title, setTitle] = useState(issue.title);
  const [tagDraft, setTagDraft] = useState((issue.tags || []).map((tag) => tag.name).join(", "));
  const [assigneeDraft, setAssigneeDraft] = useState(issue.assignee || "");
  const [tab, setTab] = useState<"source" | "preview">("source");
  const [draft, setDraft] = useState(issue.description_markdown);
  const [comments, setComments] = useState<Comment[]>(issue.recent_comments || []);
  const [comment, setComment] = useState("");
  const [commentTab, setCommentTab] = useState<"source" | "preview">("source");
  const [parentID, setParentID] = useState(issue.parent_issue_id || "");
  const [parentQuery, setParentQuery] = useState("");
  const [blockerID, setBlockerID] = useState("");
  const [blockerQuery, setBlockerQuery] = useState("");
  const [actionError, setActionError] = useState("");
  const [actionNotice, setActionNotice] = useState<ActionNotice | null>(null);
  const [commentLoadError, setCommentLoadError] = useState("");
  const [commentsLoading, setCommentsLoading] = useState(false);
  const [titleError, setTitleError] = useState("");
  const [parentError, setParentError] = useState("");
  const [blockerError, setBlockerError] = useState("");
  const [commentError, setCommentError] = useState("");
  const [actionBusy, setActionBusy] = useState(false);
  const [pendingMessage, setPendingMessage] = useState("");
  const blockerSelectRef = useRef<HTMLSelectElement | null>(null);
  const descriptionRef = useRef<HTMLTextAreaElement | null>(null);
  const commentRef = useRef<HTMLTextAreaElement | null>(null);
  const showActionLoading = useDelayedBusy(actionBusy);
  const showCommentsLoading = useDelayedBusy(commentsLoading);

  useEffect(() => setTitle(issue.title), [issue.id, issue.title]);
  useEffect(() => setTagDraft((issue.tags || []).map((tag) => tag.name).join(", ")), [issue.id, issue.tags]);
  useEffect(() => setAssigneeDraft(issue.assignee || ""), [issue.id, issue.assignee]);
  useEffect(() => setDraft(issue.description_markdown), [issue.id, issue.description_markdown]);
  useEffect(() => setParentID(issue.parent_issue_id || ""), [issue.id, issue.parent_issue_id]);
  useEffect(() => {
    setParentQuery("");
    setBlockerID("");
    setBlockerQuery("");
    setTitleError("");
    setParentError("");
    setBlockerError("");
    setCommentError("");
    setCommentLoadError("");
    setActionNotice(null);
  }, [issue.id]);

  async function loadComments() {
    setCommentLoadError("");
    setCommentsLoading(true);
    try {
      const res = await fetch(`/api/issues/${issue.id}/comments`);
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || "Unable to load comments.");
      setComments(data || []);
    } catch (err) {
      setCommentLoadError(err instanceof Error ? err.message : "Unable to load comments.");
    } finally {
      setCommentsLoading(false);
    }
  }

  useEffect(() => {
    loadComments();
  }, [issue.id, issue.comment_count]);

  async function patch(body: Record<string, unknown>, feedback?: { pending: string; success: string }) {
    await runAction(async () => {
      await api(`/api/issues/${issue.id}`, { method: "PATCH", body, username });
      await onRefresh();
    }, undefined, feedback);
  }

  async function runAction(fn: () => Promise<void>, setInlineError?: (message: string) => void, feedback: { pending: string; success: string } = { pending: "Saving changes...", success: "Changes saved." }) {
    if (actionBusy) return;
    setActionError("");
    setActionNotice(null);
    setPendingMessage(feedback.pending);
    setInlineError?.("");
    setActionBusy(true);
    try {
      await fn();
      setActionNotice({ tone: "success", message: feedback.success });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Request failed.";
      setActionNotice(null);
      if (setInlineError) {
        setInlineError(message);
      } else {
        setActionError(message);
      }
    } finally {
      setActionBusy(false);
      setPendingMessage("");
    }
  }

  const existingBlockerIDs = new Set((issue.blockers || []).map((blocker) => blocker.id));
  const existingBlockedByIDs = new Set((issue.blocked_by || []).map((blocked) => blocked.id));
  const relationshipCandidates = issues.filter((candidate) => candidate.id !== issue.id);
  const validParentCandidates = relationshipCandidates.filter((candidate) => !isDescendantOf(candidate.id, issue.id, issues));
  const parentCandidates = includeSelectedIssue(
    validParentCandidates.filter((candidate) => matchesIssueSearch(candidate, parentQuery)),
    parentID,
    validParentCandidates,
  );
  const validBlockerCandidates = relationshipCandidates.filter((candidate) => !existingBlockerIDs.has(candidate.id) && !existingBlockedByIDs.has(candidate.id));
  const blockerCandidates = includeSelectedIssue(
    validBlockerCandidates.filter((candidate) => matchesIssueSearch(candidate, blockerQuery)),
    blockerID,
    validBlockerCandidates,
  );
  const unresolvedBlockerCandidates = blockerCandidates.filter((candidate) => !isResolved(candidate));
  const resolvedBlockerCandidates = blockerCandidates.filter(isResolved);
  const currentParent = issue.parent_issue_id ? issues.find((candidate) => candidate.id === issue.parent_issue_id) : undefined;
  const parentSelectionUnchanged = parentID === (issue.parent_issue_id || "");
  const selectedParentPreserved = Boolean(parentQuery.trim() && parentID && parentCandidates.some((candidate) => candidate.id === parentID) && !matchesIssueSearch(parentCandidates.find((candidate) => candidate.id === parentID)!, parentQuery));
  const blockerQueryHasMatches = blockerCandidates.length > 0;
  function applyRelationshipFilter(partial: Partial<IssueFilters>) {
    onApplyFilters({ ...emptyFilters(), ...partial });
  }

  return (
    <div className="detail">
      {actionError && <div className="field-error"><AlertCircle size={15} />{actionError}</div>}
      {detailLoading && <LoadingStatus message="Loading latest issue detail..." compact />}
      {showActionLoading && pendingMessage && <LoadingStatus message={pendingMessage} compact />}
      {actionNotice && <ActionStatus notice={actionNotice} />}
      <section className="detail-hero">
        <div className="card-title-row"><h2>{issue.title}</h2><span>{shortID(issue.id)}</span></div>
        <div className="edit-stack">
          <div className="edit-field">
            <label>Title</label>
            <div className="field-action-row">
              <input className={titleError ? "invalid" : ""} value={title} onChange={(e) => {
                setTitle(e.target.value);
                setTitleError("");
                setActionNotice(null);
              }} />
              <button className="button" disabled={actionBusy} onClick={() => {
                if (!title.trim()) {
                  setTitleError("Title is required.");
                  return;
                }
                patch({ title }, { pending: "Saving title...", success: "Title saved." });
              }}>Save</button>
            </div>
            {titleError && <div className="field-error inline-error"><AlertCircle size={15} />{titleError}</div>}
          </div>
          <div className="meta-row detail-meta-row">
            <select value={issue.status} disabled={actionBusy} onChange={(e) => patch({ status: e.target.value }, { pending: "Saving status...", success: "Status saved." })}>{statuses.map((s) => <option key={s} value={s}>{statusLabel(s)}</option>)}</select>
            <select value={issue.priority} disabled={actionBusy} onChange={(e) => patch({ priority: e.target.value }, { pending: "Saving priority...", success: "Priority saved." })}>{priorities.map((p) => <option key={p} value={p}>{p}</option>)}</select>
            <Badge>Created by {issue.created_by}</Badge>
          </div>
          <div className="edit-field">
            <label>Assignee</label>
            <div className="field-action-row">
              <input value={assigneeDraft} placeholder="Assignee" onChange={(e) => {
                setAssigneeDraft(e.target.value);
                setActionNotice(null);
              }} />
              <button className="button" disabled={actionBusy} onClick={() => patch({ assignee: optionalText(assigneeDraft) }, { pending: "Saving assignee...", success: "Assignee saved." })}>Save assignee</button>
            </div>
          </div>
          <div className="edit-field">
            <label>Tags</label>
            <div className="field-action-row">
              <input value={tagDraft} onChange={(e) => {
                setTagDraft(e.target.value);
                setActionNotice(null);
              }} placeholder="mcp, api" />
              <button className="button" disabled={actionBusy} onClick={() => runAction(async () => {
                await api(`/api/issues/${issue.id}`, { method: "PATCH", body: { tag_names: splitTags(tagDraft) }, username });
                await Promise.all([onRefresh(), onTagsChanged()]);
              }, undefined, { pending: "Saving tags...", success: "Tags saved." })}>Save</button>
            </div>
          </div>
        </div>
      </section>

      <section className="panel">
        <div className="section-title"><h3>Description</h3><Segment value={tab} onChange={setTab} /></div>
        {tab === "source" ? (
          <textarea ref={descriptionRef} className="editor-surface" value={draft} onChange={(e) => {
            setDraft(e.target.value);
            setActionNotice(null);
          }} rows={7} />
        ) : (
          <div className="comment-preview editor-preview"><Markdown text={draft || "_No description yet._"} /></div>
        )}
        <div className="detail-actions">
          <ImageUploadControl username={username} disabled={actionBusy} onUploaded={(markdown) => insertAtCursor(descriptionRef, draft, setDraft, markdown)} />
          <button className="button" disabled={actionBusy} onClick={() => patch({ description_markdown: draft }, { pending: "Saving description...", success: "Description saved." })}>Save description</button>
        </div>
      </section>

      <section className="panel">
        <div className="section-title">
          <h3>Relationships</h3>
          <div className="section-actions">
            <button className="icon-button small" disabled={actionBusy} onClick={() => runAction(onRefresh, undefined, { pending: "Refreshing relationship context...", success: "Relationship context refreshed." })} aria-label="Refresh relationship context" title="Refresh relationship context"><RefreshCw size={15} /></button>
            <Link2 size={18} />
          </div>
        </div>
        <div className="relationship-grid">
          <button type="button" disabled={!issue.parent_issue_id} onClick={() => issue.parent_issue_id && applyRelationshipFilter({ id: issue.parent_issue_id })}><strong>{issue.parent_issue_id ? 1 : 0}</strong><span>Parent</span></button>
          <button type="button" disabled={(issue.children?.length || 0) === 0} onClick={() => applyRelationshipFilter({ parent_id: issue.id })}><strong>{issue.children?.length || 0}</strong><span>Children</span></button>
          <button type="button" disabled={(issue.blockers?.length || 0) === 0} onClick={() => applyRelationshipFilter({ blocker_of: issue.id })}><strong>{issue.blockers?.length || 0}</strong><span>Blockers</span></button>
          <button type="button" disabled={(issue.blocked_by?.length || 0) === 0} onClick={() => applyRelationshipFilter({ blocked_by: issue.id })}><strong>{issue.blocked_by?.length || 0}</strong><span>Blocked by this</span></button>
        </div>

        <div className="edit-field">
          <label>Parent issue</label>
          <div className="relationship-control">
            <input value={parentQuery} onChange={(e) => setParentQuery(e.target.value)} placeholder="Search parent issues..." aria-label="Search parent issues" />
            <PickerFeedback
              visibleCount={parentCandidates.length}
              totalCount={validParentCandidates.length}
              query={parentQuery}
              emptyLabel="No parent candidates match this search."
              preservedLabel={selectedParentPreserved ? "Selected parent stays available while filtering." : undefined}
            />
          </div>
          <div className="field-action-row">
            <select value={parentID} onChange={(e) => {
              setParentID(e.target.value);
              setActionNotice(null);
            }}>
              <option value="">No parent</option>
              {parentCandidates.map((candidate) => <option key={candidate.id} value={candidate.id}>{issueOptionLabel(candidate)}</option>)}
            </select>
            <button className="button" disabled={actionBusy || parentSelectionUnchanged} onClick={() => runAction(async () => {
              await api(`/api/issues/${issue.id}/parent`, { method: "PUT", body: { parent_issue_id: parentID || null }, username });
              await onRefresh();
            }, setParentError, { pending: "Updating parent...", success: "Parent updated." })}>Set</button>
          </div>
          {parentError && <div className="field-error inline-error"><AlertCircle size={15} />{parentError}</div>}
        </div>

        <RelationshipList title="Parent" issues={currentParent ? [currentParent] : []} onOpen={onOpenIssue} />
        <RelationshipList title="Children" issues={issue.children || []} onOpen={onOpenIssue} />
        <BlockerRelationshipList issues={issue.blockers || []} onRemove={(blocker) => runAction(async () => {
          await api(`/api/issues/${issue.id}/blockers/${blocker.id}`, { method: "DELETE", username });
          await onRefresh();
        }, setBlockerError, { pending: "Removing blocker...", success: "Blocker removed." })} onOpen={onOpenIssue} />
        <RelationshipList title="Blocking" issues={issue.blocked_by || []} onOpen={onOpenIssue} />

        <div className="edit-field">
          <label>Add blocker</label>
          <div className="relationship-control">
            <input value={blockerQuery} onChange={(e) => setBlockerQuery(e.target.value)} placeholder="Search blocker issues..." aria-label="Search blocker issues" />
            <PickerFeedback
              visibleCount={blockerCandidates.length}
              totalCount={validBlockerCandidates.length}
              query={blockerQuery}
              emptyLabel="No blocker candidates match this search."
            />
          </div>
          <select ref={blockerSelectRef} value={blockerID} onChange={(e) => {
            setBlockerID(e.target.value);
            setActionNotice(null);
          }}>
            <option value="">Select blocker issue</option>
            <optgroup label={`Unresolved blockers (${unresolvedBlockerCandidates.length})`}>
              {unresolvedBlockerCandidates.map((candidate) => <option key={candidate.id} value={candidate.id}>{issueOptionLabel(candidate)}</option>)}
            </optgroup>
            <optgroup label={`Completed or canceled (${resolvedBlockerCandidates.length})`}>
              {resolvedBlockerCandidates.map((candidate) => <option key={candidate.id} value={candidate.id}>{issueOptionLabel(candidate)}</option>)}
            </optgroup>
          </select>
          <div className="detail-actions">
            <button className="button" disabled={actionBusy || !blockerID || !blockerQueryHasMatches} onClick={() => runAction(async () => {
              const selectedBlockerID = blockerSelectRef.current?.value || blockerID;
              if (!selectedBlockerID) return;
              await api(`/api/issues/${issue.id}/blockers`, { method: "POST", body: { blocker_issue_id: selectedBlockerID }, username });
              setBlockerID("");
              setBlockerQuery("");
              if (blockerSelectRef.current) blockerSelectRef.current.value = "";
              await onRefresh();
            }, setBlockerError, { pending: "Adding blocker...", success: "Blocker added." })}>Add blocker</button>
          </div>
          {blockerError && <div className="field-error inline-error"><AlertCircle size={15} />{blockerError}</div>}
        </div>
      </section>

      <section className="panel">
        <div className="section-title"><h3>Comments</h3><span>{issue.comment_count}</span></div>
        {commentLoadError && <RequestError message={commentLoadError} onRetry={loadComments} onDismiss={() => setCommentLoadError("")} compact />}
        {showCommentsLoading && <LoadingStatus message="Loading comments..." compact />}
        <div className="comments">
          {comments.length === 0 && !commentsLoading ? (
            <EmptyState title="No comments yet" description="Add the first update or decision for this issue." compact />
          ) : comments.map((item) => <CommentView key={item.id} comment={item} />)}
        </div>
        <div className="section-title compact-title"><h3>Add comment</h3><Segment value={commentTab} onChange={setCommentTab} /></div>
        {commentTab === "source" ? (
          <textarea ref={commentRef} className="editor-surface" value={comment} onChange={(e) => {
            setComment(e.target.value);
            setCommentError("");
            setActionNotice(null);
          }} placeholder="Add a Markdown comment..." rows={4} />
        ) : (
          <div className="comment-preview"><Markdown text={comment || "_No comment yet._"} /></div>
        )}
        {commentError && <div className="field-error inline-error"><AlertCircle size={15} />{commentError}</div>}
        <div className="detail-actions">
          <ImageUploadControl username={username} disabled={actionBusy} onUploaded={(markdown) => insertAtCursor(commentRef, comment, setComment, markdown)} />
          <button className="button primary" disabled={actionBusy} onClick={() => {
            if (!comment.trim()) {
              setCommentError("Comment body is required.");
              return;
            }
            runAction(async () => {
              await api(`/api/issues/${issue.id}/comments`, { method: "POST", body: { body_markdown: comment }, username });
              setComment("");
              setCommentTab("source");
              setCommentError("");
              await onRefresh();
            }, setCommentError, { pending: "Posting comment...", success: "Comment added." });
          }}><Send size={16} />{actionBusy ? "Saving..." : "Add comment"}</button>
        </div>
      </section>

      <button className="button ghost" onClick={onClose}>Back to board</button>
    </div>
  );
}

function PickerFeedback({ visibleCount, totalCount, query, emptyLabel, preservedLabel }: { visibleCount: number; totalCount: number; query: string; emptyLabel: string; preservedLabel?: string }) {
  const hasQuery = Boolean(query.trim());
  const label = hasQuery
    ? visibleCount === 0
      ? emptyLabel
      : `Showing ${visibleCount} of ${totalCount} candidates.`
    : `${totalCount} candidates available.`;
  return <div className={`picker-feedback ${visibleCount === 0 ? "empty" : ""}`}>
    <span>{label}</span>
    {preservedLabel && <span>{preservedLabel}</span>}
  </div>;
}

function RelationshipList({ title, issues, onRemove, onOpen }: { title: string; issues: Issue[]; onRemove?: (issue: Issue) => void; onOpen?: (id: string) => void }) {
  return <div className="relationship-list">
    <h4>{title}</h4>
    {issues.length === 0 ? (
      <EmptyState title={`No ${title.toLowerCase()}`} description="No linked issues in this group." compact />
    ) : issues.map((issue) => <RelationshipItem key={issue.id} issue={issue} onRemove={onRemove} onOpen={onOpen} />)}
  </div>;
}

function BlockerRelationshipList({ issues, onRemove, onOpen }: { issues: Issue[]; onRemove: (issue: Issue) => void; onOpen?: (id: string) => void }) {
  const unresolved = issues.filter((issue) => !isResolved(issue));
  const resolved = issues.filter(isResolved);
  return <div className="relationship-list">
    <h4>Blocked by</h4>
    {issues.length === 0 && <EmptyState title="No blockers" description="This issue has no dependency blockers." compact />}
    {unresolved.length > 0 && <div className="relationship-group">
      <span>Unresolved blockers</span>
      {unresolved.map((issue) => <RelationshipItem key={issue.id} issue={issue} onRemove={onRemove} onOpen={onOpen} />)}
    </div>}
    {resolved.length > 0 && <div className="relationship-group">
      <span>Completed or canceled blockers</span>
      {resolved.map((issue) => <RelationshipItem key={issue.id} issue={issue} onRemove={onRemove} onOpen={onOpen} />)}
    </div>}
  </div>;
}

function RelationshipItem({ issue, onRemove, onOpen }: { issue: Issue; onRemove?: (issue: Issue) => void; onOpen?: (id: string) => void }) {
  return <div className="relationship-item">
    <button className="relationship-link" onClick={() => onOpen?.(issue.id)}>{issue.title}</button>
    <Badge tone={isResolved(issue) ? "good" : issue.blocked ? "danger" : "neutral"}>{statusLabel(issue.status)}</Badge>
    <Badge tone={issue.priority === "P0" || issue.priority === "P1" ? "danger" : "neutral"}>{issue.priority}</Badge>
    {onRemove && <button className="icon-button small" onClick={() => onRemove(issue)} aria-label={`Remove ${issue.title}`}><X size={15} /></button>}
  </div>;
}

function isResolved(issue: Issue) {
  return issue.status === "completed" || issue.status === "canceled";
}

function includeSelectedIssue(visible: Issue[], selectedID: string, allCandidates: Issue[]) {
  if (!selectedID || visible.some((issue) => issue.id === selectedID)) return visible;
  const selected = allCandidates.find((issue) => issue.id === selectedID);
  return selected ? [selected, ...visible] : visible;
}

function isDescendantOf(candidateID: string, ancestorID: string, issues: Issue[]) {
  let current = issues.find((issue) => issue.id === candidateID);
  const visited = new Set<string>();
  while (current?.parent_issue_id) {
    if (current.parent_issue_id === ancestorID) return true;
    if (visited.has(current.parent_issue_id)) return false;
    visited.add(current.parent_issue_id);
    current = issues.find((issue) => issue.id === current?.parent_issue_id);
  }
  return false;
}

function issueOptionLabel(issue: Issue) {
  const assignee = issue.assignee ? `@${issue.assignee}` : "unassigned";
  return `${issue.title} - ${issue.priority} - ${statusLabel(issue.status)} - ${assignee}`;
}

function matchesIssueSearch(issue: Issue, query: string) {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return true;
  const haystack = [
    issue.title,
    issue.description_markdown,
    issue.id,
    issue.priority,
    statusLabel(issue.status),
    issue.created_by,
    issue.assignee || "",
    ...(issue.tags || []).map((tag) => tag.name),
    ...(issue.recent_comments || []).map((comment) => comment.body_markdown),
    ...(issue.children || []).map((child) => child.title),
    ...(issue.blockers || []).map((blocker) => blocker.title),
    ...(issue.blocked_by || []).map((blocked) => blocked.title),
  ].join(" ").toLowerCase();
  return haystack.includes(normalized);
}

function Hierarchy({ issues, onOpen }: { issues: Issue[]; onOpen: (id: string) => void }) {
  if (issues.length === 0) {
    return <section className="empty-state"><h2>No hierarchy yet</h2><p>Create parent and child issues to build a planning tree.</p></section>;
  }
  const roots = issues.filter((issue) => !issue.parent_issue_id);
  return <div className="planning">{roots.map((issue) => <TreeNode key={issue.id} issue={issue} all={issues} onOpen={onOpen} depth={0} />)}</div>;
}

function TreeNode({ issue, all, onOpen, depth }: { issue: Issue; all: Issue[]; onOpen: (id: string) => void; depth: number }) {
  const children = all.filter((child) => child.parent_issue_id === issue.id);
  return <div className="tree-node" style={{ marginLeft: depth * 14 }}>
    <button className="tree-node-button" onClick={() => onOpen(issue.id)}>
      <CircleDot size={16} />
      <span className="tree-node-title">{issue.title}</span>
      <IssueMeta issue={issue} compact />
    </button>
    {children.map((child) => <TreeNode key={child.id} issue={child} all={all} onOpen={onOpen} depth={depth + 1} />)}
  </div>;
}

function Blockers({ issues, onOpen }: { issues: Issue[]; onOpen: (id: string) => void }) {
  const dependencyIssues = issues;
  const blockedIssues = dependencyIssues.filter((issue) => issue.blocked);
  const blockingIssues = dependencyIssues.filter((issue) => !isResolved(issue) && unresolvedIssues(issue.blocked_by).length > 0);
  const resolvedDependencyIssues = dependencyIssues.filter((issue) => !issue.blocked && resolvedIssues(issue.blockers).length > 0);
  const inactiveBlockingIssues = dependencyIssues.filter((issue) => isResolved(issue) && (issue.blocked_by?.length || 0) > 0);
  const resolvedBlockingIssues = dependencyIssues.filter((issue) => !isResolved(issue) && unresolvedIssues(issue.blocked_by).length === 0 && resolvedIssues(issue.blocked_by).length > 0);

  return <div className="planning">
    <div className="board-stats">
      <Stat label="Blocked" value={blockedIssues.length} tone="danger" />
      <Stat label="Blocking" value={blockingIssues.length} />
      <Stat label="Unblocked" value={dependencyIssues.filter((i) => !i.blocked).length} tone="good" />
    </div>
    {blockedIssues.length === 0 && blockingIssues.length === 0 && resolvedDependencyIssues.length === 0 && inactiveBlockingIssues.length === 0 && resolvedBlockingIssues.length === 0 && <section className="empty-state"><h2>No blockers</h2><p>No unresolved dependency relationships yet.</p></section>}
    {blockedIssues.map((issue) => (
      <article className="dependency" key={`blocked-${issue.id}`}>
        <button className="dependency-heading" onClick={() => onOpen(issue.id)}>
          <div>
            <h3>{issue.title}</h3>
            <IssueMeta issue={issue} />
          </div>
          <Badge tone="danger">Blocked</Badge>
        </button>
        <div className="dependency-line danger"><Blocks size={16} /><span>Blocked by {relationshipTitles(unresolvedIssues(issue.blockers), "unresolved blocker")}</span></div>
        {resolvedIssues(issue.blockers).length > 0 && <div className="dependency-line resolved"><Check size={16} /><span>Resolved blockers: {relationshipTitles(resolvedIssues(issue.blockers), "none")}</span></div>}
      </article>
    ))}
    {blockingIssues.map((issue) => (
      <article className="dependency" key={`blocking-${issue.id}`}>
        <button className="dependency-heading" onClick={() => onOpen(issue.id)}>
          <div>
            <h3>{issue.title}</h3>
            <IssueMeta issue={issue} />
          </div>
          <Badge>Blocking</Badge>
        </button>
        <div className="dependency-line neutral"><ChevronRight size={16} /><span>Blocking {relationshipTitles(unresolvedIssues(issue.blocked_by), "no active dependent issues")}</span></div>
        {resolvedIssues(issue.blocked_by).length > 0 && <div className="dependency-line resolved"><Check size={16} /><span>Resolved dependents: {relationshipTitles(resolvedIssues(issue.blocked_by), "none")}</span></div>}
      </article>
    ))}
    {(resolvedDependencyIssues.length > 0 || inactiveBlockingIssues.length > 0 || resolvedBlockingIssues.length > 0) && <section className="resolved-dependencies">
      <h3>Resolved dependencies</h3>
      {resolvedDependencyIssues.map((issue) => (
        <button className="resolved-row" key={`resolved-blocked-${issue.id}`} onClick={() => onOpen(issue.id)}>
          <span>{issue.title}</span>
          <small>Resolved blockers: {relationshipTitles(resolvedIssues(issue.blockers), "none")}</small>
        </button>
      ))}
      {inactiveBlockingIssues.map((issue) => (
        <button className="resolved-row" key={`inactive-blocking-${issue.id}`} onClick={() => onOpen(issue.id)}>
          <span>{issue.title}</span>
          <small>No longer blocking: {relationshipTitles(issue.blocked_by, "no dependent issues")}</small>
        </button>
      ))}
      {resolvedBlockingIssues.map((issue) => (
        <button className="resolved-row" key={`resolved-blocking-${issue.id}`} onClick={() => onOpen(issue.id)}>
          <span>{issue.title}</span>
          <small>Resolved dependents: {relationshipTitles(resolvedIssues(issue.blocked_by), "none")}</small>
        </button>
      ))}
    </section>}
  </div>;
}

function Profile({ username, onLogout, onTagsChanged }: { username: string; onLogout: () => void; onTagsChanged: () => Promise<void> }) {
  const [tags, setTags] = useState<Tag[]>([]);
  const [name, setName] = useState("");
  const [color, setColor] = useState("#b5f4d8");
  const [error, setError] = useState("");
  const [nameError, setNameError] = useState("");
  const [colorError, setColorError] = useState("");
  const [busy, setBusy] = useState(false);
  const showLoading = useDelayedBusy(busy);

  async function refreshTags() {
    const next = await api("/api/tags");
    setTags(next);
  }

  async function reloadTags() {
    setError("");
    setBusy(true);
    try {
      await Promise.all([refreshTags(), onTagsChanged()]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load tags.");
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    reloadTags();
  }, []);

  async function runTagAction(fn: () => Promise<void>) {
    if (busy) return;
    setError("");
    setBusy(true);
    try {
      await fn();
      await Promise.all([refreshTags(), onTagsChanged()]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Tag request failed.");
    } finally {
      setBusy(false);
    }
  }

  return <div className="profile-stack">
    <section className="panel profile"><User size={28} /><h2>{username}</h2><p>Used for REST mutations and comments on this local Tala instance.</p><button className="button" onClick={onLogout}>Change username</button></section>
    <section className="panel tag-admin">
      <div className="section-title"><h3>Tags</h3><span>{tags.length}</span></div>
      {error && <RequestError message={error} onRetry={reloadTags} onDismiss={() => setError("")} compact />}
      {showLoading && <LoadingStatus message="Loading tag changes..." compact />}
      <div className="inline-controls">
        <input className={nameError ? "invalid" : ""} value={name} onChange={(e) => {
          setName(e.target.value);
          setNameError("");
          setError("");
        }} placeholder="Tag name" />
        <ColorControl value={color} disabled={busy} invalid={Boolean(colorError)} onChange={(value) => {
          setColor(value);
          setColorError("");
          setError("");
        }} ariaLabel="Tag color" />
      </div>
      {nameError && <div className="field-error"><AlertCircle size={15} />{nameError}</div>}
      {colorError && <div className="field-error"><AlertCircle size={15} />{colorError}</div>}
      <button className="button primary" disabled={busy} onClick={() => runTagAction(async () => {
        const tagName = name.trim();
        const tagColor = color.trim();
        if (!tagName) {
          setNameError("Tag name is required.");
          return;
        }
        if (!isValidTagColor(tagColor)) {
          setColorError("Use a color token or hex value like #b5f4d8.");
          return;
        }
        await api("/api/tags", { method: "POST", username, body: { name: tagName, color: tagColor || null } });
        setName("");
      })}>Create tag</button>
      <div className="tag-list">
        {tags.length === 0 && !busy ? (
          <EmptyState title="No tags yet" description="Create a tag to reuse it on issue cards and filters." compact />
        ) : tags.map((tag) => <TagEditor key={tag.id} tag={tag} username={username} onSaved={async () => {
          await Promise.all([refreshTags(), onTagsChanged()]);
        }} onError={setError} />)}
      </div>
    </section>
  </div>;
}

function TagEditor({ tag, username, onSaved, onError }: { tag: Tag; username: string; onSaved: () => Promise<void>; onError: (message: string) => void }) {
  const [name, setName] = useState(tag.name);
  const [color, setColor] = useState(tag.color || "");
  const [nameError, setNameError] = useState("");
  const [colorError, setColorError] = useState("");
  const [busy, setBusy] = useState(false);
  const showLoading = useDelayedBusy(busy);

  useEffect(() => {
    setName(tag.name);
    setColor(tag.color || "");
    setNameError("");
    setColorError("");
  }, [tag.id, tag.name, tag.color]);

  async function save(nextColor: string | null = color || null) {
    if (busy) return;
    const cleanName = name.trim();
    const cleanColor = nextColor?.trim() || "";
    if (!cleanName) {
      setNameError("Tag name is required.");
      return;
    }
    if (!isValidTagColor(cleanColor)) {
      setColorError("Use a color token or hex value like #b5f4d8.");
      return;
    }
    onError("");
    setNameError("");
    setColorError("");
    setBusy(true);
    try {
      await api(`/api/tags/${tag.id}`, { method: "PATCH", username, body: { name: cleanName, color: cleanColor || null } });
      await onSaved();
    } catch (err) {
      onError(err instanceof Error ? err.message : "Unable to save tag.");
    } finally {
      setBusy(false);
    }
  }

  return <div className="tag-editor">
    <span className="tag" style={tagStyle({ ...tag, name, color })}>{name}</span>
    <input className={nameError ? "invalid" : ""} value={name} disabled={busy} onChange={(e) => {
      setName(e.target.value);
      setNameError("");
      onError("");
    }} aria-label={`Name for ${tag.name}`} />
    {nameError && <div className="field-error"><AlertCircle size={15} />{nameError}</div>}
    <ColorControl value={color} disabled={busy} invalid={Boolean(colorError)} onChange={(value) => {
      setColor(value);
      setColorError("");
      onError("");
    }} ariaLabel={`Color for ${tag.name}`} />
    {colorError && <div className="field-error"><AlertCircle size={15} />{colorError}</div>}
    {showLoading && <LoadingStatus message="Saving tag..." compact />}
    <div className="tag-actions">
      <button className="button" disabled={busy} onClick={() => save()}>Save</button>
      <button className="button ghost" onClick={() => {
        setColor("");
        save(null);
      }} disabled={busy}>Clear</button>
    </div>
  </div>;
}

function ColorControl({ value, disabled, invalid = false, onChange, ariaLabel }: { value: string; disabled: boolean; invalid?: boolean; onChange: (value: string) => void; ariaLabel: string }) {
  return <div className="color-control">
    <input className={`color-input ${invalid ? "invalid" : ""}`} value={value} disabled={disabled} onChange={(e) => onChange(e.target.value)} aria-label={ariaLabel} />
    <div className="color-swatches" aria-label={`${ariaLabel} swatches`}>
      {tagColorChoices.map((token) => (
        <button
          key={token}
          type="button"
          className={value === token ? "color-swatch selected" : "color-swatch"}
          style={{ backgroundColor: tagColorTokens[token] }}
          disabled={disabled}
          title={token}
          aria-label={`Use ${token}`}
          onClick={() => onChange(token)}
        />
      ))}
    </div>
  </div>;
}

function FilterSheet({ filters, tags, issues, onApplyFilters, onClose }: { filters: IssueFilters; tags: Tag[]; issues: Issue[]; onApplyFilters: (filters: IssueFilters) => void; onClose: () => void }) {
  const [local, setLocal] = useState(filters);
  const activeCount = filterKeys.filter((key) => Boolean(local[key])).length;
  const relationshipCount = [local.parent_id, local.blocked_by, local.blocker_of, local.id].filter(Boolean).length;
  const clearLocalFilters = () => setLocal(emptyFilters());

  return <Sheet title="Filters" onClose={onClose}>
    <div className="filter-summary">
      <div>
        <strong>{activeCount === 0 ? "No filters active" : `${activeCount} active ${activeCount === 1 ? "filter" : "filters"}`}</strong>
        <span>{issues.length} issues available for relationship filters</span>
      </div>
      {activeCount > 0 && <button type="button" className="button ghost compact" onClick={clearLocalFilters}>Clear</button>}
    </div>
    <section className="filter-section">
      <h3>Find</h3>
      <label>Search</label><input value={local.q} onChange={(e) => setLocal({ ...local, q: e.target.value })} placeholder="Markdown, title, or keyword" />
      <label>Assignee</label><input value={local.assignee} onChange={(e) => setLocal({ ...local, assignee: e.target.value })} placeholder="alex" />
      <label>Tag</label><select value={local.tag} onChange={(e) => setLocal({ ...local, tag: e.target.value })}><option value="">Any</option>{tags.map((tag) => <option key={tag.id} value={tag.name}>{tag.name}</option>)}</select>
      {tags.length > 0 && <div className="filter-tags">
        {tags.map((tag) => <button key={tag.id} type="button" aria-label={`Filter by tag ${tag.name}`} className={`tag ${local.tag === tag.name ? "selected" : ""}`} style={tagStyle(tag)} onClick={() => setLocal({ ...local, tag: local.tag === tag.name ? "" : tag.name })}>{tag.name}</button>)}
      </div>}
    </section>
    <section className="filter-section">
      <h3>State</h3>
      <label>State</label><select value={local.state} onChange={(e) => setLocal({ ...local, state: e.target.value })}><option value="">Any</option>{states.map((state) => <option key={state} value={state}>{state}</option>)}</select>
      <label>Status</label><select value={local.status || ""} onChange={(e) => setLocal({ ...local, status: e.target.value })}><option value="">Any</option>{statuses.map((status) => <option key={status} value={status}>{statusLabel(status)}</option>)}</select>
      <label>Priority</label><select value={local.priority} onChange={(e) => setLocal({ ...local, priority: e.target.value })}><option value="">Any</option>{priorities.map((p) => <option key={p}>{p}</option>)}</select>
    </section>
    <section className="filter-section">
      <h3>Relationships</h3>
      <div className="filter-section-meta">{relationshipCount === 0 ? "Any relationship" : `${relationshipCount} active relationship ${relationshipCount === 1 ? "filter" : "filters"}`}</div>
      <label>Parent</label><select value={local.parent_id} onChange={(e) => setLocal({ ...local, parent_id: e.target.value })}><option value="">Any</option>{issues.map((issue) => <option key={issue.id} value={issue.id}>{issueOptionLabel(issue)}</option>)}</select>
      <label>Blocked by</label><select value={local.blocked_by} onChange={(e) => setLocal({ ...local, blocked_by: e.target.value })}><option value="">Any</option>{issues.map((issue) => <option key={issue.id} value={issue.id}>{issueOptionLabel(issue)}</option>)}</select>
      <label>Blocker of</label><select value={local.blocker_of} onChange={(e) => setLocal({ ...local, blocker_of: e.target.value })}><option value="">Any</option>{issues.map((issue) => <option key={issue.id} value={issue.id}>{issueOptionLabel(issue)}</option>)}</select>
      <label>Exact issue</label><select value={local.id} onChange={(e) => setLocal({ ...local, id: e.target.value })}><option value="">Any</option>{issues.map((issue) => <option key={issue.id} value={issue.id}>{issueOptionLabel(issue)}</option>)}</select>
    </section>
    <section className="filter-section">
      <h3>Sort</h3>
      <label>Sort by</label><select value={local.sort} onChange={(e) => setLocal({ ...local, sort: e.target.value })}>{sortOptions.map((option) => <option key={option.value || "default"} value={option.value}>{option.label}</option>)}</select>
      <label>Sort order</label><select value={local.order} onChange={(e) => setLocal({ ...local, order: e.target.value })}><option value="">Default</option><option value="asc">Ascending</option><option value="desc">Descending</option></select>
    </section>
    <div className="sheet-actions"><button className="button" onClick={() => { onApplyFilters(emptyFilters()); onClose(); }}>Reset</button><button className="button primary" onClick={() => { onApplyFilters(local); onClose(); }}>Apply</button></div>
  </Sheet>;
}

function CreateSheet({ username, issues, onClose, onCreated }: { username: string; issues: Issue[]; onClose: () => void; onCreated: (issue: Issue) => void }) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [descriptionTab, setDescriptionTab] = useState<"source" | "preview">("source");
  const [priority, setPriority] = useState<Priority>("P2");
  const [assignee, setAssignee] = useState("");
  const [parentID, setParentID] = useState("");
  const [parentQuery, setParentQuery] = useState("");
  const [tags, setTags] = useState("");
  const [titleError, setTitleError] = useState("");
  const [submitError, setSubmitError] = useState("");
  const [submitNotice, setSubmitNotice] = useState<ActionNotice | null>(null);
  const [creating, setCreating] = useState(false);
  const descriptionRef = useRef<HTMLTextAreaElement | null>(null);
  const showCreating = useDelayedBusy(creating);
  const parentCandidates = includeSelectedIssue(
    issues.filter((issue) => matchesIssueSearch(issue, parentQuery)),
    parentID,
    issues,
  );
  return <Sheet title="Create issue" onClose={onClose}>
    <label>Title</label><input className={titleError ? "invalid" : ""} value={title} onChange={(e) => {
      setTitle(e.target.value);
      setTitleError("");
      setSubmitError("");
      setSubmitNotice(null);
    }} placeholder="Issue title" />
    {titleError && <div className="field-error"><AlertCircle size={15} />{titleError}</div>}
    <div className="section-title compact-title"><label>Description Markdown</label><Segment value={descriptionTab} onChange={setDescriptionTab} /></div>
    {descriptionTab === "source" ? (
      <textarea ref={descriptionRef} value={description} onChange={(e) => setDescription(e.target.value)} rows={6} placeholder="Describe the issue..." />
    ) : (
      <div className="comment-preview"><Markdown text={description || "_No description yet._"} /></div>
    )}
    <ImageUploadControl username={username} disabled={creating} onUploaded={(markdown) => insertAtCursor(descriptionRef, description, setDescription, markdown)} />
    <label>Priority</label><select value={priority} onChange={(e) => setPriority(e.target.value as Priority)}>{priorities.map((p) => <option key={p}>{p}</option>)}</select>
    <label>Assignee</label><input value={assignee} onChange={(e) => setAssignee(e.target.value)} placeholder="Unassigned" />
    <label>Parent issue</label>
    <div className="relationship-control">
      <input value={parentQuery} onChange={(e) => setParentQuery(e.target.value)} placeholder="Search parent issues..." aria-label="Search parent issues" />
    </div>
    <select value={parentID} onChange={(e) => setParentID(e.target.value)}>
      <option value="">No parent</option>
      {parentCandidates.map((issue) => <option key={issue.id} value={issue.id}>{issue.title}</option>)}
    </select>
    <label>Tags</label><input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="mcp, api" />
    {submitError && <div className="field-error"><AlertCircle size={15} />{submitError}</div>}
    {showCreating && <LoadingStatus message="Creating issue..." compact />}
    {submitNotice && <ActionStatus notice={submitNotice} />}
    <button className="button primary" disabled={creating} onClick={async () => {
      setTitleError("");
      setSubmitError("");
      setSubmitNotice(null);
      if (!title.trim()) return setTitleError("Title is required.");
      setCreating(true);
      try {
        const issue = await api("/api/issues", { method: "POST", username, body: { title, description_markdown: description, priority, assignee: optionalText(assignee), parent_issue_id: parentID || null, tag_names: splitTags(tags) } });
        setSubmitNotice({ tone: "success", message: "Issue created." });
        onCreated(issue);
      } catch (err) {
        setSubmitNotice(null);
        setSubmitError(err instanceof Error ? err.message : "Unable to create issue.");
      } finally {
        setCreating(false);
      }
    }}><Plus size={16} />Create issue</button>
  </Sheet>;
}

function Sheet({ title, children, onClose }: { title: string; children: React.ReactNode; onClose: () => void }) {
  return <div className="sheet-backdrop"><section className="sheet"><div className="sheet-header"><h2>{title}</h2><button className="icon-button" aria-label="Close" onClick={onClose}><X size={20} /></button></div>{children}</section></div>;
}

function EmptyState({ title, description, compact = false }: { title: string; description: string; compact?: boolean }) {
  return <section className={`empty-state ${compact ? "compact" : ""}`}>
    <h2>{title}</h2>
    <p>{description}</p>
  </section>;
}

function CommentView({ comment }: { comment: Comment }) {
  return <article className="comment"><div><strong>{comment.author}</strong><span>{new Date(comment.created_at).toLocaleString()}</span></div><Markdown text={comment.body_markdown} /></article>;
}

function ImageUploadControl({ username, disabled, onUploaded }: { username: string; disabled?: boolean; onUploaded: (markdown: string) => void }) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState("");
  const showUploading = useDelayedBusy(uploading);
  return <div className="image-upload-control">
    <input ref={inputRef} type="file" accept="image/png,image/jpeg,image/gif,image/webp" hidden onChange={async (event) => {
      const file = event.target.files?.[0];
      event.target.value = "";
      if (!file) return;
      setUploading(true);
      setError("");
      try {
        const uploaded = await uploadImage(file, username);
        onUploaded(uploaded.markdown || `![${file.name}](${uploaded.url})`);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unable to upload image.");
      } finally {
        setUploading(false);
      }
    }} />
    <button type="button" className="button compact" disabled={disabled || uploading} onClick={() => inputRef.current?.click()}><ImagePlus size={15} />Image</button>
    {showUploading && <LoadingStatus message="Uploading image..." compact />}
    {error && <div className="field-error inline-error"><AlertCircle size={15} />{error}</div>}
  </div>;
}

function Markdown({ text }: { text: string }) {
  return <div className="markdown"><ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeSanitize]}>{text}</ReactMarkdown></div>;
}

function Segment<T extends string>({ value, onChange }: { value: T; onChange: (value: T) => void }) {
  return <div className="segment"><button className={value === "source" ? "active" : ""} onClick={() => onChange("source" as T)}>Source</button><button className={value === "preview" ? "active" : ""} onClick={() => onChange("preview" as T)}>Preview</button></div>;
}

function NavButton({ active, icon, label, onClick }: { active: boolean; icon: React.ReactElement; label: string; onClick: () => void }) {
  return <button className={active ? "active" : ""} onClick={onClick}>{React.cloneElement(icon, { size: 20 } as any)}<span>{label}</span></button>;
}

function IssueMeta({ issue, compact = false }: { issue: Issue; compact?: boolean }) {
  return <div className={`issue-meta ${compact ? "compact" : ""}`}>
    <Badge tone={issue.priority === "P0" || issue.priority === "P1" ? "danger" : "neutral"}>{issue.priority}</Badge>
    <Badge tone={isResolved(issue) ? "good" : issue.blocked ? "danger" : "neutral"}>{statusLabel(issue.status)}</Badge>
    {!compact && <Badge>{issue.assignee || "Unassigned"}</Badge>}
    {issue.blocked && <Badge tone="danger">Blocked</Badge>}
    {issue.child_count > 0 && <span><GitBranch size={13} />{issue.child_count}</span>}
    {issue.comment_count > 0 && <span><MessageSquare size={13} />{issue.comment_count}</span>}
    {!compact && <TagRow tags={issue.tags || []} limit={3} />}
  </div>;
}

function TagRow({ tags, limit }: { tags: Tag[]; limit: number }) {
  if (tags.length === 0) return null;
  const visible = tags.slice(0, limit);
  const hidden = tags.length - visible.length;
  return <div className="tag-row">
    {visible.map((tag) => <span className="tag" style={tagStyle(tag)} key={tag.id}>{tag.name}</span>)}
    {hidden > 0 && <span className="tag tag-overflow" aria-label={`${hidden} more tags`}>+{hidden}</span>}
  </div>;
}

function Badge({ children, tone = "neutral" }: { children: React.ReactNode; tone?: "neutral" | "danger" | "good" }) {
  return <span className={`badge ${tone}`}>{children}</span>;
}

function Stat({ label, value, tone = "neutral", active = false, onClick }: { label: string; value: number; tone?: "neutral" | "danger" | "good"; active?: boolean; onClick?: () => void }) {
  const content = <><strong>{value}</strong><span>{label}</span></>;
  if (onClick) {
    return <button className={`stat stat-button ${tone} ${active ? "active" : ""}`} onClick={onClick}>{content}</button>;
  }
  return <div className={`stat ${tone}`}>{content}</div>;
}

async function api(path: string, options: { method?: string; body?: unknown; username?: string } = {}) {
  const res = await fetch(path, {
    method: options.method || "GET",
    headers: { "Content-Type": "application/json", ...(options.username ? { "X-Tala-Username": options.username } : {}) },
    body: options.body === undefined ? undefined : JSON.stringify(options.body)
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error?.message || "Request failed.");
  return data;
}

async function uploadImage(file: File, username: string): Promise<UploadedImage> {
  const form = new FormData();
  form.append("image", file);
  const res = await fetch("/api/uploads/images", {
    method: "POST",
    headers: username ? { "X-Tala-Username": username } : undefined,
    body: form
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error?.message || "Unable to upload image.");
  return data;
}

function insertAtCursor(ref: React.RefObject<HTMLTextAreaElement | null>, value: string, setValue: React.Dispatch<React.SetStateAction<string>>, inserted: string) {
  const target = ref.current;
  if (!target) {
    setValue((current) => current ? `${current}\n\n${inserted}` : inserted);
    return;
  }
  const start = target.selectionStart ?? value.length;
  const end = target.selectionEnd ?? value.length;
  const spacerBefore = start > 0 && !value.slice(0, start).endsWith("\n") ? "\n\n" : "";
  const spacerAfter = end < value.length && !value.slice(end).startsWith("\n") ? "\n\n" : "";
  const next = `${value.slice(0, start)}${spacerBefore}${inserted}${spacerAfter}${value.slice(end)}`;
  const cursor = start + spacerBefore.length + inserted.length;
  setValue(next);
  window.requestAnimationFrame(() => {
    target.focus();
    target.setSelectionRange(cursor, cursor);
  });
}

function storedUsername() {
  const stored = localStorage.getItem("tala.username");
  const trimmed = stored?.trim() || "";
  if (stored && stored !== trimmed) {
    if (trimmed) {
      localStorage.setItem("tala.username", trimmed);
    } else {
      localStorage.removeItem("tala.username");
    }
  }
  return trimmed;
}

function viewTitle(view: View) {
  return view === "board" ? "Tala" : view === "hierarchy" ? "Hierarchy" : view === "blockers" ? "Blockers" : "Profile";
}

function emptyFilters() {
  return { q: "", status: "", assignee: "", priority: "", tag: "", id: "", parent_id: "", blocked_by: "", blocker_of: "", state: "", sort: "", order: "" };
}

function filtersFromLocation(): IssueFilters {
  const params = new URLSearchParams(window.location.search);
  const filters = emptyFilters();
  filterKeys.forEach((key) => {
    filters[key] = params.get(key) || "";
  });
  return filters;
}

function issueIDFromLocation() {
  const match = window.location.pathname.match(/^\/issues\/([^/]+)\/?$/);
  if (!match) return null;
  try {
    return decodeURIComponent(match[1]);
  } catch {
    return null;
  }
}

function issuePath(issueID: string) {
  return `/issues/${encodeURIComponent(issueID)}`;
}

function viewFromLocation(): View {
  switch (window.location.pathname.replace(/\/+$/, "") || "/") {
    case "/hierarchy":
      return "hierarchy";
    case "/blockers":
      return "blockers";
    case "/profile":
      return "profile";
    default:
      return "board";
  }
}

function viewPath(view: View) {
  return view === "board" ? "/" : `/${view}`;
}

function pathWithFilters(view: View, filters: IssueFilters) {
  const path = viewPath(view);
  if (view !== "board") return path;
  const params = new URLSearchParams();
  filterKeys.forEach((key) => {
    const value = filters[key];
    if (value) params.set(key, value);
  });
  const query = params.toString();
  return query ? `${path}?${query}` : path;
}

function hasActiveFilters(filters: IssueFilters) {
  return Object.values(filters).some(Boolean);
}

function statusLabel(status: Status) {
  return status.replace("_", " ");
}

function shortID(id: string) {
  return "#" + id.replace(/^issue_/, "").slice(0, 4);
}

function formatDateTime(value: string) {
  return new Date(value).toLocaleString([], { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

function splitTags(value: string) {
  return value.split(",").map((tag) => tag.trim()).filter(Boolean);
}

function optionalText(value: string) {
  const trimmed = value.trim();
  return trimmed || null;
}

async function writeClipboard(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.select();
  try {
    if (!document.execCommand("copy")) {
      throw new Error("Copy command was not available.");
    }
  } finally {
    document.body.removeChild(textarea);
  }
}

function relationshipTitles(issues: Issue[] | undefined, fallback: string) {
  if (!issues || issues.length === 0) return fallback;
  return issues.map((issue) => issue.title).join(", ");
}

function unresolvedIssues(issues: Issue[] | undefined) {
  return (issues || []).filter((issue) => !isResolved(issue));
}

function resolvedIssues(issues: Issue[] | undefined) {
  return (issues || []).filter(isResolved);
}

function tagStyle(tag: Tag): React.CSSProperties | undefined {
  if (!tag.color) return undefined;
  const background = tagColorTokens[tag.color] || tag.color;
  const foreground = readableTextColor(background);
  return { backgroundColor: background, color: foreground };
}

function useDelayedBusy(active: boolean, delayMs = 500) {
  const [visible, setVisible] = useState(false);
  useEffect(() => {
    if (!active) {
      setVisible(false);
      return;
    }
    const timeout = window.setTimeout(() => setVisible(true), delayMs);
    return () => window.clearTimeout(timeout);
  }, [active, delayMs]);
  return visible;
}

function isValidTagColor(color: string) {
  if (!color) return true;
  return Boolean(tagColorTokens[color] || normalizedHexColor(color));
}

function readableTextColor(color: string) {
  const hex = normalizedHexColor(color);
  if (!hex) return undefined;
  const r = parseInt(hex.slice(0, 2), 16);
  const g = parseInt(hex.slice(2, 4), 16);
  const b = parseInt(hex.slice(4, 6), 16);
  return (r * 299 + g * 587 + b * 114) / 1000 > 150 ? "#191714" : "#fffdf8";
}

function normalizedHexColor(color: string) {
  const hex = color.trim().match(/^#?([0-9a-f]{3}|[0-9a-f]{6})$/i)?.[1];
  if (!hex) return undefined;
  return hex.length === 3 ? hex.split("").map((part) => part + part).join("") : hex;
}

createRoot(document.getElementById("root")!).render(<App />);
