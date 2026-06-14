import { useRef, useState } from "react";
import { AlertCircle, Plus } from "lucide-react";
import type { ActionNotice, Issue, IssueFilters, Priority, Tag } from "../types";
import { api } from "../api";
import { filterKeys, priorities, sortOptions, states, statuses, storyPointChoices } from "../constants";
import { useDelayedBusy } from "../hooks";
import { emptyFilters, includeSelectedIssue, insertAtCursor, issueOptionLabel, matchesIssueSearch, optionalText, splitTags, statusLabel, tagStyle } from "../utils";
import { ActionStatus, ImageUploadControl, LoadingStatus, Markdown, Segment, Sheet } from "../components/common";

export function FilterSheet({ filters, tags, issues, onApplyFilters, onClose }: { filters: IssueFilters; tags: Tag[]; issues: Issue[]; onApplyFilters: (filters: IssueFilters) => void; onClose: () => void }) {
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

export function CreateSheet({ username, issues, onClose, onCreated }: { username: string; issues: Issue[]; onClose: () => void; onCreated: (issue: Issue) => void }) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [descriptionTab, setDescriptionTab] = useState<"source" | "preview">("source");
  const [priority, setPriority] = useState<Priority>("P2");
  const [storyPoints, setStoryPoints] = useState("");
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
    <label>Story points</label><select value={storyPoints} onChange={(e) => setStoryPoints(e.target.value)}>
      <option value="">No estimate</option>
      {storyPointChoices.map((points) => <option key={points} value={points}>{points}SP</option>)}
    </select>
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
        const issue = await api("/api/issues", { method: "POST", username, body: { title, description_markdown: description, priority, story_points: storyPoints ? Number(storyPoints) : null, assignee: optionalText(assignee), parent_issue_id: parentID || null, tag_names: splitTags(tags) } });
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
