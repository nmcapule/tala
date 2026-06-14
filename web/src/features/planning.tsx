import { ArrowRight, Blocks, Check, ChevronRight, CircleDot, GitBranch } from "lucide-react";
import type React from "react";
import type { Issue } from "../types";
import { isResolved, relationshipTitles, resolvedIssues, storyPointLabel, unresolvedIssues } from "../utils";
import { Badge, IssueMeta, Stat } from "../components/common";

export function Hierarchy({ issues, onOpen }: { issues: Issue[]; onOpen: (id: string) => void }) {
  if (issues.length === 0) {
    return <section className="empty-state"><h2>No hierarchy yet</h2><p>Create parent and child issues to build a planning tree.</p></section>;
  }
  const issueIDs = new Set(issues.map((issue) => issue.id));
  const roots = issues.filter((issue) => !issue.parent_issue_id || !issueIDs.has(issue.parent_issue_id));
  const childLinks = issues.filter((issue) => issue.parent_issue_id && issueIDs.has(issue.parent_issue_id)).length;
  const blockedIssues = issues.filter((issue) => issue.blocked).length;
  const totalStoryPoints = roots.reduce((sum, issue) => sum + issue.story_points_total, 0);

  if (roots.length === 0) {
    return <section className="empty-state"><h2>No hierarchy roots</h2><p>Every visible issue has a parent. Clear a parent relationship to create a root.</p></section>;
  }

  return <div className="planning hierarchy-planning">
    <div className="board-stats">
      <Stat label="Roots" value={roots.length} />
      <Stat label="Child links" value={childLinks} tone="good" />
      <Stat label="Total SP" value={totalStoryPoints} />
      <Stat label="Blocked" value={blockedIssues} tone={blockedIssues > 0 ? "danger" : "neutral"} />
    </div>
    {roots.map((issue) => <TreeNode key={issue.id} issue={issue} all={issues} onOpen={onOpen} depth={0} />)}
  </div>;
}

function TreeNode({ issue, all, onOpen, depth }: { issue: Issue; all: Issue[]; onOpen: (id: string) => void; depth: number }) {
  const children = all.filter((child) => child.parent_issue_id === issue.id);
  const activeBlockers = unresolvedIssues(issue.blockers).length;
  const activeDependents = unresolvedIssues(issue.blocked_by).length;
  const depthLabel = depth === 0 ? "Root" : `Level ${depth + 1}`;
  return <section className={`tree-node ${depth === 0 ? "root" : ""}`} style={{ "--tree-depth": Math.min(depth, 4) } as React.CSSProperties}>
    <button className="tree-node-button" onClick={() => onOpen(issue.id)}>
      <div className="tree-node-icon"><CircleDot size={16} /></div>
      <div className="tree-node-main">
        <div className="tree-node-title-row">
          <span className="tree-depth-label">{depthLabel}</span>
          <span className="tree-node-title">{issue.title}</span>
        </div>
        <IssueMeta issue={issue} />
        <div className="tree-node-summary" aria-label="Hierarchy summary">
          <span><GitBranch size={13} />{children.length} {children.length === 1 ? "child" : "children"}</span>
          <span>{storyPointLabel(issue)}</span>
          <span className={activeBlockers > 0 ? "danger" : ""}><Blocks size={13} />{activeBlockers} blockers</span>
          {activeDependents > 0 && <span>{activeDependents} waiting</span>}
        </div>
      </div>
    </button>
    {children.length > 0 ? <div className="tree-children">
      {children.map((child) => <TreeNode key={child.id} issue={child} all={all} onOpen={onOpen} depth={depth + 1} />)}
    </div> : depth === 0 && <div className="tree-empty-children">No child issues under this root yet.</div>}
  </section>;
}

export function Blockers({ issues, onOpen }: { issues: Issue[]; onOpen: (id: string) => void }) {
  const dependencyIssues = issues;
  const blockedIssues = dependencyIssues.filter((issue) => issue.blocked);
  const blockingIssues = dependencyIssues.filter((issue) => !isResolved(issue) && unresolvedIssues(issue.blocked_by).length > 0);
  const resolvedDependencyIssues = dependencyIssues.filter((issue) => !issue.blocked && resolvedIssues(issue.blockers).length > 0);
  const inactiveBlockingIssues = dependencyIssues.filter((issue) => isResolved(issue) && (issue.blocked_by?.length || 0) > 0);
  const resolvedBlockingIssues = dependencyIssues.filter((issue) => !isResolved(issue) && unresolvedIssues(issue.blocked_by).length === 0 && resolvedIssues(issue.blocked_by).length > 0);
  const issuesWithAnyRelationships = dependencyIssues.filter((issue) => (issue.blockers?.length || 0) > 0 || (issue.blocked_by?.length || 0) > 0).length;

  return <div className="planning">
    <div className="board-stats">
      <Stat label="Active blockers" value={blockedIssues.length} tone="danger" />
      <Stat label="Blocking others" value={blockingIssues.length} />
      <Stat label="Any history" value={issuesWithAnyRelationships} tone="good" />
    </div>
    {blockedIssues.length === 0 && blockingIssues.length === 0 && resolvedDependencyIssues.length === 0 && inactiveBlockingIssues.length === 0 && resolvedBlockingIssues.length === 0 && <section className="empty-state"><h2>No blockers</h2><p>No unresolved dependency relationships yet.</p></section>}
    {blockedIssues.length > 0 && <section className="dependency-section">
      <DependencySectionHeader title="Blocked by active issues" count={blockedIssues.length} description="Incoming blockers prevent these issues from moving forward." />
      {blockedIssues.map((issue) => (
        <DependencyCard issue={issue} key={`blocked-${issue.id}`} tone="danger" badge="Blocked" onOpen={onOpen}>
          <div className="dependency-line danger"><Blocks size={16} /><span>Incoming blockers: {relationshipTitles(unresolvedIssues(issue.blockers), "none")}</span></div>
          {resolvedIssues(issue.blockers).length > 0 && <div className="dependency-line resolved"><Check size={16} /><span>Resolved blockers: {relationshipTitles(resolvedIssues(issue.blockers), "none")}</span></div>}
          {unresolvedIssues(issue.blocked_by).length > 0 && <div className="dependency-line neutral"><ArrowRight size={16} /><span>Also blocking: {relationshipTitles(unresolvedIssues(issue.blocked_by), "no active dependent issues")}</span></div>}
        </DependencyCard>
      ))}
    </section>}
    {blockingIssues.length > 0 && <section className="dependency-section">
      <DependencySectionHeader title="Actively blocking other issues" count={blockingIssues.length} description="Outgoing dependents are waiting on these issues." />
      {blockingIssues.map((issue) => (
        <DependencyCard issue={issue} key={`blocking-${issue.id}`} badge="Blocking" onOpen={onOpen}>
          <div className="dependency-line neutral"><ChevronRight size={16} /><span>Outgoing dependents: {relationshipTitles(unresolvedIssues(issue.blocked_by), "no active dependent issues")}</span></div>
          {unresolvedIssues(issue.blockers).length > 0 && <div className="dependency-line danger"><Blocks size={16} /><span>Still blocked by: {relationshipTitles(unresolvedIssues(issue.blockers), "none")}</span></div>}
          {resolvedIssues(issue.blocked_by).length > 0 && <div className="dependency-line resolved"><Check size={16} /><span>Resolved dependents: {relationshipTitles(resolvedIssues(issue.blocked_by), "none")}</span></div>}
        </DependencyCard>
      ))}
    </section>}
    {(resolvedDependencyIssues.length > 0 || inactiveBlockingIssues.length > 0 || resolvedBlockingIssues.length > 0) && <section className="resolved-dependencies">
      <DependencySectionHeader title="Resolved dependency history" count={resolvedDependencyIssues.length + inactiveBlockingIssues.length + resolvedBlockingIssues.length} description="Completed or canceled links are kept separate from active blockers." />
      {resolvedDependencyIssues.map((issue) => (
        <button className="resolved-row" key={`resolved-blocked-${issue.id}`} onClick={() => onOpen(issue.id)}>
          <span>{issue.title}</span>
          <small>Incoming blockers resolved: {relationshipTitles(resolvedIssues(issue.blockers), "none")}</small>
        </button>
      ))}
      {inactiveBlockingIssues.map((issue) => (
        <button className="resolved-row" key={`inactive-blocking-${issue.id}`} onClick={() => onOpen(issue.id)}>
          <span>{issue.title}</span>
          <small>Resolved issue no longer blocks: {relationshipTitles(issue.blocked_by, "no dependent issues")}</small>
        </button>
      ))}
      {resolvedBlockingIssues.map((issue) => (
        <button className="resolved-row" key={`resolved-blocking-${issue.id}`} onClick={() => onOpen(issue.id)}>
          <span>{issue.title}</span>
          <small>Outgoing dependents resolved: {relationshipTitles(resolvedIssues(issue.blocked_by), "none")}</small>
        </button>
      ))}
    </section>}
  </div>;
}

function DependencySectionHeader({ title, count, description }: { title: string; count: number; description: string }) {
  return <div className="dependency-section-header">
    <div>
      <h2>{title}</h2>
      <p>{description}</p>
    </div>
    <Badge>{count}</Badge>
  </div>;
}

function DependencyCard({ issue, tone = "neutral", badge, onOpen, children }: { issue: Issue; tone?: "neutral" | "danger"; badge: string; onOpen: (id: string) => void; children: React.ReactNode }) {
  const activeBlockers = unresolvedIssues(issue.blockers).length;
  const resolvedBlockers = resolvedIssues(issue.blockers).length;
  const activeDependents = unresolvedIssues(issue.blocked_by).length;
  const resolvedDependents = resolvedIssues(issue.blocked_by).length;
  return <article className="dependency">
    <button className="dependency-heading" onClick={() => onOpen(issue.id)}>
      <div>
        <h3>{issue.title}</h3>
        <IssueMeta issue={issue} />
      </div>
      <Badge tone={tone}>{badge}</Badge>
    </button>
    <div className="dependency-summary" aria-label="Dependency summary">
      <span className={activeBlockers > 0 ? "danger" : ""}>Blocked by {activeBlockers}</span>
      <span>Blocks {activeDependents}</span>
      {resolvedBlockers + resolvedDependents > 0 && <span className="resolved">{resolvedBlockers + resolvedDependents} resolved</span>}
    </div>
    {children}
  </article>;
}
