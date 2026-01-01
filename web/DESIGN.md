# notif.sh Design System

## Philosophy

**Events in motion** - notif is about real-time event streaming. The UI should feel alive, not static.

- Events are the hero, not charts
- Real-time updates are expected
- Keyboard-first, mouse-friendly
- Dense information, zero clutter
- Sharp edges = precision

---

## Visual Language

| Principle | Rule |
|-----------|------|
| Corners | `border-radius: 0` everywhere |
| Colors | Solid only, no gradients |
| Shadows | Solid (no blur): `0 2px 0 #e5e5e5` |
| Theme | Light mode primary |
| Primary | Purple `#a855f7` |

---

## Typography

```
Sans:  Inter
Mono:  JetBrains Mono (code, timestamps, JSON)
```

---

## Layout

```
┌──────────────────────────────────────────────────┐
│  notif   [Events] [Webhooks] [DLQ] [Settings] ⌘K │  Top nav (no sidebar)
├──────────────────────────────────────────────────┤
│  Filter bar                         🔴 Live      │
├──────────────────────────────────────────────────┤
│  Event stream (main content)                     │
│  - Timestamp first                               │
│  - Topic colored                                 │
│  - JSON preview inline                           │
└──────────────────────────────────────────────────┘
```

**Interactions:**
- Click row → Slide-over panel from right
- `⌘K` → Command palette
- `j/k` → Navigate, `Enter` → Open, `Esc` → Close

---

## Colors

```css
/* Primary */
--primary-500: #a855f7;
--primary-600: #9333ea;

/* Neutral */
--neutral-50:  #fafafa;  /* page bg */
--neutral-200: #e5e5e5;  /* borders */
--neutral-500: #737373;  /* secondary text */
--neutral-900: #171717;  /* headings */

/* Semantic */
--success: #22c55e;
--warning: #f59e0b;
--error:   #ef4444;
```

---

## Routes

```
/           → Events (home, live stream)
/webhooks   → Webhook list
/dlq        → Dead letter queue
/settings   → API keys, usage
```
