# Mermaid diagram support

This fixture covers the stable diagram baseline and graceful error handling.

---

## Flowchart

```mermaid
flowchart LR
    A[Write Markdown] --> B[Run md-present]
```

---

## Sequence

```mermaid
sequenceDiagram
    participant Browser
    participant Server
    Browser->>Server: Render deck
    Server-->>Browser: SVG
```

---

## Class

```mermaid
classDiagram
    class Presentation {
        +render()
    }
```

---

## State

```mermaid
stateDiagram-v2
    [*] --> Draft
    Draft --> Published
    Published --> [*]
```

---

## Entity relationship

```mermaid
erDiagram
    AUTHOR ||--o{ DECK : writes
```

---

## Timeline

```mermaid
timeline
    title Presentation timeline
    2026-08 : Draft
            : Present
```

---

## Mind map

```mermaid
mindmap
    root((md-present))
        Markdown
        Mermaid
```

---

## Architecture

```mermaid
architecture-beta
    service client(internet)[Client]
    service server(server)[Server]
    client:R --> L:server
```

---

## Invalid syntax remains visible

```mermaid
this is not Mermaid
```

---

## Configuration directives are rejected

```mermaid
%%{init: {"theme": "forest"}}%%
flowchart LR
    A --> B
```
