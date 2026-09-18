import { Chip } from './Chip';

export interface ToggleProps {
  checked: boolean;
  onChange: (next: boolean) => void;
  labelOn: string;
  labelOff: string;
  className?: string;
  disabled?: boolean;
}

/** ENABLED / DISABLED switch — rendered as a single stateful chip, like the design. */
export function Toggle({ checked, onChange, labelOn, labelOff, className, disabled }: ToggleProps) {
  return (
    <Chip
      tone="green"
      size="md"
      active={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={className}
    >
      {checked ? labelOn : labelOff}
    </Chip>
  );
}
