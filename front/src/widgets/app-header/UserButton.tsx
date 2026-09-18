import { cn } from '@/shared/lib/cn';
import { Identicon } from '@/shared/ui';

export interface UserButtonProps {
  username: string;
  active?: boolean;
  onClick: () => void;
}

export function UserButton({ username, active = false, onClick }: UserButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'flex items-center gap-8 border-0 px-14 text-txt transition-colors hover:bg-bg-hover',
        active ? 'bg-bg-hover' : 'bg-transparent',
      )}
    >
      <Identicon name={username} cell={3} />
      <span className="text-11">{username}</span>
    </button>
  );
}
