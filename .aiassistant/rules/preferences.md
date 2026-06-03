---
apply: always
---

# Preferences

- Go 1.26+: prefer `new(value)` for literal/simple-expression pointers, e.g. `new(true)`, instead of temp variables used only for `&tmp`.
- Do not add redundant `strings.TrimSpace` or nil checks for normalized domain entities whose invariants already guarantee validity.
