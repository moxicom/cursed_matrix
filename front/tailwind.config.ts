import type { Config } from 'tailwindcss';

/**
 * Design tokens extracted from design/Quest Terminal.dc.html.
 *
 * Two deliberate deviations from stock Tailwind:
 *  - spacing is a 1:1 px scale (p-14 === 14px), because the source design is
 *    px-precise and uses odd values (3, 5, 7, 9, 11, 13) throughout;
 *  - fontSize keys are px values without the dot (text-95 === 9.5px).
 */

const pxScale = (max: number): Record<string, string> =>
  Object.fromEntries(Array.from({ length: max + 1 }, (_, i) => [i, `${i}px`]));

const spacing: Record<string, string> = {
  ...pxScale(96),
  px: '1px',
  120: '120px',
  180: '180px',
  186: '186px',
  190: '190px',
  210: '210px',
  238: '238px',
  260: '260px',
  266: '266px',
  272: '272px',
  360: '360px',
  420: '420px',
};

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    // radii are zero across the whole design — only the graph legend dot is round
    borderRadius: { none: '0', DEFAULT: '0', full: '9999px' },
    spacing,
    fontSize: {
      85: ['8.5px', '1.4'],
      9: ['9px', '1.4'],
      95: ['9.5px', '1.45'],
      10: ['10px', '1.45'],
      105: ['10.5px', '1.45'],
      11: ['11px', '1.45'],
      115: ['11.5px', '1.5'],
      12: ['12px', '1.55'],
      125: ['12.5px', '1.5'],
      13: ['13px', '1.5'],
      135: ['13.5px', '1.65'],
      14: ['14px', '1.4'],
      15: ['15px', '1.3'],
      16: ['16px', '1.3'],
      20: ['20px', '1.2'],
      30: ['30px', '1.1'],
      46: ['46px', '1.05'],
    },
    letterSpacing: {
      tight: '-0.06em', // ascii progress bars
      snug: '-0.01em', // landing h1
      base: '0.01em', // body default
      t4: '0.04em',
      t5: '0.05em',
      t6: '0.06em',
      t7: '0.07em',
      t8: '0.08em',
      t9: '0.09em',
      t10: '0.1em',
    },
    extend: {
      fontFamily: {
        mono: ['"JetBrains Mono"', 'ui-monospace', 'Menlo', 'Consolas', 'monospace'],
        sans: ['"JetBrains Mono"', 'ui-monospace', 'Menlo', 'Consolas', 'monospace'],
      },
      colors: {
        // surfaces
        bg: {
          base: '#0a0b0c', // app background, graph canvas
          alt: '#0b0d0e', // landing feature cell, modal side column
          sunken: '#0c0e0f', // board toolbar, graph sidebar
          raised: '#0d0f10', // header, modal shell, ascii board
          panel: '#0e1112', // sections, column header, hover card, toast
          input: '#101214', // inputs, subtask cards, stat cells
          hover: '#14171a', // hover / pressed surface
          mine: '#101512', // "you" row in the leaderboard
        },
        line: {
          faint: '#131718', // list row separator
          subtle: '#1c2023', // inner dividers, grid-gap fill
          DEFAULT: '#23272b', // standard border
          strong: '#2e343a', // modal, emphasised buttons
          hover: '#3d4347',
        },
        txt: {
          bright: '#e6eaec',
          DEFAULT: '#d8dde0',
          soft: '#c5cbcf',
          body: '#b6bec3',
          mute: '#9aa4a9',
          dim: '#8b9599',
          tag: '#7d878c',
          faint: '#79838a',
          label: '#6f797e',
          ghost: '#6a7479',
          branch: '#58616a',
          ph: '#4e565a',
        },
        // accents
        green: {
          DEFAULT: '#5fb37f',
          light: '#7fc89b',
          bright: '#6cbf8b',
          bg: '#14201c',
          'bg-hover': '#18271f',
          border: '#3a5c46',
          'border-dim': '#2f4a3a',
          'border-soft': '#2c4536',
          id: '#3f7a55',
        },
        violet: {
          DEFAULT: '#9b8fd0',
          dim: '#8d84ad',
          sub: '#8279a6',
          light: '#b6abe4',
          pale: '#d6cffb',
          bg: '#23204a',
          'bg-on': '#171526',
          border: '#4c4487',
          'border-on': '#3f3a63',
        },
        amber: {
          DEFAULT: '#d2a04a',
          dim: '#c8973c',
          deadline: '#d8a94f',
          border: '#453a24',
          'border-soft': '#38332a',
          'border-alt': '#4a3f28',
        },
        red: {
          DEFAULT: '#d2685a',
          light: '#e0776a',
          border: '#4a3330',
        },
        cyan: {
          DEFAULT: '#6fa8c0',
          dim: '#4f93ad',
          light: '#9ecadb',
          chip: '#8fbdd0', // active tag / quadrant / period chip text
          bg: '#141b20',
          border: '#3a5260',
        },
        // eisenhower quadrant accents (QC in the source)
        quad: {
          q1: '#d2685a',
          q2: '#c8973c',
          q3: '#6fa8c0',
          q4: '#7d878c',
        },
        // user-assignable task colors (COLORS in the source)
        task: {
          none: '#23272b',
          cyan: '#4f93ad',
          violet: '#8f7fc4',
          amber: '#c8973c',
          rose: '#c05b6a',
          teal: '#3f9e8f',
          slate: '#6b7780',
        },
        // activity heatmap scale (SHADES / BD in the source)
        heat: {
          0: '#121517',
          1: '#1c3226',
          2: '#2a5238',
          3: '#3d7a4e',
          4: '#5fb37f',
        },
        'heat-bd': {
          0: '#1a1e21',
          1: '#24392c',
          2: '#33603f',
          3: '#478857',
          4: '#6cbf8b',
        },
        // graph node fills
        node: {
          active: '#1c2226',
          done: '#191d20',
          'done-border': '#2b3236',
        },
        overlay: {
          search: 'rgba(6,7,8,0.72)',
          modal: 'rgba(6,7,8,0.74)',
          pay: 'rgba(6,7,8,0.78)',
        },
      },
      maxWidth: {
        landing: '1020px',
        activity: '1180px',
        upgrade: '1080px',
        page: '880px',
        settings: '720px',
        prose: '760px',
      },
      zIndex: {
        search: '40',
        'search-panel': '41',
        modal: '50',
        'modal-panel': '51',
        pay: '60',
        'pay-panel': '61',
        toast: '60',
      },
      screens: {
        // desktop-first breakpoints lifted from the design's winW checks
        wordmark: '1000px',
        hint: '1100px',
        xpbar: '1180px',
        navkey: '1280px',
      },
    },
  },
  plugins: [],
} satisfies Config;
