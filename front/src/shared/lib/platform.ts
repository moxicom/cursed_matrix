/** macOS reports "MacIntel"/"MacARM" in platform and "Mac" in the UA. */
export function isApplePlatform(): boolean {
  if (typeof navigator === 'undefined') return false;
  return /Mac|iPhone|iPad|iPod/.test(navigator.platform || navigator.userAgent);
}

/** Search hotkey label: ⌘K on Apple hardware, CTRL+K everywhere else. */
export function shortcutLabel(): string {
  return isApplePlatform() ? '⌘K' : 'CTRL+K';
}
