import { cn } from '@/shared/lib/cn';

export interface StatCardProps {
  label: string;
  value: string | number;
  note?: string;
  /** Hex or tailwind-free color for the value, e.g. green for completions. */
  valueColor?: string;
  size?: 'sm' | 'md';
  className?: string;
}

export function StatCard({ label, value, note, valueColor, size = 'md', className }: StatCardProps) {
  return (
    <div
      className={cn(
        size === 'md' ? 'bg-bg-panel px-14 py-13' : 'bg-bg-input px-11 py-9',
        className,
      )}
    >
      <div className={cn('tracking-t8 text-txt-faint', size === 'md' ? 'text-95' : 'text-9')}>
        {label}
      </div>
      <div
        className={cn('font-bold text-txt', size === 'md' ? 'mt-7 text-20' : 'mt-4 text-14')}
        style={valueColor === undefined ? undefined : { color: valueColor }}
      >
        {value}
      </div>
      {note !== undefined && <div className="mt-4 text-95 text-txt-label">{note}</div>}
    </div>
  );
}
