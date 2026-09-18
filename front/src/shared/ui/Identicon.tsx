import { hash } from '@/shared/lib/hash';
import { cn } from '@/shared/lib/cn';

export interface IdenticonProps {
  /** Username the pattern is derived from. */
  name: string;
  /** Cell color for "on" bits. */
  accent?: string;
  /** Cell size in px — 3 in the header and leaderboard, 13 on the profile. */
  cell?: number;
  className?: string;
}

/** 4×4 deterministic avatar, same bit layout as the design. */
export function Identicon({ name, accent = '#3f7a55', cell = 3, className }: IdenticonProps) {
  const h = hash(name);
  const cells = Array.from({ length: 16 }, (_, i) => ((h >> (i % 30)) & 1) === 1);

  return (
    <span
      aria-hidden="true"
      className={cn('grid flex-none', className)}
      style={{
        gridTemplateColumns: `repeat(4, ${cell}px)`,
        gridAutoRows: `${cell}px`,
        gap: cell > 6 ? 2 : 1,
      }}
    >
      {cells.map((on, i) => (
        <span key={i} style={{ background: on ? accent : '#1c2023' }} />
      ))}
    </span>
  );
}
