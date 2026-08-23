# Overflow behavior

This fixture contains both regular and intentionally oversized slides.

- Slide 2 checks the heading-width regression from issue 12.
- Slide 3 must trigger browser and terminal overflow warnings.
- After the warning collapses, its bottom-left icon must expand it again.
- The oversized slide must scroll before keyboard controls advance.

---

## Lösung: Mock-Server + Env-Var Override

### Schritt 1: Mock-Server starten

This heading should wrap only when the available slide width requires it.

---

## Intentionally oversized slide

### Step 1: Install

```bash
npm install --save-dev some-package another-package and-one-more
```

### Step 2: Configure

```json
{
  "key": "value",
  "another": "value",
  "third": "value"
}
```

### Step 3: Run

```bash
npm run build && npm run start
```

- First bullet point with some explanation
- Second bullet point with some explanation
- Third bullet point with some explanation
- Fourth bullet point with some explanation
- Fifth bullet point with some explanation

---

## After the overflow

Reaching this slide with Down, Page Down, or Space confirms that the oversized
slide consumed those controls for scrolling before advancing.
