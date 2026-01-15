# notif.sh Web Dashboard

## Overview

Frontend dashboard for notif.sh - a managed pub/sub event hub. Built with TanStack Start (React 19 + TanStack Router + Vite).

## Tech Stack

- **Framework**: TanStack Start
- **UI**: React 19, Tailwind CSS v4
- **Routing**: TanStack Router (file-based)
- **Auth**: Clerk (JWT sessions)

## Authentication

Uses **Clerk** for authentication:
- Users sign in via Clerk
- JWT session passed to backend
- Backend accepts both Clerk JWT and API keys (`Bearer nsh_xxx`)
- API key management requires Clerk auth (can't create keys with keys)

## Design System

See `DESIGN.md` for full brand guidelines.

**Key principles:**
- Zero border radius everywhere
- Solid colors only (no gradients)
- Light mode primary
- Purple primary color (`#a855f7`)
- Inter font (sans), JetBrains Mono (code)

## Project Structure

```
src/
├── routes/           # File-based routing
│   ├── __root.tsx    # Root layout with TopNav
│   ├── index.tsx     # Events page (home)
│   ├── webhooks/     # Webhook CRUD
│   ├── schedules/    # Scheduled events
│   ├── dlq.tsx       # Dead letter queue
│   └── settings.tsx  # API keys, settings
├── components/
│   ├── ui/           # Button, Badge, Input
│   ├── layout/       # TopNav, SlideOver
│   └── events/       # EventRow, EventDetail
├── lib/              # API client, utilities
└── styles.css        # Tailwind theme
```

## Routes

| Route | Description |
|-------|-------------|
| `/` | Events stream (home) - live event feed |
| `/webhooks` | Webhook list |
| `/webhooks/new` | Create webhook |
| `/webhooks/:id` | Edit webhook |
| `/schedules` | Scheduled events list |
| `/schedules/:id` | Schedule details |
| `/dlq` | Dead letter queue |
| `/settings` | API keys management |

## Development

```bash
npm run dev    # Start dev server on :3000
npm run build  # Production build
npm run test   # Run tests
```
