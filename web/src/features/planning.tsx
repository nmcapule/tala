import { Blocks, Check, ChevronRight, CircleDot } from "lucide-react";
import type { Issue } from "../types";
import { isResolved, relationshipTitles, resolvedIssues, unresolvedIssues } from "../utils";
import { Badge, IssueMeta, Stat } from "../components/common";

export function Hierarchy({ issues, onOpen }: { issues: Issue[]; onOpen: (id: string) => void }) {
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

export function Blockers({ issues, onOpen }: { issues: Issue[]; onOpen: (id: string) => void }) {
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
