import React from 'react';
import {
  analyticsPlaceholderCopy,
  type AnalyticsProviderInfo,
  type AnalyticsWidget,
} from '../lib/dashboardAnalytics';

export const AnalyticsWidgetPlaceholder: React.FC<{
  widget: AnalyticsWidget;
  analytics?: AnalyticsProviderInfo;
  icon: React.ComponentType<{ size?: number; className?: string }>;
}> = ({ widget, analytics, icon: Icon }) => {
  const copy = analyticsPlaceholderCopy(analytics, widget);
  return (
    <div className="flex-1 flex flex-col items-center justify-center text-center p-4">
      <Icon size={32} className="text-[#424655] mb-3 animate-pulse" />
      <p className="text-xs font-semibold text-text-main">{copy.title}</p>
      <p className="text-[10px] text-[#8c90a1] mt-2 max-w-[240px] mx-auto leading-relaxed">{copy.body}</p>
      {copy.vendorUrl && copy.vendorLabel ? (
        <a
          href={copy.vendorUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="mt-4 text-[10px] font-bold uppercase tracking-widest text-blue-400 hover:text-blue-300"
        >
          {copy.vendorLabel}
        </a>
      ) : null}
    </div>
  );
};
