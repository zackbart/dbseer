# dbseer

Lightweight, browser-based Postgres GUI for development environments. Single static Go binary with an embedded React frontend.

## Architecture

- **Backend:** Go 1.22+, pgx v5, chi v5, stdlib slog
- **Frontend:** React 18, TypeScript (strict), Vite 5, Tailwind CSS v4, shadcn/ui (Base UI)
- **State:** TanStack Query v5 (server), TanStack Table v8 (grid), React Router v6 (routing), URL params (filters/sorts/pagination)

## Development

### Frontend (`web/`)

```bash
cd web
pnpm dev          # Vite dev server on :5173, proxies /api to :4983
pnpm typecheck    # tsc -b --noEmit (strict, noUnusedLocals, noUnusedParameters)
pnpm lint         # eslint --max-warnings 0
pnpm build        # tsc + vite build → ../internal/ui/dist
```

### Build and test the full binary

```bash
cd web && pnpm build && cd .. && go build -o ~/.local/bin/dbseer-dev ./cmd/dbseer
```

Then run `dbseer-dev` to test locally.

### Tailwind v4 + shadcn

- Tailwind v4 uses `@tailwindcss/vite` plugin (in `vite.config.ts`), NOT PostCSS
- No `tailwind.config.js` or `postcss.config.js` — theme is CSS-based via `@theme inline` in `src/styles.css`
- CSS variables map to the color palette: `--primary` = blue-600, `--destructive` = red-600, borders/muted = slate equivalents (oklch)
- shadcn components in `src/components/ui/` — these are generated files, ignored by ESLint
- `tsconfig.json` must have `@/*` path alias for shadcn imports to resolve
- Use semantic color tokens (`text-foreground`, `bg-muted`, `border-border`, `text-primary`, etc.), NOT hardcoded Tailwind colors

### Conventions

- Prettier: double quotes, trailingComma "es5", printWidth 100
- ESLint ignores `src/components/ui/` (generated shadcn code)
- No test runner installed (only `url.test.ts` exists)
- DataGrid's `<table>` is TanStack-managed — don't wrap it with shadcn Table
- Toast uses sonner (`toast.error()`, `toast.success()`), NOT a custom hook
