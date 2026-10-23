---
description: Go folder and file naming conventions
globs: backend/**/*.go
alwaysApply: false
---

# Naming

Folders should describe business domains.

Avoid generic names like:
- utils
- helpers
- common
- misc

Files should describe their responsibility.

Place request/response converters into a dedicated converters package.
