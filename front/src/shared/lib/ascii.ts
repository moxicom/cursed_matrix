/** `[████░░░░]` progress bar, identical to the design's bar(ratio, size). */
export function asciiBar(ratio: number, size: number): string {
  const filled = Math.max(0, Math.min(size, Math.round(ratio * size)));
  return `[${'█'.repeat(filled)}${'░'.repeat(size - filled)}]`;
}

/** Zero-padded two digit number, used for node codes and ranks. */
export function pad2(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}
