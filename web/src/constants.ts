import type { Priority, Status } from "./types";

export const statuses: Status[] = ["new", "in_progress", "completed", "canceled"];
export const priorities: Priority[] = ["P0", "P1", "P2", "P3", "P4"];
export const storyPointChoices = [1, 2, 3, 5, 8, 13, 21] as const;
export const filterKeys = ["q", "status", "assignee", "priority", "tag", "id", "parent_id", "blocked_by", "blocker_of", "state", "sort", "order"] as const;
export const states = ["open", "blocked", "done"];
export const sortOptions = [
  { value: "", label: "Default" },
  { value: "priority", label: "Priority" },
  { value: "updated_at", label: "Last update" },
  { value: "created_at", label: "Created" },
  { value: "title", label: "Title" },
  { value: "status", label: "Status" },
];
export const tagColorTokens: Record<string, string> = {
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
export const tagColorChoices = [
  "secondary-container",
  "tertiary-container",
  "error-container",
  "primary-container",
  "surface-container",
  "surface-container-highest",
  "outline",
  "secondary",
];
