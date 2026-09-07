/** Provider slice returned by GET /api/admin/providers (analytics key). */
export type AnalyticsProviderInfo = {
  name?: string;
  display_name?: string;
  vendor_url?: string;
};

export type AnalyticsWidget = 'regional' | 'push';

export type AnalyticsPlaceholderCopy = {
  title: string;
  body: string;
  vendorUrl?: string;
  vendorLabel?: string;
};

/** Built-in local analytics — no third-party regional/push dashboards. */
export function usesBuiltInAnalytics(analytics?: AnalyticsProviderInfo): boolean {
  const name = analytics?.name?.trim();
  return !name || name === 'battery';
}

/**
 * Dashboard regional/push widgets have no wired data source yet.
 * Never show hardcoded demo charts (gap-closure Phase 2).
 */
export function shouldShowAnalyticsPlaceholder(_analytics?: AnalyticsProviderInfo): boolean {
  return true;
}

export function analyticsPlaceholderCopy(
  analytics: AnalyticsProviderInfo | undefined,
  widget: AnalyticsWidget,
): AnalyticsPlaceholderCopy {
  if (usesBuiltInAnalytics(analytics)) {
    if (widget === 'regional') {
      return {
        title: 'Regional Metrics Offline',
        body: 'Configure PostHog or an external analytics provider to map regional node ingress.',
      };
    }
    return {
      title: 'Push Metrics Offline',
      body: 'Configure a push notification adapter (e.g. Firebase Cloud Messaging) to track push telemetry.',
    };
  }

  const label = analytics?.display_name?.trim() || analytics?.name?.trim() || 'your analytics provider';
  const vendorUrl = analytics?.vendor_url?.trim() || undefined;

  if (widget === 'regional') {
    return {
      title: 'Regional metrics in provider',
      body: `Node distribution is not surfaced in the admin dashboard. Open ${label} for regional analytics.`,
      vendorUrl,
      vendorLabel: vendorUrl ? `Open ${label}` : undefined,
    };
  }

  return {
    title: 'Push metrics in provider',
    body: `Push propagation is not surfaced in the admin dashboard. View delivery stats in ${label} or your push provider.`,
    vendorUrl,
    vendorLabel: vendorUrl ? `Open ${label}` : undefined,
  };
}
