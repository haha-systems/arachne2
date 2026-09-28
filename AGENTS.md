# Version Control

- Use `jj` (jujutsu) for version control. DO NOT use git, `jj` will manage the colocated git respository.
- Commit messages MUST be written. Use `jj describe -m "feat: short feature description"`.
- Commit messages MUST be conventional commit style.
- Commit messages MAY include extra detail if that detail would help a human understand the changes better.

# Code Quality and Style

- Linting and formatting MUST be performed before committing. It may be worth instituting this as a hook.
- Linting:
  - Go: use `golangci-lint` with a strict config
  - TypeScript: use `eslint` and/or `prettier` and/or `biome`
  - Python: use `black` or `biome`
  - Ruby: use `rubocop`
- Comments: comments MUST be written for docstrings. Comments MAY be written in code blocks where that comment would help a human understand the code better.

# Cumulative Epistemic State (CES)

- Agents MUST use CES methodology when finding and fixing bugs.
- Agents MAY use CES methodology when developing complex features.
- Agents may not be bound by the deterministic checks defined in CES.md. Exercise the process as faithfully as possible.
