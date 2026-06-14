import { useEffect, useState } from "react";
import { AlertCircle, User } from "lucide-react";
import type { Tag } from "../types";
import { api } from "../api";
import { tagColorChoices, tagColorTokens } from "../constants";
import { useDelayedBusy } from "../hooks";
import { isValidTagColor, tagStyle } from "../utils";
import { EmptyState, LoadingStatus, RequestError } from "../components/common";

export function Profile({ username, onLogout, onTagsChanged }: { username: string; onLogout: () => void; onTagsChanged: () => Promise<void> }) {
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
