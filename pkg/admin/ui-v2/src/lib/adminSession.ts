/** Pages reached from the header chrome, not the left sidebar. */
export const HEADER_ONLY_PAGES = new Set(['alerts', 'system-settings']);

export function adminFetch(url: string, init?: RequestInit): Promise<Response> {
  return fetch(url, { credentials: 'same-origin', ...init });
}

/** Only 401 means the HttpOnly session cookie is gone; 403 is a permission error on one route. */
export function clearAuthIfUnauthorized(
  res: Response,
  setAuthenticated: (value: boolean) => void,
): void {
  if (res.status === 401) {
    setAuthenticated(false);
  }
}

export function isSidebarPage(page?: string): boolean {
  if (!page) {
    return true;
  }
  return !HEADER_ONLY_PAGES.has(page);
}
