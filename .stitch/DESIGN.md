---
name: Tala Editorial Operations
colors:
  surface: '#f7f3ec'
  surface-dim: '#ded7cc'
  surface-bright: '#fffdf8'
  surface-container-lowest: '#ffffff'
  surface-container-low: '#f1eadf'
  surface-container: '#e8dfd2'
  surface-container-high: '#ddd2c2'
  surface-container-highest: '#d1c3b0'
  on-surface: '#191714'
  on-surface-variant: '#5a534a'
  inverse-surface: '#2d2923'
  inverse-on-surface: '#f7f3ec'
  outline: '#8b8173'
  outline-variant: '#c9bdac'
  surface-tint: '#2d2a7a'
  primary: '#191714'
  on-primary: '#ffffff'
  primary-container: '#2d2a7a'
  on-primary-container: '#f0efff'
  inverse-primary: '#c7c4ff'
  secondary: '#006c4e'
  on-secondary: '#ffffff'
  secondary-container: '#b5f4d8'
  on-secondary-container: '#003826'
  tertiary: '#8f3300'
  on-tertiary: '#ffffff'
  tertiary-container: '#ffd7bd'
  on-tertiary-container: '#411300'
  error: '#b42318'
  on-error: '#ffffff'
  error-container: '#ffe0dc'
  on-error-container: '#7a120c'
  background: '#f7f3ec'
  on-background: '#191714'
  surface-variant: '#e8dfd2'
typography:
  headline-xl:
    fontFamily: Newsreader
    fontSize: 32px
    fontWeight: '600'
    lineHeight: 38px
    letterSpacing: 0em
  headline-lg:
    fontFamily: Newsreader
    fontSize: 26px
    fontWeight: '600'
    lineHeight: 32px
    letterSpacing: 0em
  headline-md:
    fontFamily: Newsreader
    fontSize: 21px
    fontWeight: '600'
    lineHeight: 28px
    letterSpacing: 0em
  body-lg:
    fontFamily: Inter
    fontSize: 16px
    fontWeight: '400'
    lineHeight: 24px
    letterSpacing: 0em
  body-md:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: '400'
    lineHeight: 20px
    letterSpacing: 0em
  label-md:
    fontFamily: Inter
    fontSize: 12px
    fontWeight: '650'
    lineHeight: 16px
    letterSpacing: 0.02em
  label-sm:
    fontFamily: Inter
    fontSize: 11px
    fontWeight: '700'
    lineHeight: 14px
    letterSpacing: 0.04em
rounded:
  sm: 0.125rem
  DEFAULT: 0.25rem
  md: 0.375rem
  lg: 0.5rem
  xl: 0.75rem
  full: 9999px
spacing:
  base: 4px
  xs: 6px
  sm: 8px
  md: 12px
  lg: 16px
  xl: 24px
  gutter-mobile: 12px
  margin-mobile: 16px
---

# Tala Editorial Operations

## Brand And Style

Tala is a local-first coordination tool for people and AI agents working from one shared issue model. The interface should feel like a premium operations notebook: editorial, deliberate, information-rich, and calm under pressure. It should avoid generic SaaS decoration while remaining practical for repeated triage, planning, and feedback workflows.

The design language balances expressive page titles with dense operational controls. Issue data must be easy to scan at mobile sizes: priority, status, assignee, tags, blockers, child counts, and comments should be visible without turning cards into large marketing panels.

## Colors

The base uses warm paper surfaces to create a focused field-notebook feeling. Charcoal is the primary action and text color. Deep indigo is the planning/navigation accent used for selected states, hierarchy focus, and dependency context. Green is reserved for completed, resolved, or unblocked states. Vermilion and amber are reserved for blockers, high priority, destructive warnings, and validation errors.

Do not use colors decoratively. Every strong color should communicate state, priority, navigation focus, or issue health.

## Typography

Use Newsreader for screen titles, major section headings, and empty-state headlines. Use Inter for all controls, metadata, issue cards, editor surfaces, comments, and relationship lists.

All data-heavy UI should be compact and left aligned. Labels may use slight uppercase styling and modest tracking. Body text and Markdown previews should remain comfortable to read.

## Layout And Spacing

The app is mobile-first. Use a narrow single-column structure with persistent bottom navigation for Board, Hierarchy, Blockers, and Profile. Use top bars for current context and actions. Use drawers or sheets for filters, create issue, and relationship pickers.

Structure is created through tonal surfaces and typographic hierarchy, not heavy outlines. Cards can be compact, but should keep enough spacing for tap targets and text wrapping.

## Elevation And Depth

Favor tonal layering over large shadows. Floating sheets and menus can use a soft ambient shadow and a slightly brighter surface. Issue cards should feel like ledger entries, not decorative tiles.

## Shapes

Use subtle radii. Buttons, fields, cards, and chips should feel precise and editorial. Reserve full pills for tags, compact metadata chips, and segmented controls.

## Components

### Navigation

Bottom navigation has four destinations: Board, Hierarchy, Blockers, and Profile. Active state uses the deep indigo accent and a clear label.

### Issue Cards

Issue cards must show title, priority, assignee, tags, blocked indicator, child count, and comment count. They should support compact metadata rows and a clear status affordance.

### Filters

Filters appear in a drawer with text query, assignee, priority, and tag controls. The drawer has clear apply and reset actions.

### Markdown Editing

Markdown descriptions and comments use source-first editing with Source and Preview tabs. Preview should be visually distinct from the editor and show sanitized rendered Markdown.

### Relationship Controls

Parent and blocker pickers use searchable issue lists. Blockers should clearly separate unresolved blockers from completed or canceled blockers. Cycle and validation errors should appear inline near the attempted relationship change.

### Planning Views

Hierarchy and blocker planning screens are alternate projections of the same issue model. They should not introduce new data concepts. Use compact nodes, relationship lines, and status/priority markers that remain legible on mobile.
