# Security notes

## Filesystem generation

- Generated files are written through `os.Root` to prevent directory symlinks
  escaping the selected project. Files are created exclusively, never truncated
  or replaced. A new API module requires a new module directory.
- Module names are validated at the generator boundary as well as the command
  boundary. Traversal, separators, and Go keywords are rejected.
- Template rendering finishes before a destination file is opened. Write and
  close errors are returned to the caller.
- Generated `.env` files have owner-only permissions (`0600` on Unix).
- Go module paths cannot start with a dash, and `go mod init` uses an explicit
  argument separator so a path cannot be interpreted as a command flag.

These protections do not make generation transactional. A failing external Go or
Git command can leave a partially generated project; inspect it before retrying.
`os.Root` is not an OS sandbox: it does not protect against hostile bind mounts,
device files, or an attacker with the same operating-system privileges.

## Local graph viewer

- The CLI binds only to `127.0.0.1`, not a public interface.
- The viewer requires a loopback Host header and rejects foreign Origin and
  cross-site browser requests to mitigate DNS rebinding and cross-site access.
- Security headers disable caching, framing, active content, and external
  resources. The page embeds SVG as an image rather than an active document.
- HTTP header, read, write, idle, and graceful-shutdown timeouts are bounded.
- Graphviz rendering is cancelled after 30 seconds. DOT input is limited to
  8 MiB, SVG output to 32 MiB, and diagnostics to 64 KiB.

The viewer has no user authentication. Other local users/processes may access
it. Do not expose it through a reverse proxy, port forwarding, or public tunnel.
Keep the system Graphviz installation patched; Go dependency scanning does not
scan that external executable.

## Generated applications

New applications listen on loopback, have HTTP timeouts, and do not enable
wildcard CORS. Add explicit trusted frontend origins if cross-origin access is
needed. CORS is not authentication or CSRF protection. Add application-specific
authorization, request limits, and deployment security before public exposure.

Template changes affect newly generated projects only. Existing projects must be
reviewed and updated separately.

## Analysis and dependency checks

Analyze trusted Go projects only. Package loading invokes the Go toolchain and
may download dependencies. SSA/CHA analysis is not sandboxed and can consume
substantial CPU and memory on large or adversarial inputs. Renderer limits are
not operating-system memory limits or a sandbox.

Run regression checks and the Go vulnerability scanner:

```sh
go test ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

A clean scan means no known reachable Go vulnerabilities were reported by that
database at scan time; it is not a guarantee that the application is vulnerability-free.
