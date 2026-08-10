module tokensale

go 1.26.5



// This is Go's module manifest — the file that turns a directory into a Go module:
// - module tokensale — declares the module's import path/name. Since there's just one package main, this mostly matters for the build to know what to call the binary and how imports would resolve if this had sub-packages.
// - go 1.26.5 — declares the minimum Go language version/toolchain this module requires, so go build/go test know which language features are allowed.

// There's no require block because you haven't imported any third-party packages yet — only standard library (errors, fmt).

