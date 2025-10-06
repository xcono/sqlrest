## PostgREST compatibility analysis (sql-to-rest vs our builder)

This report compares supabase-community/sql-to-rest TypeScript processors (SQL → PostgREST params) with our Go builder (PostgREST URL → SQL) and recommends changes to improve compatibility, simplicity, and parity with PostgREST clients like postgrest-js.

### Scope and references
- Renderers/Processors reviewed (TypeScript):
  - `filter.ts`, `aggregate.ts`, `limit.ts`, `select.ts`, `sort.ts`, `types.ts`, `util.ts`
  - Repo: `https://github.com/supabase-community/sql-to-rest`
- Our code (Go):
  - `builder/query.go`, `builder/join.go`, tests in `builder/query_test.go`, `builder/join_test.go`

### High-level differences
- **Direction**: TS converts SQL AST → PostgREST parameters; ours parses PostgREST URL params → SQL.
- **Feature coverage (TS > Go)**:
  - Text search operators: `fts`, `plfts`, `phfts`, `wfts` (Postgres), `match`, `imatch`.
  - Logical NOT with filter-level `negate` and boolean expressions.
  - ORDER BY with `NULLS FIRST/LAST` and explicit direction handling.
  - Aggregates and GROUP BY validation (`count`, `avg`, `sum`, `min`, `max`).
  - JSON path targets (e.g., `col->>'path'`).
- **Feature coverage (Go ≥ TS in areas we target)**:
  - URL param parsing for standard operators and logical `and/or`.
  - JOIN embedding via `select=posts!inner(id,...)` with aliasing and nested embeds.

---

### Findings and concrete improvement opportunities

#### 1) JOIN ON conditions should use table aliases
- Today, if `EmbedDefinition.OnCondition` is provided (e.g., `users.id = posts.user_id`), we embed it verbatim in the JOIN. This conflicts with aliasing (`FROM users AS t1`) and can produce invalid SQL when referencing the original table names.
- Recommended: rewrite ON conditions to the generated aliases (`t1.id = t2.user_id`).
  - Map referenced table prefixes to aliases via `JoinAliasManager.GetAllAliases()` and replace `\b<table>\.` → `<alias>.`.
  - Keep a safe, token-aware replacement (regex on identifiers) to avoid partial replacements.
- Location:
  - `builder/query.go` → `buildJoinClause()`.

Example rewrite logic (new code):
```go
func rewriteJoinConditionToAliases(cond string, aliases map[string]string) string {
    // crude but effective: replace `table.` with `alias.` using word-boundaries
    for table, alias := range aliases {
        re := regexp.MustCompile(fmt.Sprintf(`\b%[1]s\.`, regexp.QuoteMeta(table)))
        cond = re.ReplaceAllString(cond, alias+".")
    }
    return cond
}
```
Then in `buildJoinClause`:
```go
aliases := aliasManager.GetAllAliases()
joinCondition = rewriteJoinConditionToAliases(joinCondition, aliases)
```

#### 2) ORDER BY: support `nullsfirst/nullslast` and qualify with aliases
- TS `sort.ts` requires explicit direction and nulls policy (`first/last`). PostgREST supports `order=col.desc.nullsfirst`.
- Our parser currently normalizes `column.desc` → `column DESC` but ignores `nullsfirst/nullslast`, and does not alias-qualify.
- Recommended:
  - Parse `order` parts into structured components `{ column, direction, nulls }`.
  - Convert to builder calls with alias-qualified columns:
    - If the order column is `table.col`, map `table` → alias and output `alias.col`.
  - Append `NULLS FIRST/NULLS LAST` when present (Postgres) and gracefully ignore on engines that don’t support it.
- Location:
  - `builder/query.go` → in `ParseURLParams` order section and `BuildSQL` application.

Example URL: `?order=created_at.desc.nullslast,name.asc`
Expected (Postgres flavor): `ORDER BY t1.created_at DESC NULLS LAST, t1.name ASC`.

#### 3) Add NOT operator support
- TS supports boolean NOT by setting `negate: true` on a filter or flipping a logical expression.
- Our builder defines `OpNot` but does not parse nor apply it.
- Recommended options:
  - Support logical `not` wrapper param like `not=(status.eq.1)` mirroring our `or/and` parentheses format, or
  - Support `not.<op>` operator prefix per PostgREST convention (`name=not.ilike.*foo*`).
- Implementation sketch:
  - In `parseFilterParam`, detect `not.` prefix; wrap the produced filter in a `LogicalFilter` that negates when building SQL, or represent as a filter with `Negate` flag.
  - In `applySimpleFilter`/`buildSimpleCondition`, if `negate` is set, wrap the condition with `NOT(...)`.

#### 4) Regex match and Postgres FTS operators
- TS `filter.ts` implements `match`/`imatch` (SQL regex `~`, `~*`) and text search: `fts`, `plfts`, `phfts`, `wfts`.
- Our builder lacks these; PostgREST users expect them (especially FTS on Postgres).
- Recommended:
  - Add operators:
    - `match`, `imatch`: emit vendor-specific syntax
      - Postgres: `col ~ ?`, `col ~* ?`
      - MySQL: emulate with `REGEXP`/`RLIKE` (caveat differences)
    - `fts` variants: Postgres only. Map to `to_tsquery`, `plainto_tsquery`, `phraseto_tsquery`, `websearch_to_tsquery`; use `to_tsvector(col) @@ <queryFn>(...)` pattern.
  - Gate by SQL flavor (builder could accept a `Flavor` or infer from driver).
- Location:
  - `builder/query.go` → operator constants, `parseFilterParam` (`validOps`) and `applySimpleFilter`/`buildSimpleCondition`.

#### 5) Aggregates and GROUP BY parity
- TS `select.ts` + `aggregate.ts` supports aggregates (`avg`, `count`, `max`, `min`, `sum`) and validates GROUP BY.
- Our builder doesn’t yet support aggregates in `select`.
- Recommended (incremental):
  - Minimal: parse `select=count(*),avg(price)` and emit `SELECT COUNT(*), AVG(t1.price) ...`.
  - Validate `GROUP BY` if any non-aggregate targets are present.
  - Return columns with sensible JSON keys (respect aliases if provided).
- Location:
  - `builder/query.go` → `ParseSelectWithEmbeds` to recognize aggregates (tokens that look like `fn(args)` and are in an allowlist); `buildSelectClause` to render.

#### 6) Qualify ORDER and WHERE columns with aliases consistently
- We alias SELECT columns, but `ORDER BY` and user-supplied filters like `users.status` pass through unchanged. This can create ambiguity after JOINs.
- Recommended: when a filter column has a qualified form `table.column`, rewrite `table` to alias.
  - Do the same in ORDER by step.
- Location:
  - `builder/query.go` → before applying filters/order, normalize column prefixes using `JoinAliasManager`.

#### 7) Unify embed parsing between `select` and legacy `embed` param
- Our `embed` param support sets `Table: "author(profile)"` as a single string for nested syntax, which can’t be used for JOIN building reliably.
- Recommended: pipe legacy `embed` values through `EmbedParser` (same as `select` handling) to produce structured `EmbedDefinition` (table, joinType, columns, nestedEmbeds), not a raw string.
- Location:
  - `builder/query.go` → `ParseURLParams` legacy embed branch; reuse `parseEmbedFromSelect` for each item.

#### 8) Optional FK-based ON detection in `EmbedParser`
- We already have `ForeignKeyResolver` and `EmbedParser` support, but we pass `nil` in `parseEmbedFromSelect`.
- Recommended: allow `PostgRESTBuilder` to be constructed with an optional `*sql.DB` (or a strategy) to enable automatic ON detection when caller provides a DB. Default remains current behavior.
- Location:
  - `builder/query.go` → `parseEmbedFromSelect()`; 
  - expose a `NewPostgRESTBuilderWithDB(db *sql.DB)` that sets an internal `fkResolver`.

---

### Code hotspots (current behavior)

- Alias generation and use:
```343:353:builder/query.go
// Build FROM clause with main table
sb.From(fmt.Sprintf("%s AS %s", q.Table, mainTableAlias))
```
```717:721:builder/query.go
// Apply JOIN with appropriate type
joinOption := embed.JoinType.ToSQLJoinOption()
sb.JoinWithOption(joinOption, fmt.Sprintf("%s AS %s", embed.Table, embedAlias), joinCondition)
```

- ON condition not rewritten to aliases if provided by user:
```706:715:builder/query.go
// Build JOIN condition
joinCondition := embed.OnCondition
if joinCondition == "" {
    joinCondition = fmt.Sprintf("%s.id = %s.%s_id",
        aliasManager.GetAlias(parentTable),
        embedAlias,
        parentTable)
}
```

- ORDER parsing ignores nulls policy:
```99:117:builder/query.go
if orderParam := params.Get("order"); orderParam != "" {
    orderParts := strings.Split(orderParam, ",")
    for i, part := range orderParts {
        part = strings.TrimSpace(part)
        if strings.Contains(part, ".") {
            parts := strings.Split(part, ".")
            if len(parts) == 2 {
                column := parts[0]
                direction := strings.ToUpper(parts[1])
                if direction == "DESC" || direction == "ASC" {
                    orderParts[i] = column + " " + direction
                }
            }
        }
    }
    query.Order = orderParts
}
```

---

### Examples (expected behavior after changes)

- Aliased JOIN condition rewrite
  - Input: `select=id,name,posts!inner(id,title)` with `posts.OnCondition = "users.id = posts.user_id"`
  - Output snippet: `... FROM users AS t1 INNER JOIN posts AS t2 ON t1.id = t2.user_id ...`

- Order with nulls handling and aliasing
  - URL: `?order=created_at.desc.nullslast,name.asc`
  - SQL (Postgres): `ORDER BY t1.created_at DESC NULLS LAST, t1.name ASC`

- NOT operator
  - URL: `?name=not.ilike.*test*`
  - SQL: `WHERE NOT (LOWER(name) LIKE LOWER(?))`

- Regex and FTS (Postgres)
  - URL: `?title=match.^foo.*&content=fts.(english,bar|baz)`
  - SQL: `WHERE title ~ ? AND to_tsvector(content) @@ to_tsquery(?, ?)`

- Aggregates
  - URL: `?select=count(*),avg(price)&group=price`
  - SQL: `SELECT COUNT(*), AVG(t1.price) FROM products AS t1 GROUP BY t1.price`

---

### Suggested implementation plan (incremental)
1. Alias correctness
   - Rewrite ON conditions to aliases (safe regex replace).
   - Qualify ORDER and filter columns with aliases.
2. ORDER nulls policy
   - Parse `nullsfirst/nullslast`; append for Postgres.
3. NOT operator
   - Support `not.<op>` and/or a `not=(...)` wrapper.
4. Aggregates (minimal)
   - Recognize `count`, `avg`, `sum`, `min`, `max` in `select`; render; basic GROUP BY validation.
5. Regex + FTS (gated by flavor)
   - Add `match/imatch` and `fts/plfts/phfts/wfts` for Postgres.
6. Legacy embed parsing
   - Route `embed` through `EmbedParser` for structured embeds.
7. Optional FK resolver
   - Expose constructor that wires `ForeignKeyResolver` to `EmbedParser`.

---

### Test coverage updates
- Add tests for:
  - ON alias rewriting in JOINs (assert ON uses `t*` aliases).
  - `order=col.desc.nullsfirst` rendering.
  - `not.ilike`, `match`, and FTS operators (Postgres flavor only).
  - Aggregates and GROUP BY combinations (success and validation failures).
  - Legacy `embed` parsing producing proper JOINs.

---

### Compatibility notes
- Some features are engine-specific (e.g., `NULLS FIRST/LAST`, FTS, regex semantics). Our builder should either:
  - Detect SQL flavor and adapt, or
  - Document the supported subset per driver (MySQL, Postgres, SQLite) and return clear errors when unsupported.

---

### References to TS implementation
- Filters and operators: [`filter.ts`](https://github.com/supabase-community/sql-to-rest/blob/main/src/processor/filter.ts)
- Aggregates and GROUP BY: [`aggregate.ts`](https://github.com/supabase-community/sql-to-rest/blob/main/src/processor/aggregate.ts)
- ORDER BY mapping: [`sort.ts`](https://github.com/supabase-community/sql-to-rest/blob/main/src/processor/sort.ts)
- LIMIT/OFFSET: [`limit.ts`](https://github.com/supabase-community/sql-to-rest/blob/main/src/processor/limit.ts)
- SELECT and JOIN (embedded targets): [`select.ts`](https://github.com/supabase-community/sql-to-rest/blob/main/src/processor/select.ts)

This plan aligns our builder with PostgREST expectations and the sql-to-rest mapping, reducing surprises for postgrest-js clients while keeping our core architecture intact.
