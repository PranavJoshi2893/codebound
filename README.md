# CodeBound `v0.1.4-nightly.3`

 **CodeBound is an opinionated Go CLI for scaffolding consistent project structures and building toward enforceable architectural boundaries.**

 Starting a new Go project often means recreating the same structure, configuration, server, router, database, and migration setup over and over.

 CodeBound gives you a consistent starting point from a single command:

```
codebound create example-api
```

 > **Prerelease Notice:** CodeBound is currently in **v0.1.4-nightly.3**. APIs, project layouts, and behaviors may change as the project evolves and we gather community feedback.

---

 ## Why CodeBound?

 Go makes it easy to build simple, maintainable projects, but teams and developers often end up recreating similar project structures manually.

 CodeBound provides an opinionated starting point for Go projects so you can spend less time setting up folders and more time building your application.

 Instead of manually creating:

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
go install github.com/PranavJoshi2893/codebound@v0.1.4-nightly.3
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
**Fit** restores the full graph. Keyboard shortcuts: **+ / −** to zoom,
**arrow keys** to pan, and **0** or **Home** to fit. The displayed percentage is
relative to the fitted graph; resizing the window fits the graph again.

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

#### How the call tree works

1. **Read the source.** Load and type-check the selected Go packages and their
   dependencies.
2. **Build an analysis-friendly representation.** Convert the source into SSA
   (Static Single Assignment), a form that makes value assignments easier to track.
3. **Find possible calls.** Direct calls such as `run()` identify their target
   immediately. For callbacks and interface methods, VTA follows which function
   values and concrete types can reach the call site.
4. **Show the project graph.** Starting at the selected root (usually `main`),
   follow calls to named project functions, annotate architecture boundaries, and
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

### Context usage in the call tree

Context findings appear automatically as **separate context nodes and dashed
arrows** in the browser/DOT graph. Function boxes contain only function identity
and architecture layer. Solid arrows remain function calls; dashed arrows show
context relationships, with cancellation arrows in red. No extra flag is needed.

Context inputs and creation sites use ellipses. Arrows are labeled `receives`,
`creates`, `parent`, `passes context`, `reads`, `observes Done`, or `cancels`.
`WithoutCancel` uses a `values only` parent arrow rather than implying inherited
cancellation. Unresolved or missing origins use diamonds; targets not matched to a displayed
function use separate notes labeled `unmatched context target`, rather than
guessing a concrete destination.

Connectors use straight segments with bends where needed to avoid nodes.
Context actions with the same source, destination, and direction share one arrow
with combined labels; cancellation remains a separate red arrow. Repeated
equivalent function-call edges are drawn once. JSON retains all analysis entries.

Full operation details remain in each function's optional `context` JSON field:

| Finding | Meaning |
| --- | --- |
| `parameters` | Parameters declared as `context.Context`, including type aliases |
| `create` | An independent `context.Background()` or `context.TODO()` |
| `derive` | A child created with cancellation, timeout, deadline, values, or `signal.NotifyContext` |
| `detach` | `context.WithoutCancel`: parent values remain, but cancellation and deadline are not inherited |
| `forward` | A context supplied to another call, including context-aware library APIs |
| `read` | A call to `Done`, `Err`, `Deadline`, `Value`, or `context.Cause` |
| `observe` | A channel receive or select receive case using a traced `Done()` channel |
| `cancel` | A call to a cancellation function, with its originating context when known |

Each JSON operation includes a source location; context creation nodes show their
creation locations. Findings distinguish incoming
parameters, derived children, and independent contexts; constant timeouts are
shown as durations, while runtime values are labeled `dynamic`. Calls made with
`defer` or `go` are labeled accordingly. Context facts inside anonymous functions
are attached to the enclosing named function with a separate closure scope;
closure declarations alone do not prove invocation.

This is a local, conservative source inspection, not an end-to-end runtime
context trace. It follows local aliases, branch merges, and captured variables.
Parameters identify local inputs, and forwarding operations identify outgoing
calls; it does not substitute caller values throughout every callee. Origins
through fields, globals, custom wrapper returns, or unsupported indirect calls
can remain `unresolved`. Escaping pointers add an unresolved possibility, and
merged branches may list several origins. Unsupported usage can be omitted;
absence of a finding is not proof of absence of context usage.

Merely receiving a context, calling `Done()`, or having a receive case does not
prove a function will stop on cancellation. No cancellation-compliance or leak
verdict is issued. Context keys, values, and cancellation-cause contents are not
exported by this inspection.

### Large projects and performance

Call-tree analysis loads and builds SSA once per analysis session, then lazily
builds and reuses its VTA graph across root queries. Reachable calls are traversed
iteratively with cached function descriptions, preserving depth-first ordering
without a recursive walk. DOT exports stream nodes and calls with identity
indexes and compact context-arrow grouping, rather than copying the analysis graph.

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
go test ./internal/calltree -run '^$' -bench . -benchmem
```

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

 The goal is to provide a predictable foundation that can evolve as your application grows.

---

 ## Architectural Boundaries

 One of CodeBound's long-term goals is to help developers define and maintain **architectural boundaries** within their projects.

 The project is still in its early stages, so this area is actively evolving.

 The broader goal is to move beyond simply generating folders and help make architectural decisions **explicit and enforceable**.

---

 ## Current Status

 CodeBound is currently **`v0.1.4-nightly.3`**.

 ### Available

 - Go project scaffolding
- Opinionated project structure
- Custom Go module paths
 - CLI-based project initialization
- Interactive project initialization with native Huh controls and Lip Gloss styling
- Go call-tree analysis with interface/callback targets and architecture annotations
- Local browser graph viewer and DOT/JSON export
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

 - Go developers starting new projects
- Developers who prefer opinionated project structures
- Teams that want consistency across Go projects
- Developers interested in making architectural boundaries explicit

 If you already have a project structure you like, CodeBound may not be useful to you — and that's okay. The goal is to provide a useful opinionated default rather than a universal architecture.

---

  ## Contributing

### Code organization

- `cmd/`: Cobra commands and orchestration; running without arguments opens the wizard.
- `internal/tui/`: form, presentation, and answer collection only; no filesystem creation or Cobra dependency.
- `internal/project/`: shared options, validation, and project-creation entry point; no Cobra or Charm dependency.
- `internal/scaffold/`: existing generators and embedded templates, reused by both entry paths.
- `internal/calltree/`: package loading, SSA/VTA analysis, graph data, architecture checks, and DOT encoding; no Cobra, HTTP, or Graphviz process dependency.
- `internal/calltree/viewer/`: Graphviz SVG rendering and a local HTTP viewer with context-driven shutdown; no Cobra dependency.

Run the checks with:

```sh
go test ./...
go vet ./...
```

When Graphviz JSON output is available, tests also check rendered context-label
positions for overlapping text.

Optional browser interaction tests use headless Firefox under the viewer's real
security policy. With `firefox` in `PATH`, run:

```sh
CODEBOUND_BROWSER_TESTS=1 go test ./internal/calltree/viewer -run TestBrowserGraphNavigation -count=1
```

 CodeBound is still in its early stages, and feedback is especially valuable right now.

 If you try CodeBound, feel free to:

 - Report bugs
- Suggest features
- Open an issue
- Propose improvements
- Contribute code

 If you have an opinion about the generated project structure, **I'd especially like to hear it**.

---

 ## Feedback

 Try it out and let me know:

 - What you like
- What feels unnecessary
- What you'd change in the generated structure
- What project types you'd like CodeBound to support
- What architectural problems you'd want CodeBound to help solve

 The project is intentionally early, so feedback can directly influence its direction.

---

 ## License

 See the repository's license for details.
