# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

> Note: the product is a terminal application, outside the schema's native/mobile categories; `web` is recorded only as the closest schema value. The actual shipping surface is a cross-platform terminal UI backed by a local or remote HTTP API.

## Users

Developers and AI-agent operators who already live in the terminal and need a shared, lightweight kanban without opening a browser or operating a heavyweight project-management system.

## Product Purpose

glplan makes project work visible and operable from the terminal. Success means a user can install one binary, run one command, and immediately manage a useful board locally or against a shared server, while AI agents use the same source of truth through MCP.

## Positioning

Unlike browser-first project trackers, glplan treats the terminal and AI agents as first-class operators: humans use a fast TUI and agents use MCP over the same HTTP API and SQLite-backed board.

## Operating Context

- Local-first use on Linux and Windows, with optional remote API use.
- One binary distribution; users should not need to understand the client/server split before getting value.
- Boards can be shared by running `glplan serve`; agents connect through `glplan mcp`.
- Common work: triage, prioritization, task breakdown, moving work across columns, checklist progress, comments, and activity review.

## Capabilities and Constraints

- Go CLI/API/TUI/MCP architecture with SQLite persistence.
- Terminal constraints: keyboard-first interaction, variable viewport sizes, no reliance on mouse or images.
- The user delegated the visual direction with the constraint that it should be “o que for mais foda”: ambitious, polished, and coherent rather than merely decorative.
- Current round includes plug-and-play operation, expanded workflows, and stronger foundation/tests.

## Brand Commitments

Name: glplan. Voice: concise, developer-native, Portuguese interface copy. Product language should feel precise and modern rather than playful or enterprise-heavy.

## Evidence on Hand

README, installer scripts, release workflow, existing TUI/API/MCP implementation, and seeded sample board content. No user testimonials, metrics, or external proof assets exist; do not fabricate them.

## Product Principles

1. First value must be immediate: install, run, board.
2. Human and agent workflows share one truthful backend.
3. Dense information is acceptable only when hierarchy makes it scannable.
4. Keyboard interaction should feel deterministic, fast, and recoverable.
5. Local simplicity beats enterprise configurability until a real workflow demands it.
