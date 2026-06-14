import React, { useEffect, useRef, useState } from "react";
import { AlertCircle, Check, Link2, Pencil, RefreshCw, Send, X } from "lucide-react";
import type { ActionNotice, Comment, Issue, IssueFilters } from "../types";
import { ApiError, api } from "../api";
import { priorities, statuses } from "../constants";
import { useDelayedBusy } from "../hooks";
import { emptyFilters, insertAtCursor, optionalText, shortID, splitTags, statusLabel } from "../utils";
import { ActionStatus, Badge, CommentView, EmptyState, ImageUploadControl, LoadingStatus, Markdown, RequestError, Segment, TagRow } from "../components/common";

export function IssueDetail({ issue, detailLoading, username, issues, onOpenIssue, onApplyFilters, onRefresh, onTagsChanged, onClose }: { issue: Issue; detailLoading: boolean; username: string; issues: Issue[]; onOpenIssue: (id: string) => void; onApplyFilters: (filters: IssueFilters) => void; onRefresh: () => Promise<void>; onTagsChanged: () => Promise<void>; onClose: () => void }) {
  const [title, setTitle] = useState(issue.title);
  const [tagDraft, setTagDraft] = useState((issue.tags || []).map((tag) => tag.name).join(", "));
  const [assigneeDraft, setAssigneeDraft] = useState(issue.assignee || "");
  const [tab, setTab] = useState<"source" | "preview">("source");
  const [draft, setDraft] = useState(issue.description_markdown);
  const [editingInlineFields, setEditingInlineFields] = useState(false);
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
  const savedTagDraft = (issue.tags || []).map((tag) => tag.name).join(", ");

  useEffect(() => setTitle(issue.title), [issue.id, issue.title]);
  useEffect(() => setTagDraft(savedTagDraft), [issue.id, savedTagDraft]);
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
    setEditingInlineFields(false);
  }, [issue.id]);

  function resetInlineDrafts() {
    setTitle(issue.title);
    setTagDraft(savedTagDraft);
    setAssigneeDraft(issue.assignee || "");
    setDraft(issue.description_markdown);
    setTab("source");
    setTitleError("");
    setActionNotice(null);
  }

  function hasUnsavedInlineDrafts() {
    return title !== issue.title
      || tagDraft !== savedTagDraft
      || assigneeDraft !== (issue.assignee || "")
      || draft !== issue.description_markdown;
  }

  function closeInlineEditing() {
    if (hasUnsavedInlineDrafts() && !window.confirm("Discard unsaved field changes?")) return;
    resetInlineDrafts();
    setEditingInlineFields(false);
  }

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

  async function runAction(fn: () => Promise<void>, setInlineError?: (message: string) => void, feedback: { pending: string; success: string } = { pending: "Saving changes...", success: "Changes saved." }, routeError?: (err: unknown, message: string) => boolean) {
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
      if (routeError?.(err, message)) {
        return;
      }
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
  const unresolvedBlockers = (issue.blockers || []).filter((blocker) => !isResolved(blocker));
  const activeBlockedBy = activeBlockedByIssues(issue);
  const resolvedBlockedBy = resolvedBlockedByIssues(issue);
  const parentSelectionUnchanged = parentID === (issue.parent_issue_id || "");
  const selectedParentPreserved = Boolean(parentQuery.trim() && parentID && parentCandidates.some((candidate) => candidate.id === parentID) && !matchesIssueSearch(parentCandidates.find((candidate) => candidate.id === parentID)!, parentQuery));
  const blockerQueryHasMatches = blockerCandidates.length > 0;
  function applyRelationshipFilter(partial: Partial<IssueFilters>) {
    onApplyFilters({ ...emptyFilters(), ...partial });
  }

  function routeRelationshipError(err: unknown, message: string) {
    if (!(err instanceof ApiError)) return false;
    if (err.field === "parent_issue_id") {
      setParentError(message);
      return true;
    }
    if (err.field === "blocker_issue_id") {
      setBlockerError(message);
      return true;
    }
    return false;
  }

  return (
    <div className="detail">
      {actionError && <div className="field-error"><AlertCircle size={15} />{actionError}</div>}
      {detailLoading && <LoadingStatus message="Loading latest issue detail..." compact />}
      {showActionLoading && pendingMessage && <LoadingStatus message={pendingMessage} compact />}
      {actionNotice && <ActionStatus notice={actionNotice} />}
      <section className="detail-hero">
        <div className="card-title-row">
          <h2>{issue.title}</h2>
          <div className="detail-title-actions">
            <span>{shortID(issue.id)}</span>
            {editingInlineFields ? (
              <button className="button compact" disabled={actionBusy} onClick={closeInlineEditing}><Check size={15} />Done</button>
            ) : (
              <button className="button compact" disabled={actionBusy} onClick={() => setEditingInlineFields(true)}><Pencil size={15} />Edit</button>
            )}
          </div>
        </div>
        <div className="meta-row detail-meta-row">
          <Badge tone={isResolved(issue) ? "good" : issue.blocked ? "danger" : "neutral"}>{statusLabel(issue.status)}</Badge>
          <Badge tone={issue.priority === "P0" || issue.priority === "P1" ? "danger" : "neutral"}>{issue.priority}</Badge>
          <Badge>{issue.assignee || "Unassigned"}</Badge>
          <Badge>Created by {issue.created_by}</Badge>
        </div>
        <TagRow tags={issue.tags || []} limit={4} />
        {editingInlineFields && (
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
        )}
      </section>

      <section className="panel">
        <div className="section-title">
          <h3>Description</h3>
          {editingInlineFields && <Segment value={tab} onChange={setTab} />}
        </div>
        {editingInlineFields ? (
          tab === "source" ? (
            <textarea ref={descriptionRef} className="editor-surface" value={draft} onChange={(e) => {
              setDraft(e.target.value);
              setActionNotice(null);
            }} rows={7} />
          ) : (
            <div className="comment-preview editor-preview"><Markdown text={draft || "_No description yet._"} /></div>
          )
        ) : (
          <div className="comment-preview editor-preview"><Markdown text={issue.description_markdown || "_No description yet._"} /></div>
        )}
        {editingInlineFields && (
          <div className="detail-actions">
            <ImageUploadControl username={username} disabled={actionBusy} onUploaded={(markdown) => insertAtCursor(descriptionRef, draft, setDraft, markdown)} />
            <button className="button" disabled={actionBusy} onClick={() => patch({ description_markdown: draft }, { pending: "Saving description...", success: "Description saved." })}>Save description</button>
          </div>
        )}
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
          <button type="button" disabled={(issue.blockers?.length || 0) === 0} onClick={() => applyRelationshipFilter({ blocker_of: issue.id })}><strong>{unresolvedBlockers.length}</strong><span>Active blockers</span></button>
          <button type="button" disabled={(issue.blocked_by?.length || 0) === 0} onClick={() => applyRelationshipFilter({ blocked_by: issue.id })}><strong>{activeBlockedBy.length}</strong><span>Active dependents</span></button>
        </div>

        <div className="edit-field">
          <label>Parent issue</label>
          <div className="relationship-control">
            <input value={parentQuery} onChange={(e) => {
              setParentQuery(e.target.value);
              setParentError("");
            }} placeholder="Search parent issues..." aria-label="Search parent issues" />
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
              setParentError("");
            }}>
              <option value="">No parent</option>
              {parentCandidates.map((candidate) => <option key={candidate.id} value={candidate.id}>{issueOptionLabel(candidate)}</option>)}
            </select>
            <button className="button" disabled={actionBusy || parentSelectionUnchanged} onClick={() => runAction(async () => {
              await api(`/api/issues/${issue.id}/parent`, { method: "PUT", body: { parent_issue_id: parentID || null }, username });
              await onRefresh();
            }, setParentError, { pending: "Updating parent...", success: "Parent updated." }, routeRelationshipError)}>Set</button>
          </div>
          {parentError && <div className="field-error inline-error"><AlertCircle size={15} />{parentError}</div>}
        </div>

        <RelationshipList title="Parent" issues={currentParent ? [currentParent] : []} onOpen={onOpenIssue} />
        <RelationshipList title="Children" issues={issue.children || []} onOpen={onOpenIssue} />
        <BlockerRelationshipList issues={issue.blockers || []} onRemove={(blocker) => runAction(async () => {
          await api(`/api/issues/${issue.id}/blockers/${blocker.id}`, { method: "DELETE", username });
          await onRefresh();
        }, setBlockerError, { pending: "Removing blocker...", success: "Blocker removed." }, routeRelationshipError)} onOpen={onOpenIssue} />
        <BlockingRelationshipList active={activeBlockedBy} resolved={resolvedBlockedBy} currentIssueResolved={isResolved(issue)} onOpen={onOpenIssue} />

        <div className="edit-field">
          <label>Add blocker</label>
          <div className="relationship-control">
            <input value={blockerQuery} onChange={(e) => {
              setBlockerQuery(e.target.value);
              setBlockerError("");
            }} placeholder="Search blocker issues..." aria-label="Search blocker issues" />
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
            setBlockerError("");
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
            }, setBlockerError, { pending: "Adding blocker...", success: "Blocker added." }, routeRelationshipError)}>Add blocker</button>
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

function BlockingRelationshipList({ active, resolved, currentIssueResolved, onOpen }: { active: Issue[]; resolved: Issue[]; currentIssueResolved: boolean; onOpen?: (id: string) => void }) {
  return <div className="relationship-list">
    <h4>Blocking</h4>
    {active.length === 0 && resolved.length === 0 && <EmptyState title="No blocked dependents" description="No issues depend on this one." compact />}
    {active.length > 0 && <div className="relationship-group">
      <span>Actively blocking</span>
      {active.map((issue) => <RelationshipItem key={issue.id} issue={issue} onOpen={onOpen} />)}
    </div>}
    {resolved.length > 0 && <div className="relationship-group">
      <span>{currentIssueResolved ? "Resolved because this issue is completed or canceled" : "Completed or canceled dependents"}</span>
      {resolved.map((issue) => <RelationshipItem key={issue.id} issue={issue} onOpen={onOpen} />)}
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

function activeBlockedByIssues(issue: Issue) {
  if (isResolved(issue)) return [];
  return (issue.blocked_by || []).filter((blockedBy) => !isResolved(blockedBy));
}

function resolvedBlockedByIssues(issue: Issue) {
  return (issue.blocked_by || []).filter((blockedBy) => isResolved(issue) || isResolved(blockedBy));
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
