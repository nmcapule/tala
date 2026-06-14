import React from "react";
import type { Issue, IssueFilters, Status, Tag, View } from "./types";
import { filterKeys, tagColorTokens } from "./constants";

export function isResolved(issue: Issue) {
  return issue.status === "completed" || issue.status === "canceled";
}

export function includeSelectedIssue(visible: Issue[], selectedID: string, allCandidates: Issue[]) {
  if (!selectedID || visible.some((issue) => issue.id === selectedID)) return visible;
  const selected = allCandidates.find((issue) => issue.id === selectedID);
  return selected ? [selected, ...visible] : visible;
}

export function isDescendantOf(candidateID: string, ancestorID: string, issues: Issue[]) {
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

export function issueOptionLabel(issue: Issue) {
	const assignee = issue.assignee ? `@${issue.assignee}` : "unassigned";
	return `${issue.title} - ${issue.priority} - ${storyPointLabel(issue)} - ${statusLabel(issue.status)} - ${assignee}`;
}

export function matchesIssueSearch(issue: Issue, query: string) {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return true;
  const haystack = [
    issue.title,
    issue.description_markdown,
    issue.id,
    issue.priority,
    issue.story_points == null ? "" : `${issue.story_points}SP`,
    `${issue.story_points_total}SP`,
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

export function insertAtCursor(ref: React.RefObject<HTMLTextAreaElement | null>, value: string, setValue: React.Dispatch<React.SetStateAction<string>>, inserted: string) {
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

export function storedUsername() {
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

export function viewTitle(view: View) {
  return view === "board" ? "Tala" : view === "hierarchy" ? "Hierarchy" : view === "blockers" ? "Blockers" : "Profile";
}

export function emptyFilters() {
  return { q: "", status: "", assignee: "", priority: "", tag: "", id: "", parent_id: "", blocked_by: "", blocker_of: "", state: "", sort: "", order: "" };
}

export function filtersFromLocation(): IssueFilters {
  const params = new URLSearchParams(window.location.search);
  const filters = emptyFilters();
  filterKeys.forEach((key) => {
    filters[key] = params.get(key) || "";
  });
  return filters;
}

export function issueIDFromLocation() {
  const match = window.location.pathname.match(/^\/issues\/([^/]+)\/?$/);
  if (!match) return null;
  try {
    return decodeURIComponent(match[1]);
  } catch {
    return null;
  }
}

export function issuePath(issueID: string) {
  return `/issues/${encodeURIComponent(issueID)}`;
}

export function viewFromLocation(): View {
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

export function viewPath(view: View) {
  return view === "board" ? "/" : `/${view}`;
}

export function pathWithFilters(view: View, filters: IssueFilters) {
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

export function hasActiveFilters(filters: IssueFilters) {
  return Object.values(filters).some(Boolean);
}

export function statusLabel(status: Status) {
	return status.replace("_", " ");
}

export function storyPointLabel(issue: Pick<Issue, "story_points" | "story_points_total">) {
  if (issue.story_points == null && issue.story_points_total === 0) return "No SP";
  if (issue.story_points == null) return `${issue.story_points_total}SP total`;
  if (issue.story_points_total !== issue.story_points) return `${issue.story_points}SP/${issue.story_points_total}SP`;
  return `${issue.story_points}SP`;
}

export function shortID(id: string) {
  return "#" + id.replace(/^issue_/, "").slice(0, 4);
}

export function formatDateTime(value: string) {
  return new Date(value).toLocaleString([], { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

export function splitTags(value: string) {
  return value.split(",").map((tag) => tag.trim()).filter(Boolean);
}

export function optionalText(value: string) {
  const trimmed = value.trim();
  return trimmed || null;
}

export async function writeClipboard(value: string) {
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

export function relationshipTitles(issues: Issue[] | undefined, fallback: string) {
  if (!issues || issues.length === 0) return fallback;
  return issues.map((issue) => issue.title).join(", ");
}

export function unresolvedIssues(issues: Issue[] | undefined) {
  return (issues || []).filter((issue) => !isResolved(issue));
}

export function resolvedIssues(issues: Issue[] | undefined) {
  return (issues || []).filter(isResolved);
}

export function tagStyle(tag: Tag): React.CSSProperties | undefined {
  if (!tag.color) return undefined;
  const background = tagColorTokens[tag.color] || tag.color;
  const foreground = readableTextColor(background);
  return { backgroundColor: background, color: foreground };
}

export function isValidTagColor(color: string) {
  if (!color) return true;
  return Boolean(tagColorTokens[color] || normalizedHexColor(color));
}

export function readableTextColor(color: string) {
  const hex = normalizedHexColor(color);
  if (!hex) return undefined;
  const r = parseInt(hex.slice(0, 2), 16);
  const g = parseInt(hex.slice(2, 4), 16);
  const b = parseInt(hex.slice(4, 6), 16);
  return (r * 299 + g * 587 + b * 114) / 1000 > 150 ? "#191714" : "#fffdf8";
}

export function normalizedHexColor(color: string) {
  const hex = color.trim().match(/^#?([0-9a-f]{3}|[0-9a-f]{6})$/i)?.[1];
  if (!hex) return undefined;
  return hex.length === 3 ? hex.split("").map((part) => part + part).join("") : hex;
}
