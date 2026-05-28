- Keep changes to the project minimal and strictly related to the codebase. No temporary files in the project directory.

# Architecture and Entrypoints
- Golang CLI (`interfacer`): Entrypoint is `cmd/browsh`.
- Web Extension (`webext`): Node.js/Webpack.

# Commands
- Interfacer Run: `go run ./cmd/browsh --debug` (logs to `interfacer/debug.log`)
- Interfacer Tests: `go test src/browsh/*.go`, `go test test/tty/*.go`, `go test test/http-server/*.go`
- Web Extension Build: `npx webpack --watch`
- Web Extension Package (for Marionette): 
  1. Build the JS: `npm run build:dev` (inside `webext`)
  2. Package it: `npx web-ext build --source-dir dist --artifacts-dir dist --overwrite-dest`
  3. Move to Browsh embed path as XPI: `move dist\browsh-*.zip ..\interfacer\src\browsh\browsh.xpi` (or `mv` on Unix)
- Web Extension Tests: `npm test`
- Prefer `rg` over `grep` for code and text searches in this workspace.

# Running the Server
- **DO NOT** attempt to start background servers (like Browsh `--http-server-mode`) yourself, as it blocks the execution flow and prevents returning control.
- Instead, **ASK THE USER** to start the server and wait for their confirmation before proceeding.
- By default, the user will run: `go run ./cmd/browsh --http-server-mode`. If you need it run with different flags or arguments, explicitly tell the user.
