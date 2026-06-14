import { useEffect, useRef, useState } from "react";
import { ArrowLeft, Blocks, ClipboardList, Filter, GitBranch, Link2, Minimize2, Plus, Settings, User } from "lucide-react";
import type { Issue, IssueFilters, Tag, View } from "./types";
import { filterKeys } from "./constants";
import { useDelayedBusy } from "./hooks";
import { emptyFilters, filtersFromLocation, hasActiveFilters, issueIDFromLocation, issuePath, pathWithFilters, storedUsername, viewFromLocation, viewTitle, writeClipboard } from "./utils";
import { Board } from "./features/board";
import { IssueDetail } from "./features/detail";
import { Blockers, Hierarchy } from "./features/planning";
import { Profile } from "./features/profile";
import { CreateSheet, FilterSheet } from "./features/sheets";
import { Login } from "./features/login";
import { EmptyState, LoadingStatus, NavButton, RequestError } from "./components/common";

export function App() {
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
  const [compactMode, setCompactMode] = useState(() => localStorage.getItem("tala.compactMode") === "true");
  const [tags, setTags] = useState<Tag[]>([]);
  const [error, setError] = useState("");
  const [permalinkCopied, setPermalinkCopied] = useState(false);
  const [boardLoading, setBoardLoading] = useState(0);
  const [contextLoading, setContextLoading] = useState(0);
  const [tagsLoading, setTagsLoading] = useState(0);
  const [selectedLoading, setSelectedLoading] = useState(Boolean(initialIssueID));
  const [sheetLoading, setSheetLoading] = useState<"filters" | "create" | null>(null);
  const lastListView = useRef<View>(initialView);
  const backgroundRefreshInFlight = useRef(false);
  const lastBackgroundRefreshAt = useRef(0);
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
      setAllIssues(data || []);
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

  async function refreshVisibleData() {
    if (!username || document.hidden || backgroundRefreshInFlight.current) return;
    backgroundRefreshInFlight.current = true;
    try {
      await refreshAll();
    } finally {
      lastBackgroundRefreshAt.current = Date.now();
      backgroundRefreshInFlight.current = false;
    }
  }

  useEffect(() => {
    if (!username) return;
    const refreshSoon = () => {
      if (Date.now() - lastBackgroundRefreshAt.current < 2000) return;
      refreshVisibleData().catch((err) => setError(err instanceof Error ? err.message : "Unable to refresh issue context."));
    };
    const intervalID = window.setInterval(refreshSoon, 30000);
    window.addEventListener("focus", refreshSoon);
    document.addEventListener("visibilitychange", refreshSoon);
    return () => {
      window.clearInterval(intervalID);
      window.removeEventListener("focus", refreshSoon);
      document.removeEventListener("visibilitychange", refreshSoon);
    };
  }, [username, selectedID, filters.q, filters.status, filters.assignee, filters.priority, filters.tag, filters.id, filters.parent_id, filters.blocked_by, filters.blocker_of, filters.state, filters.sort, filters.order]);

  async function retryGlobalLoad() {
    setError("");
    try {
      await refreshAll();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to reload data.");
    }
  }

  function toggleCompactMode() {
    setCompactMode((current) => {
      const next = !current;
      localStorage.setItem("tala.compactMode", next ? "true" : "false");
      return next;
    });
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
          {!selected && <button className={`icon-button ${compactMode ? "active" : ""}`} onClick={toggleCompactMode} aria-label="Compact mode" aria-pressed={compactMode} title={compactMode ? "Compact mode on" : "Compact mode"}><Minimize2 size={20} /></button>}
          {!selected && <button className="icon-button" disabled={Boolean(sheetLoading)} onClick={openFilterSheet} aria-label="Filters"><Filter size={20} /></button>}
          {!selected && <button className="icon-button primary" disabled={Boolean(sheetLoading)} onClick={openCreateSheet} aria-label="Create issue"><Plus size={20} /></button>}
        </div>
      </header>

      {error && <RequestError message={error} onRetry={retryGlobalLoad} onDismiss={() => setError("")} />}
      {showSheetLoading && <div className="content loading-content"><LoadingStatus message={sheetLoading === "create" ? "Loading issue context..." : "Loading filter context..."} /></div>}

      <main className="content">
        {selected ? (
          <IssueDetail issue={selected} detailLoading={showSelectedLoading} username={username} issues={allIssues} compactMode={compactMode} onOpenIssue={openIssue} onApplyFilters={applyFilters} onRefresh={refreshAll} onTagsChanged={refreshTags} onClose={closeIssue} />
        ) : selectedID ? (
          selectedLoading ? (showSelectedLoading ? <LoadingStatus message="Loading issue detail..." /> : null) : <EmptyState title="Issue not found" description="The selected issue could not be loaded." />
        ) : view === "board" ? (
          <Board issues={issues} totalIssues={allIssues.length} filters={filters} hasFilters={hasActiveFilters(filters)} loading={boardLoading > 0 || contextLoading > 0} showLoading={showBoardLoading || showContextLoading} username={username} compactMode={compactMode} onOpen={openIssue} onRefresh={refresh} onResetFilters={resetFilters} onApplyFilters={applyFilters} />
        ) : view === "hierarchy" ? (
          contextLoading > 0 ? (showContextLoading ? <LoadingStatus message="Loading hierarchy..." /> : null) : <Hierarchy issues={allIssues} compactMode={compactMode} onOpen={openIssue} />
        ) : view === "blockers" ? (
          contextLoading > 0 ? (showContextLoading ? <LoadingStatus message="Loading blockers..." /> : null) : <Blockers issues={allIssues} compactMode={compactMode} onOpen={openIssue} />
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
