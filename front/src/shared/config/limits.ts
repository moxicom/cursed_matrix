/**
 * Input length limits, enforced in the UI and mirrored by the backend.
 *
 * Titles are one line on a card, so 100 characters is about as much as can be
 * read at a glance. Descriptions hold context, links and acceptance criteria,
 * which comfortably fits in 2000. Tags are chips in a filter row, so they stay
 * short enough to never dominate the toolbar.
 */
export const TITLE_MAX_LENGTH = 100;
export const DESCRIPTION_MAX_LENGTH = 2000;
export const TAG_MAX_LENGTH = 24;

/** Show the counter only once the field is close to its limit. */
export const COUNTER_THRESHOLD = 0.8;

export function counterFor(value: string, max: number): string | undefined {
  return value.length >= max * COUNTER_THRESHOLD ? `${value.length}/${max}` : undefined;
}

export function atLimit(value: string, max: number): boolean {
  return value.length >= max;
}
