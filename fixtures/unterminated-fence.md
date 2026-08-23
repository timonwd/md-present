# Unterminated fenced code block

This fixture intentionally leaves the following fence open. md-present should
warn while preserving Markdown semantics: everything after the opening fence is
code, including the apparent slide separator.

```text
---

## This is code, not a second slide

The fence is intentionally not closed.
