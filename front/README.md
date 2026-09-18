# front — cursed_matrix (React)

Frontend of the `cursed_matrix` application. Design source:
`../design/Quest Terminal.dc.html`.

## Stack

React 18 · TypeScript (strict) · Vite 5 · Tailwind CSS 3 · React Router 6 · Zustand

## Commands

```bash
npm run dev        # dev server on http://localhost:5173
npm run build      # typecheck + production build
npm run preview    # preview the production build
npm run typecheck  # types only
npm run lint       # eslint: typescript-eslint (type-aware), react-hooks, jsx-a11y
```

`npm run lint` is expected to stay at zero problems. The two deliberate
`eslint-disable` lines are on the task card and subtask row: clicking the card
body is a mouse shortcut, and the keyboard path is the title button inside it,
so the card must not become a second tab stop.

## Path aliases

`@/*` → `src/*`, plus the explicit ones: `@/app`, `@/pages`, `@/widgets`,
`@/features`, `@/entities`, `@/shared`. They are declared in both
`vite.config.ts` (bundling) and `tsconfig.app.json` (types).

## Design tokens

All tokens live in `tailwind.config.ts` and come straight from the design. Two
deviations from stock Tailwind are worth knowing about:

1. **`spacing` is a 1:1 pixel scale.** `p-14` is 14px, `gap-8` is 8px, `mt-7` is
   7px. The design uses odd values (3, 5, 7, 9, 11, 13) that Tailwind's rem
   scale cannot express without an arbitrary value on every class.
2. **`fontSize` keys are pixel values without the dot.** `text-95` is 9.5px,
   `text-105` is 10.5px, `text-115` is 11.5px. Fractional sizes are mandatory
   here.

Radii are zero across the design (`rounded` = 0). The only rounded shape is the
graph legend dot (`rounded-full`).

Color groups: `bg-*` (surfaces), `line-*` (borders), `txt-*` (text),
`green/violet/amber/red/cyan` (accents), `quad-q1..q4` (quadrants), `task-*`
(user-assignable task colors), `heat-0..4` + `heat-bd-0..4` (heatmap), `node-*`
(graph nodes), `overlay-*` (modal backdrops).

The `wordmark/hint/xpbar/navkey` breakpoints (1000/1100/1180/1280px) mirror the
`winW` checks from the design — the layout is desktop-first.

## Structure

```
src/
├── app/        # root component, providers, router, layout, access guard
├── pages/      # screens (landing, board, graph, activity, leaderboard,
│               #          profile, settings, pricing, 404)
├── widgets/    # header, sidebars, layout shells
├── features/   # user-facing behaviour (filters, search, task modal, paywall,
│               #                        graph simulation, stores)
├── entities/   # domain components (TaskCard, SubtaskRow, PlanCard, …)
└── shared/
    ├── ui/     # reusable primitives
    ├── lib/    # helpers (cn, ascii bars, deadlines, progression)
    ├── config/ # domain constants (XP, quadrants, plan limits, routes)
    ├── i18n/   # EN/RU dictionaries and the language store
    ├── types/  # types shared with the backend contract
    └── mocks/  # mock data typed with those contracts
```

## Data

Every screen currently runs on mocks from `src/shared/mocks`, typed with the
interfaces in `src/shared/types/domain.ts`. Those are the shapes the Go backend
in `back/` is expected to return, so swapping mocks for HTTP calls requires no
changes in the components.

## Localization

The UI ships in English and Russian. `src/shared/i18n/en.ts` defines the key set
and `ru.ts` must implement exactly the same keys — the dictionaries are typed
against each other. Enum values, event codes and achievement codes are
language-independent and are never translated.

## npm registry

`front/.npmrc` pins the public npm registry. This is deliberate: it keeps
`package-lock.json` free of internal mirror URLs when the project is built
inside a corporate network.
