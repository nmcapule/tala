import React, { useState } from "react";
import { ChevronDown, ChevronRight, Clock, GitBranch, MessageSquare, User } from "lucide-react";
import type { Issue, IssueFilters, Status } from "../types";
import { api } from "../api";
import { statuses } from "../constants";
import { useDelayedBusy } from "../hooks";
import { emptyFilters, formatDateTime, isResolved, shortID, statusLabel } from "../utils";
import { Badge, EmptyState, LoadingStatus, RequestError, Stat, TagRow } from "../components/common";

export function Board({ issues, totalIssues, filters, hasFilters, loading, showLoading, username, onOpen, onRefresh, onResetFilters, onApplyFilters }: { issues: Issue[]; totalIssues: number; filters: IssueFilters; hasFilters: boolean; loading: boolean; showLoading: boolean; username: string; onOpen: (id: string) => void; onRefresh: () => Promise<void>; onResetFilters: () => void; onApplyFilters: (filters: IssueFilters) => void }) {
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
