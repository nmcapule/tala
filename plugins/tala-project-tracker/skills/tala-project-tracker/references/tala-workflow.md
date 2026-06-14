# Tala Workflow Reference

## Issue Taxonomy

- `Roadmap:` or `Epic:` parent issues: broad outcomes with child work.
- `Feature:` user-visible capability.
- `Bug:` incorrect behavior or regression. For UI-related bugs, include one or more uploaded visual references in the description.
- `Investigation:` unknown root cause or design discovery.
- `Task:` implementation or operational work.
- `Release:` release readiness or verification work.
- `Tech debt:` maintainability or cleanup.

## Priority Rules

- `P0`: release blocker, data integrity, security exposure, or unusable core workflow.
- `P1`: major feature, correctness issue, MCP/REST contract issue, or important UX blocker.
- `P2`: normal planned work.
- `P3`: polish, docs refinement, lower-risk improvement.
- `P4`: future idea or optional enhancement.

## Breakdown Rules

- Create children for work that can be resumed independently.
- Use nested children when a child has multiple phases, such as design, backend, frontend, tests, docs.
- Keep titles action-oriented and searchable.
- Put detail in Markdown descriptions and comments, not overly long titles.
- Format newly filed issue descriptions like this:

```markdown
**Context**: <detailed description on why this bug exists or is needed>

**References** <optional>

**Visual references** <required for UI-related bugs>

- ![short description](/uploads/images/<filename>)
- Viewport/device/route/state: <context>

**Action Items**

- [ ] <sample action item 1>
    - [ ] <sample action subitem>
- [ ] <...and so on...>
```

- Omit the `References` section when there are no relevant docs, issues, files, logs, or external links.
- Omit the `Visual references` section only for non-UI issues. If a UI bug cannot include an uploaded image, keep the section and explain why capture/upload was unavailable.
- Link blockers when one issue cannot proceed until another is resolved.
- Avoid blocker links for simple ordering preferences.

## Duplicate Avoidance

Search by title keywords, tag, and likely domain before creating an issue.

When a near duplicate exists:

- Reuse it if the scope matches.
- Add a comment if the new request adds context.
- Create a child issue if the request is a distinct piece of the existing parent.

## Comment Templates

### Planning

```markdown
Planning update

Objective:
- ...

Checklist:
- [ ] ...
- [ ] ...

Assumptions:
- ...

Risks/blockers:
- ...

Verification:
- ...
```

### Progress

```markdown
Progress update

Done:
- ...

Observed:
- ...

Visual references: <required when this update reports UI state, browser QA, a visual defect, or a design mismatch>
- ![short description](/uploads/images/<filename>)
- Viewport/device/route/state: ...

Next:
- ...
```

### Interruption/Handoff

```markdown
Handoff update

Current state:
- ...

Completed:
- ...

Visual references: <required when current state or completed work is UI-related>
- ![short description](/uploads/images/<filename>)
- Viewport/device/route/state: ...

Remaining:
- ...

Verification so far:
- ...

Resume from:
- ...
```

### Completion

```markdown
Completion update

Completed:
- ...

Verification:
- ...

Visual references: <required when completion verifies UI behavior or visual state>
- ![short description](/uploads/images/<filename>)
- Viewport/device/route/state: ...

Follow-ups:
- ...
```
