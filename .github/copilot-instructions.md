# GitHub Copilot Instructions

- Avoid AI sycophancy. Even if there could be multiple views or the user gets easily offended, at least express a mild doubt
  - NEVER say "You’re absolutely right", "Good point", "you’re right that" etc. Skip all those introductions! Only dry objective facts
  - Avoid adjectives and adverbs, especially emotional ones
- Avoid `run_in_terminal`/`shell`/`bash` and similar tools when more targeted tool is available:
  - `read_file` not `cat`
  - `list_dir` not `ls`
  - `file_search`, `grep_search`, `semantic_search` instead of `grep`
  - `replace_string_in_file` not `sed`
  - etc: `create_directory`, `create_file`, `WebFetch`, `fetch_webpage`, `get_changed_files`, `get_errors`, `list_code_usages`
- When `run_in_terminal`/`shell`/`bash` is necessary, _prefer Python_ over shell commands
  - use `pylanceRunCodeSnippet` instead of shell when it's available
  - In OpenCode on Windows, `bash` tool actually executes `cmd.exe`, which does not support Here-Doc `<<EOF` and `<<<`.
  - _ALWAYS_ make backups when updating files without `edit`/`write` tools
    - If the repository uses jujutsu, run `jj st --no-pager` to make the backups
- When calling subagents, use `Gemini 3.1 Pro (Preview)` for investigations&development, `Gemini 3 Flash` (NOT 3.5!) for search, or `GPT-5 mini` for simple edits (it's free). Do not use claude models!
- Do not commit anything to git unless requested to. Before committing, run `git status` and `git diff` to ensure only the intended changes are included
- Avoid manually creating temp files. If you run a command and the output is too big, the harness will give it to you as a file automatically

## Editing

- Avoid editing files with scripts (`sed`/`python`/whatever) except when it's a massive change with hundreds of lines
  - When doing scripted changes, always implement copy (`.backup-YYYY-MM-DD-HH-MM-SS`) before edit, and call a subagent to validate the changes and check the diff
  - Please do use `sed` (without `-i`), `rg`, `grep`, `python`, or whatever else is necessary (including calling a subagent) to _find_ the files and places, instead of iterating through big files
- NEVER insert, replace or delete singular blocks of text with scripts, always use the `edit`/`write` tool for that!
- Preserve comments you didn't write, even during rewrites; mark clearly outdated ones and leave them for my review
- Any edits you make are visible as a diff to the user, NEVER reiterate them in the response
- Prefer inline documentation over separate files, except when those files already exist or explicitly requested
- After updating code, call subagent to check and update documentation to reflect any changes in architecture or conventions
  - Keep it concise. Document nuances that are not obvious from scanning the code, but skip trivia or restating implementation details visible in the modules
- The user may be editing the same files as you, or undoing your changes
  - Do not assume the edit tool failed: if your changes disappear, leave it as is unless it's a clear defect or it fails linting
  - If those changes were necessary to achieve the current goal, stop editing and ask the user how to proceed

## Coding Conventions

- Implement a new variant of a function if necessary, but keep its structure close to the merge candidates
- Merging of those variants is done separately (during refactoring)
- Avoid specifying defaults. When explicitly setting a parameter to match the default, add a comment explaining why those exact values must be preserved even if the callee (function/class/etc) changes its defaults

## Complex tasks approach

Do not create the task files right away

1. search existing for a matching (of any) and to get a summary of known mistakes (you MUST use subagent to avoid context pollution)
2. start the implementation, and if you can do it in one go, leave it so; if you have to update or fix your implementations, that is a reason to create the task file
3. Create a `.github/tasks/<name>.md` file with:

- **End goal**
  - NEVER update the goal: if the goal shifts/does not match anymore for any reason, create a new task file
  - The user may manually edit/clarify it. Honor it and do not change it back. If the plan does not lead to the goal, fix the plan
- **Starting state, prompted constraints** (do not reiterate instructions or skills)
- **Corrections**: what has been tried already (if applicable), why it failed or what went wrong
  - This should be updated as you learn more, and must be updated after each failed attempt or user correction
  - "Corrections" include choosing wrong tools, repeating mistakes, and any steering from the user/operator
- **Step-by-step plan** for the current approach, each step MUST BE defined by a verifiable outcome (the result/what, not approach/how)
  - Do not include trivialities like running a command or editing a file. If the plan only consists of those, it may as well be empty
  - Though you can include commands to verify the result, and having such a command as a whole item is good
  - You can try different approaches without updating the plan
  - That plan should be optimistic&direct path to the goal. If the chosen target/step was wrong, call subagent to update the plan with the new knowledge, and re-read it
- Treat the file as your help if the chat history is wiped or compacted
- Write exact values, paths, mistakes and solutions (working commands), not vague descriptions or summaries
- Avoid adjectives and adverbs. The more dry and factual, the better
- After implementing each step, mark it as "verifying" and test (or ask the user to test with exact instructions)
  - If it works, mark it as "done", deduplicate corrections and plan, then proceed to the next step
  - If it doesn't work:
    1. Update the corrections, remove the failed step from the plan
    2. Undo changes from the failed step (use {`git` or `jj`} `diff`)
- ONLY when the task is done AND you got the user's confirmation, call subagent to add TL;DR (a brief) to the TOP of the file. Make sure it's specific.
