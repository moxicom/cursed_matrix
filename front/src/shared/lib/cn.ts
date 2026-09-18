type ClassValue = string | number | false | null | undefined;

/** Minimal class joiner — no runtime dependency, keeps className overrides last. */
export function cn(...values: ClassValue[]): string {
  return values.filter((v): v is string | number => v !== false && v != null && v !== '').join(' ');
}
