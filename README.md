# CodeBound `v0.1.0-alpha`

 **CodeBound is an opinionated Go CLI for scaffolding consistent project structures and building toward enforceable architectural boundaries.**

 Starting a new Go project often means recreating the same structure, configuration, server, router, database, and migration setup over and over.

 CodeBound gives you a consistent starting point from a single command:

```
codebound init example-api
```

 > **Alpha Notice:** CodeBound is currently in **v0.1.0-alpha**. APIs, project layouts, and behaviors may change as the project evolves and we gather community feedback.

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
codebound init example-api
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

 ### Create a project

 Initialize a new Go project with CodeBound's default structure:

```
codebound init example-api
```

 ### Create a project with a custom module path

 You can also specify the Go module path:

```
codebound init example-api -m github.com/repository/example-api
```
or
```
codebound init example-api --mod github.com/repository/example-api
```

---

 ## Generated Project Structure

 Running:

```
codebound init example-api
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

 CodeBound is currently **`v0.1.0-alpha`**.

 ### Available

 - Go project scaffolding
- Opinionated project structure
- Custom Go module paths
- CLI-based project initialization

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