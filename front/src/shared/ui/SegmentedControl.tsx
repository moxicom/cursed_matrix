import { Chip, type ChipSize, type ChipTone } from './Chip';
import { cn } from '@/shared/lib/cn';

export interface SegmentedOption<T extends string> {
  value: T;
  label: string;
}

export interface SegmentedControlProps<T extends string> {
  options: ReadonlyArray<SegmentedOption<T>>;
  value: T;
  onChange: (value: T) => void;
  tone?: ChipTone;
  size?: ChipSize;
  className?: string;
}

/** Status filters, ranking periods — a row of mutually exclusive chips. */
export function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
  tone = 'green',
  size = 'md',
  className,
}: SegmentedControlProps<T>) {
  return (
    <div className={cn('flex gap-2', className)}>
      {options.map((option) => (
        <Chip
          key={option.value}
          tone={tone}
          size={size}
          active={option.value === value}
          onClick={() => onChange(option.value)}
        >
          {option.label}
        </Chip>
      ))}
    </div>
  );
}
