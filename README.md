# CodeBound `v0.1.4-nightly.7`

**CodeBound is a Go CLI built to help reduce architectural entropy in AI-generated code.**

AI makes it easier to generate code, but keeping that code coherent as a project grows takes more than working functions. Inconsistent structure, blurred responsibilities, and unintended dependencies can make a codebase harder to understand and maintain.

CodeBound helps developers start with a consistent foundation and inspect how their code connects through call-tree visualization, context-flow inspection, and architectural annotations.

The goal is not to generate more code—it is to keep growing codebases understandable and aligned with their intended architecture.

Start a new Go project:

```
codebound create example-api
```

 > **Prerelease Notice:** CodeBound is currently in **v0.1.4-nightly.7**. APIs, project layouts, and behaviors may change as the project evolves and we gather community feedback.

---

 ## Why CodeBound?

Here, **architectural entropy** means the gradual loss of consistency and clarity in a codebase: responsibilities spread across unrelated packages, dependencies bypass intended layers, and new code introduces competing patterns instead of following existing ones. This can happen with any code, but rapid AI-generated changes can make that drift harder to review.

CodeBound approaches this through:

- **A consistent starting point:** opinionated Go scaffolding gives new projects a predictable foundation.
- **Visibility into existing code:** call-tree and context-flow inspection help you review how functions connect and where contexts are created and passed.
- **Architectural feedback:** naming-based annotations highlight possible layer violations. Explicit, configurable boundaries and validation are longer-term goals, not current enforcement capabilities.

CodeBound does not measure an entropy score or guarantee the quality of generated code. It provides structure and visibility to support human review, whether code is written by a developer, an AI tool, or both.

For a new project, instead of manually creating:

```
cmd/
internal/
├── config/
├── database/
├── router/
└── server/
migrations/
Makefile
```

 you can start with:

```
codebound create example-api
```

---

 ## Installation

 Make sure you have [Go](<https://go.dev/>) installed, then install CodeBound:

```
go install github.com/PranavJoshi2893/codebound@v0.1.4-nightly.7
```

 Verify the installation:

```
codebound --version
```

This is a prerelease, so use the explicit version above rather than `@latest`,
which may resolve to a different release. The browser call-tree viewer also
requires Graphviz's `dot` executable in `PATH`.

---

  ## Quick Start

  ### Interactive setup

Run CodeBound without arguments in an interactive terminal:

```sh
codebound
```

The Charm-based menu offers two actions:

- **Create a project:** enter a name and choose whether to use a custom module
  path. Choosing **No** uses the project name. Submission creates the project
  using the same scaffolding logic as `codebound create`.
- **View a call tree:** enter a project directory (press Enter for the current
  directory). The app discovers executable entry points, asks you to select one
  only when there are multiple, and starts the browser viewer on an available port.

Press `Ctrl+C` to cancel any prompt. With redirected input or output, the root
command displays help instead of prompting. The call-tree menu and direct command
share the same analysis, rendering, and viewer code.

Project names retain the CLI's existing rules: at least five characters, starting
with a letter, and containing only letters, digits, hyphens, or underscores.
Module paths are validated before creation. The wizard trims surrounding input
whitespace before validation and submission.

 ### Create a project

 Initialize a new Go project with CodeBound's default structure:

```
codebound create example-api
```

 ### Create a project with a custom module path

 You can also specify the Go module path:

```
codebound create example-api -m github.com/repository/example-api
```
or
```
codebound create example-api --mod github.com/repository/example-api
```

### Inspect a call tree

From inside a project, no flags are needed:

```sh
cd my-project
codebound calltree
```

Or pass the project directory directly:

```sh
codebound calltree ~/Projects/my-project
```

All packages (`./...`) are loaded by default, including nested executables such
as `cmd/api`. A single executable `main` is selected automatically. Multiple
entry points open a Huh selector when running the browser view in a terminal.
Exports and non-interactive invocations never prompt: they list the choices and
ask for `--root` instead. Libraries with no executable entry point also require
an explicit `--root`.

Open the printed local URL (the port is chosen automatically). Press `Ctrl+C`
to stop the server. The browser view requires Graphviz's `dot` executable in
`PATH`; the Go toolchain must also be installed to load and analyze packages.

In the browser, use **+ / −** or the mouse wheel to zoom, and drag to pan.
**Fit graph** shows the full diagram without enlarging a small graph.
**100% size** restores readable natural size. Keyboard shortcuts: **+ / −** to
zoom, **arrow keys** to pan, **0** or **Home** to fit, and **1** for natural size.
The percentage is absolute: **100% always means the same rendering scale**.
Graph changes preserve zoom and the selected node's screen position. Resizing
preserves manual zoom and the graph point at the center; after an explicit fit,
resizing keeps that diagram fitted. Status/legend updates do not resize the
center canvas. Large diagrams start at a readable root, not a tiny overview;
use **Fit graph** or **Selected + neighbors** when needed.
Boxes can grow when context text is revealed, but the diagram is not automatically
rescaled to compensate.

Export without Graphviz:

```sh
codebound calltree ./my-project --format dot > calltree.dot
codebound calltree ./my-project --format json > calltree.json
```

| Flag | Default | Purpose |
| --- | --- | --- |
| `--path` | `.` | Alternative to the positional project directory; do not use both |
| `--pattern` | `./...` | Override which packages are loaded |
| `--root` | Auto-discover | Function name, `Type.Method`, or full package-qualified name |
| `--format` | `web` | Browser viewer, `dot`, or `json` |
| `--port` | `0` | Automatically choose a browser-view port; set a number for a fixed port |

For ambiguous names, qualify the root, for example
`--root example.com/app/internal/server.Server.Start`. Source packages and their
dependencies must type-check under the active Go environment and build tags.

The graph includes reachable named function and method declarations in the
loaded modules. External-library nodes, anonymous-function nodes, and synthetic
SSA wrappers are not displayed. It uses Variable Type Analysis (VTA) to narrow
down callback and interface targets. These edges still represent possible calls,
not a runtime trace or proof that a call happens. Reflection and `unsafe` can
limit accuracy.

Hidden nodes are still traversed. Dashed **indirect** relationships cross hidden
closures, wrappers, or framework functions; they are not direct calls between the
displayed functions. **Possible HTTP dispatch** marks paths crossing the actual
`net/http` server-to-handler invocation, separately from route configuration.
Constructors remain ordinary startup calls. Library operations such as SQL calls
stay execution rows without expanding library internals into diagram cards.

JSON includes shared `evidence` nodes/edges and each indirect call's `path.entry`
and `path.exit` edge indexes. All hidden-only walks between those edges are retained,
including alternatives and cycles; `http_dispatch` selects walks crossing that
boundary (false selects those without it). Evidence records actual SSA instruction
addresses and source locations, even for synthetic calls without source positions.
Hidden context-creation facts stay in the evidence so their origin IDs remain
traceable without adding library-function cards.
An indirect relationship's `site` and `execution` identify its entry, while context
bindings describe the exit arguments. Unsupported origins remain unresolved, not
assumed to inherit a startup or request context. The browser details show the entry
and exit evidence; JSON retains the complete shared corridors.

#### How the call tree works

1. **Read the source.** Load and type-check the selected Go packages and their
   dependencies.
2. **Build an analysis-friendly representation.** Convert the source into SSA
   (Static Single Assignment), a form that makes value assignments easier to track.
3. **Find possible calls.** Direct calls such as `run()` identify their target
   immediately. For callbacks and interface methods, VTA follows which function
   values and concrete types can reach the call site.
4. **Show the project graph.** Starting at the selected root (usually `main`),
   follow raw calls through hidden nodes, project named project functions,
   annotate direct architecture boundaries, and
   export the result to the browser, DOT, or JSON.

For example, the generated project contains:

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()
err := run(ctx)
```

Both `stop` and `main` have the signature `func()`, but that does **not** mean
`stop` can call `main`. The old Class Hierarchy Analysis (CHA) matched callback
targets by signature, creating a false `main → main` arrow. VTA tracks where
`stop` comes from instead, removing that false arrow while preserving genuine
recursive calls. It does not run your application and may still include targets
from branches that are never taken at runtime.

Naming-based architecture checks annotate calls in every output format:

- Handler-to-service and service-to-repository calls are allowed.
- Direct handler-to-repository calls and other reversed/skipped layers are flagged.
- Calls between different service packages must use an interface.
- Unrecognized layers are left unclassified. Detection is heuristic, not a
  configurable architecture policy or an enforcement gate; findings do not
  change the command's exit status.

### Execution order inside function blocks

The browser keeps the package grouping and function-to-function map, but each
function now contains small **control-flow blocks**. Start at the entry block,
read its rows downward, then follow its labeled exits:

- **True / false** preserve alternative paths and joins, including short-circuit
  conditions, switches, early returns, and select dispatch.
- **Loop → Bn** explicitly returns to an earlier block. Block placement and block
  numbers are addresses, not a global execution sequence.
- **go** launches concurrent work; the caller continues without waiting.
- **Register defer** schedules work. The **deferred stack** runs in reverse
  registration order at exit; branches and loops determine which registrations
  actually happened. Panic exits unwind the stack too; a possible recovered-panic
  entry is labeled separately, not inserted into the normal sequential path.
- **Closures** have separately labeled entries. Declaration does not execute
  their bodies; local call/spawn/defer rows identify a known closure target.

Order comes from SSA instructions and control-flow edges, not line sorting:
`outer(inner())` shows `inner()` first. Rows cover calls and channel actions, not
every value assignment or implicit runtime operation. External calls are shown
as steps but their bodies are not expanded. This is possible source execution
flow, not a runtime trace or a total ordering between goroutines.

Function cards use up to three horizontal columns instead of stacking everything:
alternative branches share a row, their joins sit below all incoming paths, and
independent closure panels can sit beside the body. This placement does not imply
concurrent execution. Long straight-line blocks use contiguous, numbered step
ranges; read down a range, then follow **continue → Bn.step** to the next column.
Only the last range shows the actual return/branch/loop exit. Block IDs, call-site
IDs, context positions, and the analysis evidence are unchanged.

Panels containing outgoing calls expand to the card's sides so connectors attach
visibly to the actual call rows without crossing neighboring panels. Other panels
remain compact. A **filled source dot** identifies an exact call row; a **hollow
dot** means the source row is unavailable. Distinct call rows have separate
connectors, while repeated evidence at the same row shares a route. Context notes
and bindings stay with their own call site, including closures and deferred
registrations. JSON retains optional `flow` and `site` evidence; the detailed DOT
export keeps its existing presentation.

Hover a calling row or connector to highlight its source, path, and destination
while dimming unrelated content. Click a connector to pin the trace; click it
again or press **Esc** to clear. Keyboard-focus a function to trace its outgoing
calls. Traces stay aligned during zoom/pan and reset when the diagram changes.

### Router-to-handler wiring

The browser shows route registration separately from request dispatch for **Chi
v5 and Go 1.22+ `net/http.ServeMux`**. A thin connector labeled **registers
GET /items/** ends in a **hollow diamond**, not a call arrowhead. Registration
assigns a handler; it does not invoke it or pass request context. Existing possible
HTTP-dispatch paths remain separate.

Registration links are visible by default. Uncheck **Show route registrations**
to hide this secondary layer. Hover/pin tracing, context mode, and **Selected +
neighbors** also work with registration links. Function details include separate
**Registers routes** and **Registered by** sections with source/prefix evidence.
Self-registration stays as an explicit **◇ registers … → this function** note at
its row rather than a tangled exterior loop.
Feedback registration connectors also keep their method/path caption at the source
row, so it cannot collide with forward-call labels.

Chi recognition composes known `Route`/`Group` prefixes through configuration
callbacks and project helpers. ServeMux supports instance and package-level
`Handle`/`HandleFunc`, named/bound/inline function handlers, HandlerFunc adapters,
and statically known concrete `ServeHTTP` methods. Separate router allocations
retain separate identities; these identify static configuration contexts, not
unique runtime objects. Executable package initialization is inspected without
inventing `main → init` call edges.

ServeMux patterns retain methods, hosts, wildcards (`{id}`, `{rest...}`, `{$}`),
and trailing slashes. A methodless pattern is **ALL**; GET also matches HEAD under
modern ServeMux rules. The analyzer does not infer deployed runtime settings:
`GODEBUG=httpmuxgo121=1` enables legacy pattern behavior.

Dynamic prefixes/patterns, ambiguous handler values, opaque adapters, unsupported
Chi mounts, and bounded/recursive configuration remain explicitly unresolved or
partial. Nil handlers are findings, not invented destinations. A handler known
only from registration is not classified as request-reachable. Registration never
changes context propagation or architecture-call checks. JSON adds optional
`registrations` and `registration_warnings`; detailed DOT remains call-only.

### Context usage in the call tree

The browser's **Calls + context** mode uses one diagram: context travels along
the existing function-to-function call arrows, with creation and derivation
shown inside the owning function. There are no separate context branches.
Detailed DOT exports also include standalone lifecycle markers.
No extra CLI flag is needed; the browser starts with a simpler call-only map.

The HTML viewer opens with a **Start here** reading guide and an arrow/context
legend. Use **Explore functions** to search by name, package, or file, then read
one function's callers, possible calls, and context actions in plain language.
Select a caller or callee to continue exploring, or **Back to root** to restart.
Source locations and expandable technical evidence remain available for deeper
inspection.
Large call/finding lists initially render 50 items with explicit **Show next**
controls, so the dashboard stays responsive without discarding evidence.

The **diagram starts with a left-to-right call map** from a blue **ROOT**;
execution rows inside each function still read top-to-bottom.
Functions in the same package share one light group, with the package named once
on its first function; single-function packages keep a subtitle without a group.
When the graph contains recorded HTTP dispatch, package groups are split into
**ENTRY / LIFECYCLE** (blue), **HTTP REQUEST** (teal), and **SHARED · ENTRY + HTTP**
(amber). Entry-side work includes setup, route registration, and server/shutdown
logic. Request classification propagates only after a recorded dispatch boundary;
shared functions retain both possibilities. No function names are used to assign
these captions, and focused views retain the full tree's classification.

Tall, private terminal branches can use a spare adjacent layout column instead of
forcing every sibling into one tall stack. Shared targets, feedback/recursive
calls, dispatch boundaries, and package groups keep their relationships intact.
Packing does not introduce calls or change execution order. Multi-column arrow
captions reserve a text-free gutter so turning routes stay clear of their labels.

Ordinary call arrows have no caption. **go** means concurrent, **defer** means on
return, dotted arrows mean possible interface/callback targets, dashed arrows mean
paths through hidden calls, and red arrows mean
naming-based architecture warnings. Full evidence stays in the function details.
These browser-only presentation changes do not alter DOT/JSON exports.

Click a function directly in the diagram to select it (amber corner markers) and
read its details. Choose **Selected + neighbors** to redraw just its direct callers and
callees; click another node to keep following the flow. **+N links** counts hidden
relationships, so a focused view is not mistaken for the whole analysis.
**Whole tree** restores the complete map. Choose **Calls + context** to reveal
context bindings on blue call arrows, including within a focused view. Boundary
functions show their package names at both ends. **new C1** marks creation inside
a function; **C2 ← C1** marks derivation there. Context actions remain at their
actual step within a control-flow block rather than being reordered by kind.
Context evidence is hidden, not discarded, in simple mode. Zoom and pan keep clickable
node regions aligned with the graph; keyboard users can tab to and activate nodes.
Selection, hover, and keyboard focus use separated corner markers outside the
SVG box, rather than drawing overlapping borders on its rounded outline.

For example, a signal context can continue through the call tree without being
drawn as a different context at every parameter:

```text
main [starts C2; deferred stop C2]
  └─ ctx=C2 → run
               └─ ctx=C2 → Server.Run [observes Done: C2]
```

- **`C1`, `C2`, …** identify creation sites in this graph, not unique runtime
  objects. A loop or repeated invocation can create many objects at one site.
- **`I1`, `I2`, …** identify inputs at the selected root. Their creation is
  outside the analyzed tree; receiving them does not count as a new creation.
- **Function boxes** summarize receiving, creating, forwarding, reading, and
  cancelling contexts. `observes Done` is distinguished from merely getting the
  `Done()` channel.
- **Call arrows** match actual SSA arguments to parameters, including interface
  receivers. Multiple possible inputs are labeled `possible`; unsupported flows
  stay unresolved. In the browser, repeated evidence from the same call row to the
  same target shares one arrow listing possible origins and call/go/defer variants.
  Evidence without a known row shares a hollow-dot arrow. **partial**
  marks missing binding evidence on some variants, not proof that no context is
  passed. Detailed DOT exports
  keep distinct context-passing arrows separate.
- **Exported lifecycle markers** show `START`, parent IDs, and possible cancellation
  conditions: timeout/deadline, signals, parent cancellation, or a resolved
  cancel/stop call (including deferred calls). `WithoutCancel` explicitly detaches
  cancellation and deadlines. `Background` and `TODO` have no cancellation.

Browser connectors use exterior left/right ports, with room between card columns
for labels. Feedback calls leave the opposite side without forcing cyclic ranks;
the local execution blocks and call-site references do not change. Positioned
self-recursion appears as a **↻ recursive call → entry Bn** marker at its exact
row, keeping per-site context bindings and scheduling without an exterior loop.
Recursion without position evidence retains an explicit route. Dense graphs
can still have crossings, but narrow white under-strokes distinguish these from
junctions. Relationship evidence is retained, including the local recursion
markers. DOT exports
keep polygonal connectors. Browser aggregation does not drop explorer or JSON
analysis entries.

Full operation details remain in each function's optional `context` JSON field:

| Finding | Meaning |
| --- | --- |
| `parameters` | Parameters declared as `context.Context`, including type aliases |
| `bindings` | Possible source IDs received by each parameter |
| `create` | An independent `context.Background()` or `context.TODO()` |
| `derive` | A child created with cancellation, timeout, deadline, values, or `signal.NotifyContext` |
| `detach` | `context.WithoutCancel`: parent values remain, but cancellation and deadline are not inherited |
| `forward` | A context supplied to another call, including context-aware library APIs |
| `read` | A call to `Done`, `Err`, `Deadline`, `Value`, or `context.Cause` |
| `observe` | A channel receive or select receive case using a traced `Done()` channel |
| `cancel` | A call to a cancellation function, with its originating context when known |

Each JSON operation includes a source location and resolved `ids`, `parent_ids`,
and uncertainty flags when applicable. Each call's optional `contexts` list
records its context argument bindings. Exported lifecycle markers show their creation
locations. Findings distinguish incoming
parameters, derived children, and independent contexts; constant timeouts are
shown as durations, while runtime values are labeled `dynamic`. Calls made with
`defer` or `go` are labeled accordingly. Context facts inside anonymous functions
are attached to the enclosing named function with a separate closure scope;
closure declarations alone do not prove invocation.

This is a conservative source inspection, not a runtime context trace. It follows
local aliases, branch merges, and captured variables, then propagates possible
source IDs across supported calls until the bindings stop changing. Local SSA
expressions are compiled once per query; a dependency worklist revisits only
rules affected by changed parameter bindings. Shared
callees can receive several possible IDs, while individual call arrows retain
their argument bindings. Recursive calls are supported without inventing new
creations for parameters. Origins through fields, globals, custom wrapper
returns, or unsupported indirect/synthetic calls can remain `unresolved` or be
omitted. Escaping pointers add an unresolved possibility. Absence of a finding
is not proof of absence of context usage.

Cancellation conditions are **not proven runtime end points** and are not the
same as a function returning. Merely receiving a context, calling `Done()`, or having a receive case does not
prove a function will stop on cancellation. No cancellation-compliance or leak
verdict is issued. Context keys, values, and cancellation-cause contents are not
exported by this inspection.

### Large projects and performance

Call-tree analysis loads and builds SSA once per analysis session, then lazily
builds and reuses its VTA graph across root queries. Reachable calls are traversed
iteratively with cached function descriptions, preserving depth-first ordering
without a recursive walk. Context IDs use the same loaded SSA and call graph;
there is no second package load. DOT exports use compact identity indexes and
retain detailed findings in JSON. Lifecycle cancellation inheritance is computed
once across the creation graph rather than re-walking each ancestor chain, and
large function labels are assembled in linear time.

The browser server builds its root-first declaration/incident-call index once and reuses it
for selections. Focused DOT generation visits the selected node's neighborhood,
not the entire graph. DOT text is streamed using reusable buffers. Graph JSON is
encoded once, while a byte-bounded LRU caches up to eight on-demand SVG views
within a shared 32 MiB budget (in addition to the initial SVG). Only one Graphviz
layout runs at a time, and slow HTTP clients do not hold the layout/cache lock.
The initial image and click metadata use one request; image replacements are
decoded before committing, and only metadata—not a second full SVG DOM—is parsed
for selection. SVGs remain isolated images.

This is a local development tool, not an unbounded analysis service. Loading,
type-checking, SSA construction, and VTA can still require substantial memory
and time. The result graph remains in memory. For large repositories, narrow the
scope with `--pattern` and use DOT/JSON instead of the browser view if renderer
limits are exceeded:

```sh
codebound calltree . --pattern ./cmd/api --format json > calltree.json
```

Run the synthetic 10,000-edge DOT and loaded 1,000-function-chain benchmarks:

```sh
go test ./internal/calltree/... -run '^$' -bench . -benchmem
```

The benchmark suite also includes forward/reverse context-flow chains and deep
`WithValue` parent chains. These synthetic cases isolate solver and lifecycle
scaling; they are not a promise of whole-project CLI performance.

`BenchmarkBrowserDOT` includes index construction; `BenchmarkIndexedBrowserDOT`
measures repeated queries with that index already built, as used by the server.
`BenchmarkGraphJSON` measures repeated HTTP responses after server setup.
These exclude Graphviz layout and browser painting. For a local comparison on a
synthetic 10,000-call chain, indexed focused DOT preparation fell from roughly
14–22 ms / 8.6 MB to 5–21 µs / 4.4 KB per query. Whole-map preparation with the
index reused took roughly 10–25 ms / 2.7 MB, versus 48–60 ms / 23 MB before this
cleanup (including the old per-query indexing/copies). The index still has a
one-time construction cost; these are not end-to-end CLI or render speedups.

The loaded-analysis benchmark excludes source loading and SSA construction; it
does not measure first-run CLI latency. Benchmark allocation totals are not peak
process memory, and timings depend on hardware and project structure.

---

 ## Generated Project Structure

Generated servers listen on `127.0.0.1` by default. For public deployments,
place the server behind an HTTPS reverse proxy. The generated `.env` file
uses owner-only permissions (`0600`) on Unix systems.

Generated applications do not enable wildcard CORS. Configure an explicit
trusted-origin allowlist if needed. See [security notes](SECURITY.md) for
filesystem protections, viewer restrictions, and remaining trust assumptions.

 Running:

```
codebound create example-api
```

 generates an opinionated project structure:

```
example-api/
├── cmd
│   └── api
│       └── main.go
├── go.mod
├── go.sum
├── internal
│   ├── config
│   │   └── config.go
│   ├── database
│   │   └── database.go
│   ├── router
│   │   └── router.go
│   └── server
│       └── server.go
├── Makefile
└── migrations
```

The goal is to provide a predictable foundation that developers and AI coding tools can extend consistently as the application grows.

---

 ## Architectural Boundaries

Reducing architectural entropy means making intended boundaries explicit, so new code can be reviewed against the project's design rather than judged only by whether it compiles or passes tests.

Today, CodeBound annotates possible layer violations in its call-tree analysis using naming-based heuristics. These annotations support review; they are not configurable policies or an enforcement gate, and findings do not change the command's exit status.

The long-term goal is to let developers define and validate **architectural boundaries**, helping keep both human-written and AI-generated changes aligned with the intended design. The project is still in its early stages, and that enforcement capability is not yet available.

---

 ## Current Status

 CodeBound is currently **`v0.1.4-nightly.7`**.

 ### Available

 - Go project scaffolding
- Opinionated project structure
- Custom Go module paths
 - CLI-based project initialization
- Interactive project initialization with native Huh controls and Lip Gloss styling
- Go call-tree analysis with interface/callback targets and architecture annotations
- Context creation-site IDs, call-argument propagation, and lifecycle/cancellation evidence
- Zoomable, pannable local browser graph viewer and DOT/JSON export
- Interactive action menu, automatic entry-point discovery, and a root selector

 ### Planned

 The project is still evolving. Planned areas include:

 - Configurable project structures
- Custom templates
- Architectural boundary definitions
- Architecture validation
- Additional project-generation options

 The roadmap may change based on community feedback and real-world usage.

---

 ## Who Is CodeBound For?

CodeBound is primarily aimed at:

- Go developers using AI coding tools who want to keep generated changes understandable and consistent
- Teams reviewing existing Go projects for architectural drift and unintended dependencies
- Developers starting new projects who want an opinionated, predictable foundation
- Developers interested in making architectural boundaries explicit

You do not need to adopt CodeBound's scaffolding to inspect an existing Go project. The goal is to support architectural clarity, not impose a universal architecture or replace code review.

---

  ## Contributing

### Code organization

- `main.go`: process signals and exit status; one cancellation context covers every command.
- `cmd/`: independent Cobra command constructors, argument parsing, terminal detection, and adapter composition; no global command state.
- `internal/app/`: shared application workflows and narrow prompt, analysis, browser, and generation boundaries; no Cobra or Charm dependency.
- `internal/tui/`: form, presentation, and answer collection only; no filesystem creation or Cobra dependency.
- `internal/project/`: shared options, validation, and project-creation entry point; no Cobra or Charm dependency.
- `internal/scaffold/`: generators and embedded blueprints; all templates compile into ordered plans before destination changes. Plans use confined, exclusive file creation, and external setup tools share a cancellable runner with bounded diagnostics.
- `internal/calltree/`: package loading, SSA/VTA analysis, context propagation, and architecture checks; no presentation, HTTP, or Graphviz process dependency.
- `internal/calltree/model/`: shared graph evidence and declaration identity, independent of the Go analysis engine.
- `internal/calltree/diagram/`: DOT exports, browser presentation indexes, labels, and lifecycle presentation; depends on evidence, not the analyzer.
- `internal/calltree/viewer/`: Graphviz SVG rendering and a local HTTP viewer with context-driven shutdown; no Cobra dependency.
- `internal/calltree/viewer/assets/`: embedded HTML/CSS and separate camera, graph-request, and text-explorer controllers; no frontend build pipeline or external assets.
- `internal/calltree/viewer/testdata/`: browser-only test scripts, not served by the production viewer.
- `internal/architecture/`: executable import-boundary checks to keep these responsibilities separate.

Run the checks with:

```sh
go test ./...
go vet ./...
```

The Makefile also provides `make check`, `make test-race`, `make test-browser`,
`make test-browser-mobile`, `make bench`, and `make deadcode`. The dead-code check
uses a pinned Go tool and includes tests; it does not automatically delete
exported APIs or reflective callbacks. Context-flow orchestration and the dependency
solver stay in the analyzer; lifecycle presentation and DOT encoding live in `diagram/`.

When Graphviz JSON output is available, tests also check rendered text overlaps,
routes through labels, and tall-fanout compaction.

Optional read-only checks against the rebalancer reference project:

```sh
CODEBOUND_REBALANCER_DIR=/path/to/rebalancer go test ./internal/calltree/diagram ./internal/calltree/viewer -run TestRebalancer -count=1 -v
```

Set `CODEBOUND_LAYOUT_ARTIFACTS` to an existing output directory to also save its
call-only/context browser DOT and SVG files for visual inspection.

Optional browser interaction tests use headless Firefox under the viewer's real
security policy. With `firefox` in `PATH`, run:

```sh
make test-browser
make test-browser-mobile
```

 CodeBound is still in its early stages, and feedback is especially valuable right now.

 If you try CodeBound, feel free to:

 - Report bugs
- Suggest features
- Open an issue
- Propose improvements
- Contribute code

If you have examples of AI-generated changes that made a codebase harder to understand or maintain, **I'd especially like to hear what architectural feedback would have helped**.

---

 ## Feedback

 Try it out and let me know:

 - What you like
- What feels unnecessary
- What you'd change in the generated structure
- What project types you'd like CodeBound to support
- What architectural problems you'd want CodeBound to help solve
- Where AI-generated changes introduce inconsistency or architectural drift in your projects

 The project is intentionally early, so feedback can directly influence its direction.

---

 ## License

 See the repository's license for details.
