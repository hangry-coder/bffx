// Centralized label humanization for the admin UI.
//
// Backend manifests use snake_case (`fasting_plan`, `ai_credit_transaction`).
// The admin should render them as `Fasting Plan`, `AI Credit Transaction`.
// A manifest-level `displayName` (when present on the API response) takes
// precedence over the algorithmic transform.

const ACRONYMS = new Set([
  'ai',
  'api',
  'id',
  'jwt',
  'otp',
  'sms',
  'sso',
  'url',
  'uri',
  'ip',
  'ui',
  'os',
  'tz',
]);

/**
 * Convert a snake_case identifier into a human label.
 *
 * - All underscores become spaces.
 * - Each word is title-cased.
 * - Known acronyms (see `ACRONYMS`) are upper-cased so `ai_credit_transaction`
 *   → `AI Credit Transaction` instead of `Ai Credit Transaction`.
 * - When `displayName` is non-empty, it wins.
 */
export function humanizeResourceName(name: string, displayName?: string): string {
  if (displayName && displayName.trim()) return displayName;
  if (!name) return '';
  return name
    .replace(/_/g, ' ')
    .split(/\s+/)
    .filter(Boolean)
    .map((w) =>
      ACRONYMS.has(w.toLowerCase())
        ? w.toUpperCase()
        : w.charAt(0).toUpperCase() + w.slice(1).toLowerCase(),
    )
    .join(' ');
}

/**
 * Same algorithm, but tuned for field names so `created_at` → `Created At`
 * and `user_id` → `User ID`.
 */
export function humanizeFieldName(name: string): string {
  return humanizeResourceName(name);
}
