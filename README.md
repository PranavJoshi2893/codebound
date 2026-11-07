# CodeBound `v0.1.4-nightly.2`

 **CodeBound is an opinionated Go CLI for scaffolding consistent project structures and building toward enforceable architectural boundaries.**

 Starting a new Go project often means recreating the same structure, configuration, server, router, database, and migration setup over and over.

 CodeBound gives you a consistent starting point from a single command:

```
codebound create example-api
```

 > **Alpha Notice:** CodeBound is currently in **v0.1.4-nightly.2**. APIs, project layouts, and behaviors may change as the project evolves and we gather community feedback.

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
go install github.com/PranavJoshi2893/codebound@latest
```

 Verify the installation:

```
codebound --help
```

---

  ## Quick Start

  ### Interactive setup

Run CodeBound without arguments in an interactive terminal:

```sh
codebound
```

The Charm-based wizard asks for a project name and whether to use a custom
module path. Choosing **No** skips the module-path question and uses the project
name. Submitting the form creates the project using the same scaffolding logic
as `codebound create`. Press `Ctrl+C` to cancel before submission without creating
files. With redirected input, the root command displays help instead of prompting.

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

Analyze calls reachable from a function and view the graph in your browser:

```sh
codebound calltree --path ./my-project --pattern ./... --root main
```

Open the printed local URL (by default `http://127.0.0.1:3000`). Press `Ctrl+C`
to stop the server. The browser view requires Graphviz's `dot` executable in
`PATH`; the Go toolchain must also be installed to load and analyze packages.

Export without Graphviz:

```sh
codebound calltree --path ./my-project --root main --format dot > calltree.dot
codebound calltree --path ./my-project --root main --format json > calltree.json
```

| Flag | Default | Purpose |
| --- | --- | --- |
| `--path` | `.` | Project or package directory |
| `--pattern` | `.` | Go package pattern; use `./...` to load all packages |
| `--root` | `main` | Function name, `Type.Method`, or full package-qualified name |
| `--format` | `web` | Browser viewer, `dot`, or `json` |
| `--port` | `3000` | Browser-view port; `0` chooses an available port |

For ambiguous names, qualify the root, for example
`--root example.com/app/internal/server.Server.Start`. Source packages and their
dependencies must type-check under the active Go environment and build tags.

The graph includes reachable named function and method declarations in the
loaded modules. External-library nodes, anonymous function bodies, and synthetic
SSA wrappers are not displayed. It uses conservative class hierarchy analysis
(CHA), so interface and callback edges represent possible targets, not a runtime
trace or proof that a call happens.

Naming-based architecture checks annotate calls in every output format:

- Handler-to-service and service-to-repository calls are allowed.
- Direct handler-to-repository calls and other reversed/skipped layers are flagged.
- Calls between different service packages must use an interface.
- Unrecognized layers are left unclassified. Detection is heuristic, not a
  configurable architecture policy or an enforcement gate; findings do not
  change the command's exit status.

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

 CodeBound is currently **`v0.1.4-nightly.2`**.

 ### Available

 - Go project scaffolding
- Opinionated project structure
- Custom Go module paths
 - CLI-based project initialization
- Interactive project initialization with native Huh controls and Lip Gloss styling
- Go call-tree analysis with interface/callback targets and architecture annotations
- Local browser graph viewer and DOT/JSON export

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
- `internal/calltree/`: package loading, SSA/CHA analysis, graph data, architecture checks, and DOT encoding; no Cobra, HTTP, or Graphviz process dependency.
- `internal/calltree/viewer/`: Graphviz SVG rendering and a local HTTP viewer with context-driven shutdown; no Cobra dependency.

Run the checks with:

```sh
go test ./...
go vet ./...
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
