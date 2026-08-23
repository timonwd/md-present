# Fence-aware slide separators

This fixture distinguishes real slide separators from Markdown that merely
looks like one.

```text
---
This separator is code, so it must stay on the first slide.
```

- A nested fenced block also keeps its separator:

  ```text
  ---
  Still nested code.
  ```

---

## Second real slide

Only the separator above starts this slide. Unknown code languages remain
safely escaped plain code:

```custom
<slide>
content remains text
</slide>
```
