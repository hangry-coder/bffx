import { describe, expect, it } from 'vitest';
import {
  analyticsPlaceholderCopy,
  shouldShowAnalyticsPlaceholder,
  usesBuiltInAnalytics,
} from './dashboardAnalytics';

describe('usesBuiltInAnalytics', () => {
  it('treats missing and battery as built-in', () => {
    expect(usesBuiltInAnalytics(undefined)).toBe(true);
    expect(usesBuiltInAnalytics({})).toBe(true);
    expect(usesBuiltInAnalytics({ name: 'battery' })).toBe(true);
  });

  it('treats external providers as non-built-in', () => {
    expect(usesBuiltInAnalytics({ name: 'posthog' })).toBe(false);
    expect(usesBuiltInAnalytics({ name: 'segment' })).toBe(false);
  });
});

describe('shouldShowAnalyticsPlaceholder', () => {
  it('always uses placeholder until real regional/push APIs exist', () => {
    expect(shouldShowAnalyticsPlaceholder()).toBe(true);
    expect(shouldShowAnalyticsPlaceholder({ name: 'posthog' })).toBe(true);
  });
});

describe('analyticsPlaceholderCopy', () => {
  it('returns configure copy for battery regional widget', () => {
    const copy = analyticsPlaceholderCopy({ name: 'battery' }, 'regional');
    expect(copy.title).toBe('Regional Metrics Offline');
    expect(copy.body).toContain('PostHog');
    expect(copy.vendorUrl).toBeUndefined();
  });

  it('returns configure copy for battery push widget', () => {
    const copy = analyticsPlaceholderCopy({ name: 'battery' }, 'push');
    expect(copy.title).toBe('Push Metrics Offline');
    expect(copy.body).toContain('Firebase');
  });

  it('returns vendor link copy for external analytics', () => {
    const copy = analyticsPlaceholderCopy(
      {
        name: 'posthog',
        display_name: 'PostHog',
        vendor_url: 'https://app.posthog.com/project/abc',
      },
      'regional',
    );
    expect(copy.title).toBe('Regional metrics in provider');
    expect(copy.body).toContain('PostHog');
    expect(copy.vendorUrl).toBe('https://app.posthog.com/project/abc');
    expect(copy.vendorLabel).toBe('Open PostHog');
  });
});
