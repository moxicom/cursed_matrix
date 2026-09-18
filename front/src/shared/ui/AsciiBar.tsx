import { asciiBar } from '@/shared/lib/ascii';
import { cn } from '@/shared/lib/cn';

export interface AsciiBarProps {
  /** 0..1 */
  ratio: number;
  /** Number of cells between the brackets. 8 in the header, 16 on the profile. */
  size?: number;
  className?: string;
}

export function AsciiBar({ ratio, size = 8, className }: AsciiBarProps) {
  return (
    <span className={cn('whitespace-nowrap tracking-tight text-txt-ghost', className)}>
      {asciiBar(ratio, size)}
    </span>
  );
}
