#!/usr/bin/env bash
set -euo pipefail

base="${1:-http://127.0.0.1:8081}"
session="tala-browser-smoke-$$"
username="browser_smoke_$$"
parent_title="Browser smoke parent $$"
blocker_title="Browser smoke blocker $$"
child_title="Browser smoke child $$"
renamed_child_title="Browser smoke child renamed $$"
long_wrap_title="Browser-smoke-long-wrap-$$-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
external_parent_title="Browser smoke external parent $$"
detail_tag="detail-smoke-$$"
profile_tag="profile-smoke-$$"
short_hex_tag="short-hex-smoke-$$"
parent_tag="parent-smoke-$$"
blocker_tag="blocker-smoke-$$"
overflow_tag_a="overflow-a-$$"
overflow_tag_b="overflow-b-$$"
overflow_tag_c="overflow-c-$$"

cleanup() {
  agent-browser --session "$session" close >/dev/null 2>&1 || true
  if [[ -n "${TALA_SMOKE_DB:-}" && -f "${TALA_SMOKE_DB:-}" ]] && command -v sqlite3 >/dev/null 2>&1; then
    sqlite3 "$TALA_SMOKE_DB" <<SQL >/dev/null
PRAGMA foreign_keys = ON;
BEGIN;
DELETE FROM issues WHERE created_by = '$username';
DELETE FROM tags
WHERE name IN (
  '$detail_tag',
  '$profile_tag',
  '$short_hex_tag',
  '$parent_tag',
  '$blocker_tag',
  '$overflow_tag_a',
  '$overflow_tag_b',
  '$overflow_tag_c'
);
COMMIT;
SQL
  fi
}
trap cleanup EXIT

json_field() {
  bun -e "let s=''; process.stdin.on('data', d => s += d); process.stdin.on('end', () => console.log(JSON.parse(s)$1 ?? ''))"
}

click_create() {
  agent-browser --session "$session" find role button click --name "Create issue" >/dev/null
  agent-browser --session "$session" wait --text "Create issue" >/dev/null
}

create_issue() {
  local title="$1"
  local description="$2"
  local priority="$3"
  local assignee="$4"
  local parent_label="${5:-}"
  local tags="$6"

  click_create
  agent-browser --session "$session" find placeholder "Issue title" fill "$title" >/dev/null
  agent-browser --session "$session" find placeholder "Describe the issue..." fill "$description" >/dev/null
  agent-browser --session "$session" select "select" "$priority" >/dev/null
  if [[ -n "$assignee" ]]; then
    agent-browser --session "$session" find placeholder "Unassigned" fill "$assignee" >/dev/null
  fi
  if [[ -n "$parent_label" ]]; then
    agent-browser --session "$session" find placeholder "Search parent issues..." fill "$parent_label" >/dev/null
    agent-browser --session "$session" select "select:nth-of-type(2)" "$parent_label" >/dev/null
  fi
  agent-browser --session "$session" find placeholder "mcp, api" fill "$tags" >/dev/null
  agent-browser --session "$session" eval "document.querySelector('.sheet .button.primary')?.click(); true;" >/dev/null
  agent-browser --session "$session" wait --text "Issue detail" >/dev/null
  agent-browser --session "$session" find role button click --name "Back" >/dev/null
  agent-browser --session "$session" wait --text "$title" >/dev/null
}

curl -fsS "$base/healthz" >/dev/null
agent-browser --session "$session" set viewport 390 844 >/dev/null
agent-browser --session "$session" open "$base" >/dev/null
agent-browser --session "$session" wait --text "Welcome to Tala" >/dev/null
agent-browser --session "$session" eval "localStorage.setItem('tala.username', '   '); true;" >/dev/null
agent-browser --session "$session" open "$base" >/dev/null
agent-browser --session "$session" wait --text "Welcome to Tala" >/dev/null
agent-browser --session "$session" eval "(() => {
  const stored = localStorage.getItem('tala.username');
  if (stored !== null) throw new Error('blank stored username should be cleared');
  if (!document.body.innerText.includes('Welcome to Tala')) {
    throw new Error('blank stored username should return to login screen');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find placeholder "e.g. jdoe_ops" fill "$username" >/dev/null
agent-browser --session "$session" find role button click --name "Continue" >/dev/null
agent-browser --session "$session" wait --fn "document.querySelectorAll('.status-section').length >= 4" >/dev/null
agent-browser --session "$session" eval "(() => {
  const headings = Array.from(document.querySelectorAll('.status-section h2')).map((heading) => heading.textContent?.toLowerCase() || '').join('\\n');
  for (const status of ['new', 'in progress', 'completed', 'canceled']) {
    if (!headings.includes(status)) {
      throw new Error('empty board missing status column: ' + status);
    }
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Hierarchy" >/dev/null
agent-browser --session "$session" wait --fn "location.pathname === '/hierarchy' && !!document.querySelector('.planning, .empty-state')" >/dev/null
agent-browser --session "$session" eval "(() => {
  if (location.pathname !== '/hierarchy') {
    throw new Error('hierarchy nav did not push canonical path: ' + location.pathname);
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" open "$base/hierarchy" >/dev/null
agent-browser --session "$session" wait --fn "location.pathname === '/hierarchy' && !!document.querySelector('.planning, .empty-state')" >/dev/null
agent-browser --session "$session" eval "(() => {
  if (location.pathname !== '/hierarchy') {
    throw new Error('hierarchy refresh did not preserve path: ' + location.pathname);
  }
  if (!document.querySelector('.planning, .empty-state')) {
    throw new Error('hierarchy refresh did not preserve view');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Blockers" >/dev/null
agent-browser --session "$session" wait --fn "location.pathname === '/blockers' && !!document.querySelector('.planning')" >/dev/null
agent-browser --session "$session" eval "(() => {
  if (location.pathname !== '/blockers') {
    throw new Error('blockers nav did not push canonical path: ' + location.pathname);
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" open "$base/blockers" >/dev/null
agent-browser --session "$session" wait --fn "location.pathname === '/blockers' && !!document.querySelector('.planning')" >/dev/null
agent-browser --session "$session" eval "(() => {
  if (location.pathname !== '/blockers') {
    throw new Error('blockers refresh did not preserve path: ' + location.pathname);
  }
  if (!document.querySelector('.planning')) {
    throw new Error('blockers refresh did not preserve view');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Profile" >/dev/null
agent-browser --session "$session" wait --text "$username" >/dev/null
agent-browser --session "$session" eval "(() => {
  if (location.pathname !== '/profile') {
    throw new Error('profile nav did not push canonical path: ' + location.pathname);
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" open "$base/profile" >/dev/null
agent-browser --session "$session" wait --text "$username" >/dev/null
agent-browser --session "$session" eval "(() => {
  if (location.pathname !== '/profile') {
    throw new Error('profile refresh did not preserve path: ' + location.pathname);
  }
  if (!document.body.innerText.includes('$username')) {
    throw new Error('profile refresh did not preserve view');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" network route "$base/api/tags" --abort >/dev/null
agent-browser --session "$session" open "$base/profile" >/dev/null
agent-browser --session "$session" wait --text "Retry" >/dev/null
agent-browser --session "$session" eval "(() => {
  const retryButtons = Array.from(document.querySelectorAll('button')).filter((button) => button.textContent?.trim() === 'Retry');
  if (retryButtons.length === 0) {
    throw new Error('request error did not expose a retry action');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" network unroute "$base/api/tags" >/dev/null
agent-browser --session "$session" eval "(() => {
  const tagAdmin = document.querySelector('.tag-admin');
  const retry = Array.from(tagAdmin?.querySelectorAll('button') || []).find((button) => button.textContent?.trim() === 'Retry');
  if (!retry) throw new Error('tag panel retry button not found');
  retry.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait --fn "!document.querySelector('.tag-admin')?.innerText.includes('Failed to fetch')" >/dev/null
agent-browser --session "$session" eval "Array.from(document.querySelectorAll('button')).filter((button) => button.textContent?.trim() === 'Dismiss').forEach((button) => button.click()); true;" >/dev/null
agent-browser --session "$session" find role button click --name "Board" >/dev/null
agent-browser --session "$session" wait --fn "document.querySelectorAll('.status-section').length >= 4" >/dev/null
agent-browser --session "$session" eval "(() => {
  if (location.pathname !== '/') {
    throw new Error('board nav did not return to root path: ' + location.pathname);
  }
  return true;
})()" >/dev/null
click_create
agent-browser --session "$session" eval "document.querySelector('.sheet .button.primary')?.click()" >/dev/null
agent-browser --session "$session" wait --text "Title is required." >/dev/null
agent-browser --session "$session" find role button click --name "Close" >/dev/null

click_create
agent-browser --session "$session" find placeholder "Issue title" fill "Create preview smoke $$" >/dev/null
agent-browser --session "$session" find placeholder "Describe the issue..." fill "Create preview **markdown**" >/dev/null
agent-browser --session "$session" find role button click --name "Preview" >/dev/null
agent-browser --session "$session" eval "(() => {
  const preview = document.querySelector('.sheet .comment-preview');
  if (!preview?.querySelector('strong')?.textContent?.includes('markdown')) {
    throw new Error('create description Markdown preview did not render bold text');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Close" >/dev/null

curl -fsS -X POST "$base/api/issues" \
  -H 'Content-Type: application/json' \
  -H "X-Tala-Username: $username" \
  -d "{\"title\":\"$external_parent_title\",\"description_markdown\":\"Created outside the UI.\",\"priority\":\"P3\"}" >/dev/null
click_create
agent-browser --session "$session" find placeholder "Search parent issues..." fill "$external_parent_title" >/dev/null
agent-browser --session "$session" eval "(() => {
  const parent = Array.from(document.querySelectorAll('select')).find((select) =>
    Array.from(select.options).some((option) => option.textContent?.includes('$external_parent_title'))
  );
  if (!parent) throw new Error('create parent picker did not refresh external issue context');
  return true;
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Close" >/dev/null

frontend_tag_status="$(
  curl -sS -o /tmp/tala-browser-smoke-frontend-tag.json -w '%{http_code}' -X POST "$base/api/tags" \
    -H 'Content-Type: application/json' \
    -H "X-Tala-Username: $username" \
    -d '{"name":"frontend","color":"#b5f4d8"}'
)"
if [[ "$frontend_tag_status" != "201" && "$frontend_tag_status" != "409" ]]; then
  cat /tmp/tala-browser-smoke-frontend-tag.json >&2
  exit 1
fi

create_issue "$parent_title" "Parent **markdown**" "P2" "alex" "" "$parent_tag"
create_issue "$blocker_title" "Blocks child" "P1" "" "" "$blocker_tag"
create_issue "$child_title" "Child body <script>window.talaUnsafeMarkdown = true</script> [unsafe](javascript:alert(1))" "P0" "sam" "$parent_title" "frontend, $overflow_tag_a, $overflow_tag_b, $overflow_tag_c"

child_id="$(
  curl -fsS "$base/api/issues?q=Browser%20smoke%20child%20$$" | json_field '[0].id'
)"
test -n "$child_id"
blocker_id="$(
  curl -fsS "$base/api/issues?q=Browser%20smoke%20blocker%20$$" | json_field '[0].id'
)"
test -n "$blocker_id"
parent_id="$(
  curl -fsS "$base/api/issues?q=Browser%20smoke%20parent%20$$" | json_field '[0].id'
)"
test -n "$parent_id"
curl -fsS -X POST "$base/api/issues" \
  -H 'Content-Type: application/json' \
  -H "X-Tala-Username: $username" \
  -d "{\"title\":\"$long_wrap_title\",\"description_markdown\":\"Long wrapping child for mobile detail polish.\",\"priority\":\"P2\",\"parent_issue_id\":\"$child_id\",\"tag_names\":[\"frontend\"]}" >/dev/null

agent-browser --session "$session" wait --fn "document.querySelectorAll('.status-section').length >= 4" >/dev/null
agent-browser --session "$session" eval "(() => {
  const card = Array.from(document.querySelectorAll('.issue-card')).find((item) => item.textContent?.includes('$child_title'));
  const sections = Array.from(document.querySelectorAll('.status-section'));
  const target = sections.find((item) => item.querySelector('h2')?.textContent?.includes('In progress')) || sections[1];
  if (!card || !target) throw new Error('drag target not found');
  const dataTransfer = new DataTransfer();
  card.dispatchEvent(new DragEvent('dragstart', { bubbles: true, cancelable: true, dataTransfer }));
  target.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer }));
  target.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer }));
  return true;
})()" >/dev/null
agent-browser --session "$session" wait 700 >/dev/null
curl -fsS "$base/api/issues/$child_id" >/tmp/tala-browser-smoke-drag.json
bun -e 'let d=require("/tmp/tala-browser-smoke-drag.json"); if(d.status !== "in_progress") { console.error("drag did not move issue: " + d.status); process.exit(1) }'

agent-browser --session "$session" eval "(() => {
  const frontendTag = Array.from(document.querySelectorAll('.tag')).find((tag) => tag.textContent?.trim() === 'frontend');
  if (!frontendTag) throw new Error('frontend tag not visible');
  const background = getComputedStyle(frontendTag).backgroundColor;
  if (background !== 'rgb(181, 244, 216)') {
    throw new Error('frontend tag color did not render: ' + background);
  }
  const childCard = Array.from(document.querySelectorAll('.issue-card')).find((item) => item.textContent?.includes('$child_title'));
  if (!childCard?.textContent?.includes('+1')) {
    throw new Error('issue card did not show compact tag overflow count');
  }
  if (!childCard.textContent?.includes('$username')) {
    throw new Error('issue card did not show creator metadata');
  }
  const badges = Array.from(childCard.querySelectorAll('.meta-row .badge')).map((item) => item.textContent?.trim());
  if (!badges.includes('in progress')) {
    throw new Error('issue card did not show visible status badge: ' + badges.join(', '));
  }
  return true;
})()" >/dev/null

agent-browser --session "$session" eval "(() => {
  const card = Array.from(document.querySelectorAll('.issue-card')).find((item) => item.textContent?.includes('$child_title'));
  const button = card?.querySelector('.card-main');
  if (!(button instanceof HTMLElement)) throw new Error('child issue card button not found');
  button.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait --text "Issue detail" >/dev/null
agent-browser --session "$session" eval "(() => {
  if (location.pathname !== '/issues/$child_id') {
    throw new Error('opening an issue did not push canonical permalink: ' + location.pathname);
  }
  const copyButton = document.querySelector('button[aria-label=\"Copy permalink\"]');
  if (!copyButton) throw new Error('detail view missing copy permalink button');
  return true;
})()" >/dev/null
agent-browser --session "$session" open "$base/issues/$child_id" >/dev/null
agent-browser --session "$session" wait --text "Issue detail" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" eval "(() => {
  if (location.pathname !== '/issues/$child_id') {
    throw new Error('direct issue permalink did not preserve pathname: ' + location.pathname);
  }
  if (!document.body.innerText.includes('$child_title')) {
    throw new Error('direct issue permalink did not render the issue detail');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Back" >/dev/null
agent-browser --session "$session" wait --fn "document.querySelectorAll('.status-section').length >= 4" >/dev/null
agent-browser --session "$session" eval "(() => {
  if (location.pathname !== '/') {
    throw new Error('detail Back button did not return to root URL: ' + location.pathname);
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" eval "history.back()" >/dev/null
agent-browser --session "$session" wait --text "Issue detail" >/dev/null
agent-browser --session "$session" eval "(() => {
  if (location.pathname !== '/issues/$child_id') {
    throw new Error('browser back did not restore issue permalink: ' + location.pathname);
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" eval "history.forward()" >/dev/null
agent-browser --session "$session" wait --fn "document.querySelectorAll('.status-section').length >= 4" >/dev/null
agent-browser --session "$session" eval "(() => {
  if (location.pathname !== '/') {
    throw new Error('browser forward did not return to board URL: ' + location.pathname);
  }
  const card = Array.from(document.querySelectorAll('.issue-card')).find((item) => item.textContent?.includes('$child_title'));
  const button = card?.querySelector('.card-main');
  if (!(button instanceof HTMLElement)) throw new Error('child issue card button not found after permalink history checks');
  button.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait --text "Issue detail" >/dev/null
agent-browser --session "$session" wait --text "$long_wrap_title" >/dev/null
agent-browser --session "$session" eval "(() => {
  const navText = document.querySelector('.bottom-nav')?.textContent || '';
  for (const label of ['Board', 'Hierarchy', 'Blockers', 'Profile']) {
    if (!navText.includes(label)) throw new Error('detail view missing persistent bottom nav: ' + label);
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" eval "(() => {
  const root = document.documentElement;
  if (root.scrollWidth > root.clientWidth + 1) {
    throw new Error('mobile detail view overflows horizontally: ' + root.scrollWidth + ' > ' + root.clientWidth);
  }
  const childSection = Array.from(document.querySelectorAll('.relationship-list')).find((section) => section.textContent?.includes('Children'));
  const longLink = Array.from(childSection?.querySelectorAll('.relationship-link') || []).find((button) => button.textContent?.includes('$long_wrap_title'));
  if (!longLink) throw new Error('long child relationship link not found');
  if (longLink.scrollWidth > longLink.clientWidth + 1 && getComputedStyle(longLink).whiteSpace === 'nowrap') {
    throw new Error('long relationship title is still forced onto one line');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" eval "new Promise((resolve, reject) => {
  const hero = document.querySelector('.detail-hero');
  if (!hero) return reject(new Error('detail hero not found'));
  const titleInput = hero.querySelector('input');
  const saveButton = Array.from(hero.querySelectorAll('button')).find((button) => button.textContent?.trim() === 'Save');
  if (!titleInput || !saveButton) return reject(new Error('title edit controls not found'));
  const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(titleInput), 'value')?.set;
  setter.call(titleInput, '   ');
  titleInput.dispatchEvent(new Event('input', { bubbles: true }));
  saveButton.click();
  window.setTimeout(() => {
    if (!hero.textContent?.includes('Title is required.')) {
      return reject(new Error('blank detail title validation did not render'));
    }
    if (!titleInput.classList.contains('invalid')) {
      return reject(new Error('blank detail title did not mark the input invalid'));
    }
    resolve(true);
  }, 100);
})" >/dev/null
curl -fsS "$base/api/issues/$child_id" >/tmp/tala-browser-smoke-title-blank.json
bun -e 'let d=require("/tmp/tala-browser-smoke-title-blank.json"); if(d.title !== process.argv[1]) { console.error("blank title save mutated issue: " + d.title); process.exit(1) }' "$child_title"
agent-browser --session "$session" eval "(() => {
  const hero = document.querySelector('.detail-hero');
  if (!hero) throw new Error('detail hero not found');
  const titleInput = hero.querySelector('input');
  const saveButton = Array.from(hero.querySelectorAll('button')).find((button) => button.textContent?.trim() === 'Save');
  if (!titleInput || !saveButton) throw new Error('title edit controls not found');
  const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(titleInput), 'value')?.set;
  setter.call(titleInput, '$renamed_child_title');
  titleInput.dispatchEvent(new Event('input', { bubbles: true }));
  saveButton.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait --text "$renamed_child_title" >/dev/null
curl -fsS "$base/api/issues/$child_id" >/tmp/tala-browser-smoke-title.json
bun -e 'let d=require("/tmp/tala-browser-smoke-title.json"); if(d.title !== process.argv[1]) { console.error("title did not save: " + d.title); process.exit(1) }' "$renamed_child_title"
agent-browser --session "$session" eval "(() => {
  if (!document.body.innerText.includes('Title saved.')) {
    throw new Error('detail edit did not show success feedback after saving title');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" eval "(() => {
  const hero = document.querySelector('.detail-hero');
  const titleInput = hero?.querySelector('input');
  const saveButton = Array.from(hero?.querySelectorAll('button') || []).find((button) => button.textContent?.trim() === 'Save');
  if (!titleInput || !saveButton) throw new Error('title restore controls not found');
  const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(titleInput), 'value')?.set;
  setter.call(titleInput, '$child_title');
  titleInput.dispatchEvent(new Event('input', { bubbles: true }));
  saveButton.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
curl -fsS "$base/api/issues/$child_id" >/tmp/tala-browser-smoke-title-restore.json
bun -e 'let d=require("/tmp/tala-browser-smoke-title-restore.json"); if(d.title !== process.argv[1]) { console.error("title restore did not save: " + d.title); process.exit(1) }' "$child_title"
agent-browser --session "$session" find role button click --name "Hierarchy" >/dev/null
agent-browser --session "$session" wait --text "$parent_title" >/dev/null
agent-browser --session "$session" find role button click --name "Board" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" eval "(() => {
  const card = Array.from(document.querySelectorAll('.issue-card')).find((item) => item.textContent?.includes('$child_title'));
  const button = card?.querySelector('.card-main');
  if (!(button instanceof HTMLElement)) throw new Error('child issue card button not found after nav return');
  button.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait --text "Issue detail" >/dev/null
saved_description="Detail saved **markdown** <script>window.talaUnsafeMarkdown = true</script> [unsafe](javascript:alert(1))"
agent-browser --session "$session" eval "(() => {
  const panels = Array.from(document.querySelectorAll('.panel'));
  const descriptionPanel = panels.find((panel) => panel.textContent?.includes('Description'));
  const textarea = descriptionPanel?.querySelector('textarea');
  const saveButton = Array.from(descriptionPanel?.querySelectorAll('button') || []).find((button) => button.textContent?.trim() === 'Save description');
  if (!textarea || !saveButton) throw new Error('description edit controls not found');
  const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(textarea), 'value')?.set;
  setter.call(textarea, '$saved_description');
  textarea.dispatchEvent(new Event('input', { bubbles: true }));
  saveButton.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait 500 >/dev/null
curl -fsS "$base/api/issues/$child_id" >/tmp/tala-browser-smoke-description.json
bun -e 'let d=require("/tmp/tala-browser-smoke-description.json"); if(d.description_markdown !== process.argv[1]) { console.error("description did not save: " + d.description_markdown); process.exit(1) }' "$saved_description"
agent-browser --session "$session" eval "(() => {
  const panels = Array.from(document.querySelectorAll('.panel'));
  const descriptionPanel = panels.find((panel) => panel.textContent?.includes('Description'));
  const preview = Array.from(descriptionPanel?.querySelectorAll('button') || []).find((button) => button.textContent?.trim() === 'Preview');
  if (!preview) throw new Error('description preview tab not found');
  preview.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait 250 >/dev/null
agent-browser --session "$session" eval "(() => {
  const panels = Array.from(document.querySelectorAll('.panel'));
  const descriptionPanel = panels.find((panel) => panel.textContent?.includes('Description'));
  if (window.talaUnsafeMarkdown) {
    throw new Error('unsafe Markdown script executed after description save');
  }
  for (const link of descriptionPanel?.querySelectorAll('.markdown a') || []) {
    if (link.getAttribute('href')?.startsWith('javascript:')) {
      throw new Error('unsafe Markdown URL was rendered after description save');
    }
  }
  if (!descriptionPanel?.querySelector('strong')?.textContent?.includes('markdown')) {
    throw new Error('saved description Markdown preview did not render bold text');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find placeholder "Assignee" fill "browser-owner" >/dev/null
agent-browser --session "$session" find role button click --name "Save assignee" >/dev/null
agent-browser --session "$session" wait 500 >/dev/null
curl -fsS "$base/api/issues/$child_id" >/tmp/tala-browser-smoke-assignee.json
bun -e 'let d=require("/tmp/tala-browser-smoke-assignee.json"); if(d.assignee !== "browser-owner") { console.error("assignee did not save: " + d.assignee); process.exit(1) }'
agent-browser --session "$session" find placeholder "Assignee" fill "   " >/dev/null
agent-browser --session "$session" find role button click --name "Save assignee" >/dev/null
agent-browser --session "$session" wait 500 >/dev/null
curl -fsS "$base/api/issues/$child_id" >/tmp/tala-browser-smoke-assignee-clear.json
bun -e 'let d=require("/tmp/tala-browser-smoke-assignee-clear.json"); if(d.assignee !== null) { console.error("assignee did not clear: " + d.assignee); process.exit(1) }'
agent-browser --session "$session" find placeholder "mcp, api" fill "frontend, $detail_tag" >/dev/null
agent-browser --session "$session" eval "(() => {
  const hero = document.querySelector('.detail-hero');
  const saves = Array.from(hero?.querySelectorAll('button') || []).filter((button) => button.textContent?.trim() === 'Save');
  const save = saves.at(-1);
  if (!save) throw new Error('tag save button not found');
  save.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait 700 >/dev/null
curl -fsS "$base/api/issues/$child_id" >/tmp/tala-browser-smoke-detail-tag.json
bun -e 'let d=require("/tmp/tala-browser-smoke-detail-tag.json"); if(!d.tags.some(t => t.name === process.argv[1])) { console.error("detail tag did not save"); process.exit(1) }' "$detail_tag"
agent-browser --session "$session" eval "(() => {
  if (window.talaUnsafeMarkdown) {
    throw new Error('unsafe Markdown script executed');
  }
  for (const link of document.querySelectorAll('.markdown a')) {
    if (link.getAttribute('href')?.startsWith('javascript:')) {
      throw new Error('unsafe Markdown URL was rendered');
    }
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find placeholder "Search parent issues..." fill "$parent_title" >/dev/null
agent-browser --session "$session" eval "(() => {
  const parent = Array.from(document.querySelectorAll('select')).find((select) =>
    Array.from(select.options).some((option) => option.textContent?.includes('$parent_title'))
  );
  if (!parent) throw new Error('searchable parent select not found');
  const labels = Array.from(parent.options).map((option) => option.textContent || '').join('\\n');
  if (!labels.includes('$parent_title') || labels.includes('$blocker_title')) {
    throw new Error('parent search did not narrow parent candidates');
  }
  const feedback = Array.from(document.querySelectorAll('.picker-feedback')).map((item) => item.textContent || '').join('\\n');
  if (!feedback.includes('Showing') || !feedback.includes('candidates')) {
    throw new Error('parent picker did not show candidate count feedback: ' + feedback);
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find placeholder "Search parent issues..." fill "$blocker_title" >/dev/null
agent-browser --session "$session" eval "(() => {
  const parent = Array.from(document.querySelectorAll('select')).find((select) =>
    Array.from(select.options).some((option) => option.textContent === 'No parent')
  );
  if (!parent) throw new Error('parent select not found after changing search');
  const selected = Array.from(parent.options).find((option) => option.value === '$parent_id');
  if (!selected) throw new Error('selected parent should remain visible when search text changes');
  const feedback = Array.from(document.querySelectorAll('.picker-feedback')).map((item) => item.textContent || '').join('\\n');
  if (!feedback.includes('Selected parent stays available while filtering.')) {
    throw new Error('parent picker did not explain preserved selected parent: ' + feedback);
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find placeholder "Search blocker issues..." fill "Blocks child" >/dev/null
agent-browser --session "$session" eval "(() => {
  const blocker = Array.from(document.querySelectorAll('select')).find((select) =>
    Array.from(select.options).some((option) => option.textContent === 'Select blocker issue')
  );
  if (!blocker) throw new Error('blocker select not found for description search');
  const option = Array.from(blocker.options).find((item) => item.value === '$blocker_id');
  if (!option) throw new Error('blocker picker should search issue descriptions');
  return true;
})()" >/dev/null
agent-browser --session "$session" find placeholder "Search blocker issues..." fill "$blocker_title" >/dev/null
agent-browser --session "$session" eval "(() => {
  const blocker = Array.from(document.querySelectorAll('select')).find((select) =>
    Array.from(select.options).some((option) => option.textContent === 'Select blocker issue')
  );
  if (!blocker) throw new Error('blocker select not found');
  const option = Array.from(blocker.options).find((item) => item.value === '$blocker_id');
  if (!option) throw new Error('blocker option not found');
  const groups = Array.from(blocker.querySelectorAll('optgroup')).map((group) => group.label).join('\\n');
  if (!groups.includes('Unresolved blockers (') || !groups.includes('Completed or canceled (')) {
    throw new Error('blocker picker did not show grouped candidate counts: ' + groups);
  }
  blocker.value = option.value;
  blocker.dispatchEvent(new Event('input', { bubbles: true }));
  blocker.dispatchEvent(new Event('change', { bubbles: true }));
  return blocker.value;
})()" >/dev/null
agent-browser --session "$session" find placeholder "Search blocker issues..." fill "$parent_title" >/dev/null
agent-browser --session "$session" eval "(() => {
  const blocker = Array.from(document.querySelectorAll('select')).find((select) =>
    Array.from(select.options).some((option) => option.textContent === 'Select blocker issue')
  );
  if (!blocker) throw new Error('blocker select not found after changing search');
  const selected = Array.from(blocker.options).find((option) => option.value === '$blocker_id');
  if (!selected) throw new Error('selected blocker should remain visible when search text changes');
  const addButton = Array.from(document.querySelectorAll('button')).find((button) => button.textContent?.trim() === 'Add blocker');
  if (!addButton) throw new Error('add blocker button not found');
  addButton.click();
  return blocker.value;
})()" >/dev/null
agent-browser --session "$session" wait 500 >/dev/null
agent-browser --session "$session" wait --text "Unresolved blockers" >/dev/null
agent-browser --session "$session" eval "(() => {
  const blocker = Array.from(document.querySelectorAll('select')).find((select) =>
    Array.from(select.options).some((option) => option.textContent === 'Select blocker issue')
  );
  if (!blocker) throw new Error('blocker select not found after add');
  const option = Array.from(blocker.options).find((item) => item.value === '$blocker_id');
  if (option) throw new Error('existing blocker should not remain available for duplicate add');
  return true;
})()" >/dev/null
agent-browser --session "$session" eval "(() => {
  const blockerSection = Array.from(document.querySelectorAll('.relationship-list')).find((section) => section.textContent?.includes('Blocked by'));
  const link = Array.from(blockerSection?.querySelectorAll('.relationship-link') || []).find((button) => button.textContent?.includes('$blocker_title'));
  if (!link) throw new Error('blocker relationship link not found');
  link.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait --text "$blocker_title" >/dev/null
agent-browser --session "$session" find placeholder "Search blocker issues..." fill "$child_title" >/dev/null
agent-browser --session "$session" eval "(() => {
  const blocker = Array.from(document.querySelectorAll('select')).find((select) =>
    Array.from(select.options).some((option) => option.textContent === 'Select blocker issue')
  );
  if (!blocker) throw new Error('blocker select not found on blocker detail');
  const reverse = Array.from(blocker.options).find((option) => option.value === '$child_id');
  if (reverse) throw new Error('dependent issue should not be offered as reverse blocker candidate');
  return true;
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Back" >/dev/null
agent-browser --session "$session" open "$base/issues/$child_id" >/dev/null
agent-browser --session "$session" wait --text "Issue detail" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" eval "new Promise((resolve, reject) => {
  const commentPanel = Array.from(document.querySelectorAll('.panel')).find((panel) => panel.textContent?.includes('Add comment'));
  if (!commentPanel) return reject(new Error('comment composer not found'));
  const addButton = Array.from(commentPanel.querySelectorAll('button')).find((button) => button.textContent?.trim() === 'Add comment');
  if (!addButton) return reject(new Error('comment add button not found'));
  addButton.click();
  window.setTimeout(() => {
    if (!commentPanel.textContent?.includes('Comment body is required.')) {
      return reject(new Error('empty comment validation did not render'));
    }
    resolve(true);
  }, 100);
})" >/dev/null
agent-browser --session "$session" find placeholder "Add a Markdown comment..." fill "Browser smoke comment **markdown**" >/dev/null
agent-browser --session "$session" eval "(() => {
  const commentPanel = Array.from(document.querySelectorAll('.panel')).find((panel) => panel.textContent?.includes('Add comment'));
  if (!commentPanel) throw new Error('comment composer not found');
  const preview = Array.from(commentPanel.querySelectorAll('button')).find((button) => button.textContent?.trim() === 'Preview');
  if (!preview) throw new Error('comment preview tab not found');
  preview.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait 250 >/dev/null
agent-browser --session "$session" eval "(() => {
  const preview = document.querySelector('.comment-preview');
  if (!preview?.querySelector('strong')?.textContent?.includes('markdown')) {
    throw new Error('comment Markdown preview did not render bold text');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Add comment" >/dev/null
agent-browser --session "$session" wait --load networkidle >/dev/null

curl -fsS "$base/api/issues/$child_id" >/tmp/tala-browser-smoke-detail.json
bun -e 'let d=require("/tmp/tala-browser-smoke-detail.json"); if(!d.blocked || d.blockers.length !== 1 || d.comment_count !== 1 || !d.parent_issue_id) { console.error(JSON.stringify({blocked:d.blocked, blockers:d.blockers?.length, comment_count:d.comment_count, parent_issue_id:d.parent_issue_id}, null, 2)); process.exit(1) }'

agent-browser --session "$session" eval "(() => {
  const parentSection = Array.from(document.querySelectorAll('.relationship-list')).find((section) => section.textContent?.includes('Parent'));
  const link = Array.from(parentSection?.querySelectorAll('.relationship-link') || []).find((button) => button.textContent?.includes('$parent_title'));
  if (!link) throw new Error('current parent relationship link not found');
  link.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait --text "$parent_title" >/dev/null
agent-browser --session "$session" find placeholder "Search blocker issues..." fill "Browser smoke comment" >/dev/null
agent-browser --session "$session" eval "(() => {
  const blocker = Array.from(document.querySelectorAll('select')).find((select) =>
    Array.from(select.options).some((option) => option.textContent === 'Select blocker issue')
  );
  if (!blocker) throw new Error('blocker select not found for comment search');
  const option = Array.from(blocker.options).find((item) => item.value === '$child_id');
  if (!option) throw new Error('relationship picker should search hydrated issue comments');
  return true;
})()" >/dev/null
agent-browser --session "$session" find placeholder "Search parent issues..." fill "$child_title" >/dev/null
agent-browser --session "$session" eval "(() => {
  const parent = Array.from(document.querySelectorAll('select')).find((select) =>
    Array.from(select.options).some((option) => option.textContent === 'No parent')
  );
  if (!parent) throw new Error('parent selector not found on parent issue detail');
  const descendant = Array.from(parent.options).find((option) => option.value === '$child_id');
  if (descendant) throw new Error('descendant issue should not be offered as a parent candidate');
  return true;
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Back" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" open "$base/blockers" >/dev/null
agent-browser --session "$session" wait --fn "location.pathname === '/blockers' && !!document.querySelector('.planning')" >/dev/null
agent-browser --session "$session" eval "(() => {
  const text = document.body.innerText;
  if (!text.includes('Blocked by $blocker_title') || !text.includes('Blocking $child_title')) {
    throw new Error('blocker planning view did not show both dependency directions');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" open "$base/hierarchy" >/dev/null
agent-browser --session "$session" wait --text "$parent_title" >/dev/null
agent-browser --session "$session" eval "(() => {
  const node = Array.from(document.querySelectorAll('.tree-node-button')).find((item) => item.textContent?.includes('$child_title'));
  const text = node?.textContent || '';
  if (!/P[0-4]/.test(text) || !text.includes('in progress') || !text.includes('Blocked')) {
    throw new Error('hierarchy node did not show status, priority, and blocked metadata: ' + text);
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" open "$base" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" wait --text "$detail_tag" >/dev/null
agent-browser --session "$session" eval "new Promise((resolve, reject) => {
  const tag = document.querySelector('.sheet button[aria-label=\"Filter by tag $detail_tag\"]');
  if (!tag) return reject(new Error('detail-created tag filter chip not found'));
  tag.click();
  window.setTimeout(() => {
    const apply = Array.from(document.querySelectorAll('.sheet .button.primary')).find((item) => item.textContent?.trim() === 'Apply');
    if (!apply) return reject(new Error('filter apply button not found'));
    apply.click();
    resolve(true);
  }, 100);
})" >/dev/null
agent-browser --session "$session" wait --fn "document.body.innerText.includes('$child_title')" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" find role button click --name "Reset" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" wait --text "No filters active" >/dev/null
agent-browser --session "$session" eval "(() => {
  const sheetText = document.querySelector('.sheet')?.textContent || '';
  if (!sheetText.includes('Relationships') || !sheetText.includes('Sort')) {
    throw new Error('filter drawer section headings not found');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find placeholder "Markdown, title, or keyword" fill "$child_title" >/dev/null
agent-browser --session "$session" wait --text "1 active filter" >/dev/null
agent-browser --session "$session" find role button click --name "Clear" >/dev/null
agent-browser --session "$session" wait --text "No filters active" >/dev/null
agent-browser --session "$session" eval "(() => {
  const input = document.querySelector('.sheet input[placeholder=\"Markdown, title, or keyword\"]');
  if (!(input instanceof HTMLInputElement) || input.value !== '') {
    throw new Error('filter clear did not empty pending search text');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find placeholder "Markdown, title, or keyword" fill "$child_title" >/dev/null
agent-browser --session "$session" find role button click --name "Apply" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" find role button click --name "Reset" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" find placeholder "Markdown, title, or keyword" fill "Browser smoke comment" >/dev/null
agent-browser --session "$session" find role button click --name "Apply" >/dev/null
agent-browser --session "$session" wait --fn "(() => {
  const text = document.body.innerText;
  return text.includes('$child_title') && !text.includes('$parent_title') && !text.includes('$blocker_title');
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" find role button click --name "Reset" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" eval "new Promise((resolve, reject) => {
  const tag = document.querySelector('.sheet button[aria-label=\"Filter by tag frontend\"]');
  if (!tag) return reject(new Error('frontend filter chip not found'));
  tag.click();
  window.setTimeout(() => {
    const apply = Array.from(document.querySelectorAll('.sheet .button.primary')).find((item) => item.textContent?.trim() === 'Apply');
    if (!apply) return reject(new Error('filter apply button not found'));
    apply.click();
    resolve(true);
  }, 100);
})" >/dev/null
agent-browser --session "$session" wait --fn "(() => {
  const text = document.body.innerText;
  return text.includes('$child_title') && !text.includes('$parent_title') && !text.includes('$blocker_title');
})()" >/dev/null
agent-browser --session "$session" eval "(() => {
  const text = document.body.innerText;
  if (!text.includes('$child_title') || text.includes('$parent_title') || text.includes('$blocker_title')) {
    throw new Error('tag filter did not isolate the frontend issue');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" open "$base/blockers" >/dev/null
agent-browser --session "$session" wait --fn "location.pathname === '/blockers' && !!document.querySelector('.planning')" >/dev/null
agent-browser --session "$session" eval "(() => {
  const text = document.body.innerText;
  if (!text.includes('Blocked by $blocker_title') || !text.includes('Blocking $child_title')) {
    throw new Error('blocker planning view should use full issue context while board filters are active');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" open "$base" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" find role button click --name "Reset" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" eval "(() => {
  const labels = Array.from(document.querySelectorAll('.sheet label'));
  const label = labels.find((item) => item.textContent?.trim() === 'Parent');
  const select = label?.nextElementSibling;
  if (!(select instanceof HTMLSelectElement)) throw new Error('parent filter select not found');
  const option = Array.from(select.options).find((item) => item.textContent?.includes('$parent_title'));
  if (!option) throw new Error('parent filter option not found');
  select.value = option.value;
  select.dispatchEvent(new Event('input', { bubbles: true }));
  select.dispatchEvent(new Event('change', { bubbles: true }));
  const apply = Array.from(document.querySelectorAll('.sheet .button.primary')).find((item) => item.textContent?.trim() === 'Apply');
  if (!apply) throw new Error('filter apply button not found');
  apply.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait --fn "(() => {
  const text = document.body.innerText;
  return text.includes('$child_title') && !text.includes('$parent_title') && !text.includes('$blocker_title');
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" find role button click --name "Reset" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" eval "(() => {
  const labels = Array.from(document.querySelectorAll('.sheet label'));
  const label = labels.find((item) => item.textContent?.trim() === 'Blocked by');
  const select = label?.nextElementSibling;
  if (!(select instanceof HTMLSelectElement)) throw new Error('blocked-by filter select not found');
  const option = Array.from(select.options).find((item) => item.value === '$blocker_id');
  if (!option) throw new Error('blocked-by filter option not found');
  select.value = option.value;
  select.dispatchEvent(new Event('input', { bubbles: true }));
  select.dispatchEvent(new Event('change', { bubbles: true }));
  const apply = Array.from(document.querySelectorAll('.sheet .button.primary')).find((item) => item.textContent?.trim() === 'Apply');
  if (!apply) throw new Error('filter apply button not found');
  apply.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait --fn "(() => {
  const text = document.body.innerText;
  return text.includes('$child_title') && !text.includes('$parent_title') && !text.includes('$blocker_title');
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" find role button click --name "Reset" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" eval "(() => {
  const labels = Array.from(document.querySelectorAll('.sheet label'));
  const label = labels.find((item) => item.textContent?.trim() === 'Status');
  const select = label?.nextElementSibling;
  if (!(select instanceof HTMLSelectElement)) throw new Error('status filter select not found');
  select.value = 'completed';
  select.dispatchEvent(new Event('input', { bubbles: true }));
  select.dispatchEvent(new Event('change', { bubbles: true }));
  const apply = Array.from(document.querySelectorAll('.sheet .button.primary')).find((item) => item.textContent?.trim() === 'Apply');
  if (!apply) throw new Error('filter apply button not found');
  apply.click();
  return true;
})()" >/dev/null
agent-browser --session "$session" wait 500 >/dev/null
agent-browser --session "$session" eval "(() => {
  if (document.body.innerText.includes('$child_title')) {
    throw new Error('status-only filter did not refresh the board');
  }
  if (document.querySelectorAll('.status-section').length < 4) {
    throw new Error('status-only filter should keep the board rendered');
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find role button click --name "Filters" >/dev/null
agent-browser --session "$session" wait --text "Filters" >/dev/null
agent-browser --session "$session" find role button click --name "Reset" >/dev/null
agent-browser --session "$session" wait --text "$child_title" >/dev/null
curl -fsS -X PATCH "$base/api/issues/$blocker_id" \
  -H 'Content-Type: application/json' \
  -H "X-Tala-Username: $username" \
  -d '{"status":"completed"}' >/dev/null
agent-browser --session "$session" open "$base/blockers" >/dev/null
agent-browser --session "$session" wait --text "Resolved dependencies" >/dev/null
agent-browser --session "$session" eval "(() => {
  const activeText = Array.from(document.querySelectorAll('.dependency')).map((item) => item.textContent || '').join('\\n');
  if (activeText.includes('$child_title') || activeText.includes('$blocker_title')) {
    throw new Error('resolved dependency should not remain in active blocker cards');
  }
  const resolvedText = document.querySelector('.resolved-dependencies')?.textContent || '';
  if (!resolvedText.includes('Resolved blockers: $blocker_title') || !resolvedText.includes('No longer blocking: $child_title')) {
    throw new Error('resolved dependency section did not show both resolved directions: ' + resolvedText);
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" open "$base/profile" >/dev/null
agent-browser --session "$session" wait --text "$username" >/dev/null
agent-browser --session "$session" find role button click --name "Create tag" >/dev/null
agent-browser --session "$session" wait --text "Tag name is required." >/dev/null
agent-browser --session "$session" find placeholder "Tag name" fill "$profile_tag" >/dev/null
agent-browser --session "$session" fill "input[aria-label='Tag color']" "not-a-color" >/dev/null
agent-browser --session "$session" find role button click --name "Create tag" >/dev/null
agent-browser --session "$session" wait --text "Use a color token or hex value like #b5f4d8." >/dev/null
agent-browser --session "$session" find role button click --name "Use tertiary-container" >/dev/null
agent-browser --session "$session" find role button click --name "Create tag" >/dev/null
agent-browser --session "$session" wait --text "$profile_tag" >/dev/null
agent-browser --session "$session" eval "(() => {
  const tag = Array.from(document.querySelectorAll('.tag')).find((item) => item.textContent?.trim() === '$profile_tag');
  if (!tag) throw new Error('profile-created tag not visible');
  const background = getComputedStyle(tag).backgroundColor;
  if (background !== 'rgb(255, 215, 189)') {
    throw new Error('profile-created swatch color did not render: ' + background);
  }
  return true;
})()" >/dev/null
agent-browser --session "$session" find placeholder "Tag name" fill "$short_hex_tag" >/dev/null
agent-browser --session "$session" fill "input[aria-label='Tag color']" "#123" >/dev/null
agent-browser --session "$session" find role button click --name "Create tag" >/dev/null
agent-browser --session "$session" wait --text "$short_hex_tag" >/dev/null
agent-browser --session "$session" eval "(() => {
  const tag = Array.from(document.querySelectorAll('.tag')).find((item) => item.textContent?.trim() === '$short_hex_tag');
  if (!tag) throw new Error('short hex tag not visible');
  const style = getComputedStyle(tag);
  if (style.backgroundColor !== 'rgb(17, 34, 51)' || style.color !== 'rgb(255, 253, 248)') {
    throw new Error('short hex tag colors did not render readably: ' + style.backgroundColor + ' / ' + style.color);
  }
  return true;
})()" >/dev/null

curl -fsSI "$base/" | awk 'NR == 1 { if ($2 != 200) exit 1 }'
echo "browser smoke ok: $base"
