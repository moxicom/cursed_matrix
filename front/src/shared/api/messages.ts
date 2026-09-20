import type { Dictionary } from '@/shared/i18n';

import { ApiError } from './client';

/**
 * Turns a refusal into a sentence in the reader's language.
 *
 * The server sends a code and parameters and never prose, so this is the only
 * place that decides what a failure says — and it says it in whichever
 * language is being read, without the server knowing which one that is.
 */
export function apiMessage(error: unknown, t: Dictionary): string {
  // Not an answer from the server at all: the request never arrived.
  if (!(error instanceof ApiError)) return t.errOffline;
  return messageForCode(error.code, error.params, t);
}

export function messageForCode(
  code: string,
  params: Record<string, unknown>,
  t: Dictionary,
): string {
  switch (code) {
    case 'REQUEST_FAILED':
      return t.errOffline;
    case 'INVALID_CREDENTIALS':
      return t.errInvalidCredentials;
    case 'SESSION_EXPIRED':
    case 'CSRF_TOKEN_INVALID':
      return t.errSessionExpired;
    case 'RATE_LIMITED': {
      const wait = num(params.retryAfterSeconds);
      return wait === null
        ? t.errRateLimited
        : `${t.errRateLimited} (${String(wait)}s)`;
    }
    case 'QUOTA_LIMIT_REACHED':
      return t.errQuota.replace(
        '{limit}',
        num(params.limit)?.toString() ?? '—',
      );
    case 'SUBSCRIPTION_REQUIRED':
      return t.errSubscription;
    case 'BILLING_UNAVAILABLE':
      return t.errBilling;
    case 'TASK_NOT_FOUND':
      return t.errNotFound;
    case 'TASK_ALREADY_COMPLETED':
      return t.errAlreadyDone;
    case 'COMPLETED_TASK_FROZEN':
      return t.errFrozen;
    case 'HAS_ACTIVE_SUBTASKS':
      return t.errHasSubtasks;
    case 'NESTING_NOT_ALLOWED':
      return t.errNesting;
    case 'NOT_A_SUBTASK':
      return t.errNotSubtask;
    case 'SELF_LINK':
      return t.errSelfLink;
    case 'DUPLICATE_LINK':
      return t.errDuplicateLink;
    case 'ORPHANED_NODE':
      return t.errOrphaned;
    case 'VALIDATION_FAILED':
      return validation(params, t);
    default:
      return t.errInternal;
  }
}

/** Names the field that was wrong, because 'check the fields' helps nobody. */
function validation(params: Record<string, unknown>, t: Dictionary): string {
  const field = typeof params.field === 'string' ? params.field : null;
  if (field === null) return t.errValidation;

  const max = num(params.max);
  if (max !== null)
    return t.errTooLong.replace('{field}', field).replace('{max}', String(max));
  return t.errField.replace('{field}', field);
}

function num(value: unknown): number | null {
  return typeof value === 'number' ? value : null;
}
