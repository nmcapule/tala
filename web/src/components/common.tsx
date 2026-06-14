import React, { useRef, useState } from "react";
import { AlertCircle, Check, Clock, GitBranch, ImagePlus, LoaderCircle, MessageSquare, X } from "lucide-react";
import ReactMarkdown from "react-markdown";
import rehypeSanitize from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import type { ActionNotice, Comment, Issue, Tag } from "../types";
import { uploadImage } from "../api";
import { isResolved, statusLabel, tagStyle } from "../utils";
import { useDelayedBusy } from "../hooks";

export function RequestError({ message, onRetry, onDismiss, compact = false }: { message: string; onRetry?: () => void; onDismiss?: () => void; compact?: boolean }) {
  return <div className={`${compact ? "field-error" : "error-banner"} request-error`}>
    <AlertCircle size={compact ? 15 : 16} />
    <span>{message}</span>
    <div className="request-error-actions">
      {onRetry && <button type="button" onClick={onRetry}>Retry</button>}
      {onDismiss && <button type="button" onClick={onDismiss}>Dismiss</button>}
    </div>
  </div>;
}

export function ActionStatus({ notice }: { notice: ActionNotice }) {
  return <div className={`action-status ${notice.tone}`} role="status" aria-live="polite">
    {notice.tone === "success" ? <Check size={15} /> : <Clock size={15} />}
    <span>{notice.message}</span>
  </div>;
}

export function LoadingStatus({ message, compact = false }: { message: string; compact?: boolean }) {
  return <div className={`loading-status ${compact ? "compact" : ""}`} role="status" aria-live="polite">
    <LoaderCircle size={compact ? 15 : 17} className="loading-spinner" />
    <span>{message}</span>
  </div>;
}

export function Sheet({ title, children, onClose }: { title: string; children: React.ReactNode; onClose: () => void }) {
  return <div className="sheet-backdrop"><section className="sheet"><div className="sheet-header"><h2>{title}</h2><button className="icon-button" aria-label="Close" onClick={onClose}><X size={20} /></button></div>{children}</section></div>;
}

export function EmptyState({ title, description, compact = false }: { title: string; description: string; compact?: boolean }) {
  return <section className={`empty-state ${compact ? "compact" : ""}`}>
    <h2>{title}</h2>
    <p>{description}</p>
  </section>;
}

export function CommentView({ comment }: { comment: Comment }) {
  return <article className="comment"><div><strong>{comment.author}</strong><span>{new Date(comment.created_at).toLocaleString()}</span></div><Markdown text={comment.body_markdown} /></article>;
}

export function ImageUploadControl({ username, disabled, onUploaded }: { username: string; disabled?: boolean; onUploaded: (markdown: string) => void }) {
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

export function Markdown({ text }: { text: string }) {
  return <div className="markdown"><ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeSanitize]}>{text}</ReactMarkdown></div>;
}

export function Segment<T extends string>({ value, onChange }: { value: T; onChange: (value: T) => void }) {
  return <div className="segment"><button className={value === "source" ? "active" : ""} onClick={() => onChange("source" as T)}>Source</button><button className={value === "preview" ? "active" : ""} onClick={() => onChange("preview" as T)}>Preview</button></div>;
}

export function NavButton({ active, icon, label, onClick }: { active: boolean; icon: React.ReactElement; label: string; onClick: () => void }) {
  return <button className={active ? "active" : ""} onClick={onClick}>{React.cloneElement(icon, { size: 20 } as any)}<span>{label}</span></button>;
}

export function IssueMeta({ issue, compact = false }: { issue: Issue; compact?: boolean }) {
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

export function TagRow({ tags, limit }: { tags: Tag[]; limit: number }) {
  if (tags.length === 0) return null;
  const visible = tags.slice(0, limit);
  const hidden = tags.length - visible.length;
  return <div className="tag-row">
    {visible.map((tag) => <span className="tag" style={tagStyle(tag)} key={tag.id}>{tag.name}</span>)}
    {hidden > 0 && <span className="tag tag-overflow" aria-label={`${hidden} more tags`}>+{hidden}</span>}
  </div>;
}

export function Badge({ children, tone = "neutral" }: { children: React.ReactNode; tone?: "neutral" | "danger" | "good" }) {
  return <span className={`badge ${tone}`}>{children}</span>;
}

export function Stat({ label, value, tone = "neutral", active = false, onClick }: { label: string; value: number; tone?: "neutral" | "danger" | "good"; active?: boolean; onClick?: () => void }) {
  const content = <><strong>{value}</strong><span>{label}</span></>;
  if (onClick) {
    return <button className={`stat stat-button ${tone} ${active ? "active" : ""}`} onClick={onClick}>{content}</button>;
  }
  return <div className={`stat ${tone}`}>{content}</div>;
}
