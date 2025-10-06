### PostgREST Compatibility Implementation Plan (focused: joins, filtering, escaping, security)

Purpose: Turn findings from `docs/postgrest-compat-analysis.md` into actionable, incremental tasks optimized for stability of the most common PostgREST query patterns. Keep diffs small, preserve parameterization, avoid overfitting to a specific SQL flavor, and bias toward predictable behavior for `postgrest-js` clients.

Scope emphasis (out-of-scope for now): full aggregate suite, advanced FTS/regex, broad PostgreSQL-only features.

---

### Principles and guardrails
- **Safety first**: always build parameterized SQL; never concatenate untrusted strings into SQL.
- **Alias correctness**: all SQL emitted must consistently reference table aliases in SELECT, JOIN, WHERE, and ORDER BY.
- **Determinism**: stable alias order and condition order for testability.
- **Minimal surface**: implement smallest viable feature set per task; defer nice-to-haves.
- **Tests drive behavior**: update/extend tests before or right after code; integration tests should validate end-to-end behavior with realistic inputs.

---

### Milestone 1 — JOIN alias correctness and query normalization (P0)

- Task 1.1: Rewrite user-provided JOIN ON conditions to table aliases
  - Goal: When `EmbedDefinition.OnCondition` contains base table names (e.g., `users.id = posts.user_id`), rewrite to the generated aliases (e.g., `t1.id = t2.user_id`).
  - Impacted files: `builder/query.go` (JOIN building), possibly `builder/join.go` (helper location optional).
  - Acceptance criteria:
    - All JOIN ON conditions emitted in SQL reference only `t*` aliases.
    - Nested embeds also rewrite correctly.
    - Unit tests assert alias-only ON strings in generated SQL.
  - Tests to add/update: extend `builder/join_test.go` and `builder/query_test.go` with user-supplied ON conditions and nested JOIN scenarios.
  - Risks/pitfalls: Avoid partial replacements (use identifier boundaries); don’t alter string literals inside conditions.

- Task 1.2: Qualify filter and ORDER BY columns with aliases
  - Goal: Normalize user-supplied `filters` and `order` so any `table.column` prefixes are mapped to their aliases; unqualified columns default to the main table alias.
  - Impacted files: `builder/query.go` (filter/order normalization step before applying to builder).
  - Acceptance criteria:
    - WHERE and ORDER BY clauses reference only aliases.
    - Existing tests continue to pass; new tests cover mixed `users.status` and `posts.published` cases with embeds.
  - Tests to add/update: `builder/query_test.go` (ORDER/filters with and without explicit table prefixes); `builder/join_test.go` (filters that reference embedded tables).
  - Risks/pitfalls: Don’t resolve prefixes that do not correspond to known tables; avoid altering function calls or literals.

- Task 1.3: Unify legacy `embed` parsing through the EmbedParser
  - Goal: Convert `embed` query param values through the same parser used for `select=...embed...` to produce structured `EmbedDefinition`.
  - Impacted files: `builder/query.go` (ParseURLParams `embed` branch), `builder/join.go` (parser remains the same).
  - Acceptance criteria:
    - Legacy `embed` yields the same JOIN structure as equivalent `select` embeds.
    - Backward-compat tests continue to pass and now verify JOINs appear for legacy `embed`.
  - Tests to add/update: `builder/join_test.go` and `web/integration_test.go` legacy `embed` scenarios.

---

### Milestone 2 — Result shaping for embedded resources (P0)

- Task 2.1: Emit stable, table-based JSON keys in SELECT list
  - Goal: For each selected column, emit an `AS` alias suitable for building nested JSON (e.g., `users.id` or `users__id`; choose one and apply consistently across main and embedded tables).
  - Impacted files: `builder/query.go` (build SELECT list and apply column aliases), optionally helper in `builder/join.go`.
  - Acceptance criteria:
    - Column labels uniquely identify their originating table and column.
    - Does not break existing flat responses when no embeds are present.
  - Tests to add/update: `builder/query_test.go` (assert SELECT contains `AS` aliases for both main and embedded tables).

- Task 2.2: Scanner nests results using emitted keys
  - Goal: Update `web/database/scanner.go` to interpret the chosen key format and build nested maps for embedded relations.
  - Impacted files: `web/database/scanner.go` only.
  - Acceptance criteria:
    - When joins are present, response data contains nested objects keyed by table names; when not present, flat rows remain unchanged.
    - Handles NULLs on outer joins without panicking.
  - Tests to add/update: Extend `web/integration_test.go` with a join + nested response case; verify shape and null-handling.
  - Risks/pitfalls: Mixed delimiters; choose exactly one delimiter and document it here to align builder and scanner.

---

### Milestone 3 — Filtering enhancements and escaping (P1)

- Task 3.1: Minimal NOT operator support via `not.<op>`
  - Goal: Recognize `not.<op>` forms for a small allowlist (start with `eq`, `like`, `ilike`, `is`).
  - Impacted files: `builder/query.go` (parseFilterParam, apply/build condition functions).
  - Acceptance criteria:
    - SQL wraps the emitted condition in `NOT (...)` (or uses an equivalent builder form) while preserving parameterization.
    - Unit tests cover `name=not.ilike.*test*`, `status=not.eq.1`, and `email=is.not.null` parity.
  - Tests to add/update: `builder/query_test.go` logical operators section.

- Task 3.2: Hardening around value parsing and injection attempts
  - Goal: Reconfirm parameterization end-to-end; add test vectors for special chars, quotes, and common injection attempts.
  - Impacted files: none (behavioral tests), verify `web/query/executor.go` path keeps parameterization.
  - Acceptance criteria:
    - New tests in `web/integration_test.go` pass and do not execute injected SQL.
    - No string concatenation of untrusted values introduced by earlier tasks.

---

### Milestone 4 — Optional/minimal aggregates (P2)

- Task 4.1: Recognize minimal aggregate targets in `select`
  - Goal: Allow `select=count(*),avg(price)` and render proper SQL; skip full GROUP BY validation for now.
  - Impacted files: `builder/query.go` (ParseSelectWithEmbeds, SELECT rendering).
  - Acceptance criteria:
    - SQL includes COUNT and AVG projections; arguments are qualified with aliases.
    - Tests cover simple aggregate selection without embeds.

---

### Milestone 5 — Optional FK resolver wiring (P2)

- Task 5.1: Expose builder constructor that wires an optional ForeignKeyResolver
  - Goal: Allow passing a resolver (from config or DB) so `EmbedParser` can prefill `OnCondition` when not provided.
  - Impacted files: `builder/join.go` (EmbedParser ctor remains), `builder/query.go` (builder struct gains optional field and constructor).
  - Acceptance criteria:
    - When resolver is provided and able to detect, JOINs use detected ON conditions (still rewritten to aliases by Task 1.1).
    - Unit tests can stub resolver for deterministic behavior.

---

### Cross-cutting — Testing and determinism

- Keep alias assignment deterministic (main table must be `t1`, then breadth-first across embeds).
- Sort filters where necessary to stabilize SQL for tests (already present; re-validate after changes).
- Expand tests incrementally: unit in `builder/*_test.go`, end-to-end in `web/integration_test.go`.

---

### Delivery order (recommended)
1. Task 1.1 → 1.2 → 1.3 (JOIN correctness and normalization)
2. Task 2.1 → 2.2 (result shaping)
3. Task 3.1 → 3.2 (filters and hardening)
4. Task 4.1 (optional aggregates)
5. Task 5.1 (optional FK resolver)

Each task should:
- Declare impacted files up front and keep edits narrowly scoped.
- Add or update tests in the same commit.
- Preserve parameterization via `go-sqlbuilder`.

---

### Acceptance checklist per task
- Build succeeds and unit tests pass.
- New/updated tests cover both success and failure cases.
- SQL generated uses aliases consistently.
- No unvetted string interpolation of user input in SQL.
- Integration tests demonstrate expected HTTP response shape and headers.

---

### Notes for coding agents
- Prefer small, orthogonal edits; avoid refactors that span multiple milestones.
- Be cautious with code examples in commit messages; focus on intent and acceptance criteria.
- If a test requires changing expected SQL strings, verify that the change improves aliasing/normalization rather than masking a bug.
- When in doubt, align with behaviors expected by `postgrest-js` for common patterns (select, filters, embeddings, limit/offset/order).
