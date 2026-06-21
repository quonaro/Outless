# Outless Frontend

Nuxt 4 frontend for Outless — a VLESS node management system.

This is the admin UI for managing node groups, VLESS nodes, and access tokens. It communicates with the Outless backend API.

**Architecture:** SPA with TanStack Query for server state, Pinia for client state, Zod for validation.

---

## Quick Start

```bash
# 1. Install dependencies
pnpm install

# 2. Configure environment
cp .env.example .env
# Edit: API_BASE_URL=http://localhost:41220

# 3. Run development server (port 41221)
pnpm dev
```

The app will be available at `http://localhost:41221`.

---

## Stack

- **Framework**: Nuxt 4
- **Language**: TypeScript (strict mode)
- **UI**: Shadcn-vue
- **Styling**: Tailwind CSS
- **Server State**: TanStack Query v5
- **Client State**: Pinia
- **Validation**: Zod + Vee-Validate
- **Fetching**: ofetch
- **Icons**: Lucide-vue-next
- **Package Manager**: pnpm

## Installation

```bash
pnpm install
```

## Development

```bash
pnpm dev
```

## Build

```bash
pnpm build
```

## Type Check

```bash
pnpm typecheck
```

## Architecture

See `frontend.md` for detailed architecture rules and standards.

### Directory Structure

```
app/                          # Nuxt 4 assets
  components/
    ui/                       # Shadcn atomic components
  composables/                # Global composables
  layouts/                    # Page layouts
  pages/                      # File-based routing
  middleware/                 # Route guards
  plugins/                    # Nuxt plugins

features/
  [feature-name]/
    components/               # Feature-specific components
    composables/              # Feature-scoped composables
    schemas/                  # Zod schemas
    services/                 # API calls

stores/                       # Pinia stores (client state only)
types/                        # Shared TypeScript types
```

### Key Rules

- **Schema-First**: All API data must have Zod schemas before component logic
- **Zero-CSS**: No `<style>` blocks, no custom CSS
- **TanStack Query**: Server state lives in query cache, not Pinia
- **File Size Limits**: Components < 250 lines, composables < 200 lines

---

## Deployment

### Docker

```bash
# Build image
docker build -t outless-frontend .

# Run
docker run -p 3000:3000 -e API_BASE_URL=http://backend:8080 outless-frontend
```

### Static Hosting

```bash
# Generate static site
pnpm generate

# Output in .output/public/
```

---

## Contributing

See `CONTRIBUTING.md` for detailed contribution rules.
