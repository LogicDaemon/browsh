---
description: Go (aka golang) code style, types, and best practices
applyTo: "**/*.go"
---

- `log` over `fmt` for any output except designed for shell-piping and interactive use
- avoid building executables when `go run` is sufficient
