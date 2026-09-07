import React from 'react';
import {
  Activity,
  Bell,
  BookOpen,
  Component as ComponentIcon,
  Database,
  Flag,
  Layers,
  LayoutDashboard,
  Monitor,
  Settings,
  Users,
  type LucideIcon,
} from 'lucide-react';
import type { AnalyticsProviderInfo } from './lib/dashboardAnalytics';
import { isSidebarPage } from './lib/adminSession';

export type AdminPageId =
  | 'dashboard'
  | 'users'
  | 'feature-flags'
  | 'performance'
  | 'alerts'
  | 'resources'
  | 'app-features'
  | 'app-settings'
  | 'system-settings'
  | 'api-docs'
  | 'bffx-docs';

export type AdminSiteMenuItem = {
  label: string;
  page?: string;
  priority?: number;
  children?: AdminSiteMenuItem[];
};

type AdminNavProps = {
  menu: AdminSiteMenuItem[];
  features?: {
    feature_flags?: boolean;
    api_docs?: boolean;
    app_features?: boolean;
    liveops?: boolean;
  };
  providers: {
    telemetry?: AnalyticsProviderInfo;
    incident?: AnalyticsProviderInfo;
  };
  activePage: AdminPageId;
  selectedCollectionId: string;
  selectedScreenName: string | null;
  selectedClusterName: string | null;
  onNavigate: (page: string, resourceId?: string) => void;
  SidebarItem: React.ComponentType<{
    icon: LucideIcon;
    label: string;
    active: boolean;
    onClick: () => void;
  }>;
};

const pageIcons: Record<string, LucideIcon> = {
  dashboard: LayoutDashboard,
  users: Users,
  'feature-flags': Flag,
  performance: Activity,
  alerts: Bell,
  resources: Layers,
  'app-features': ComponentIcon,
  'app-settings': Monitor,
  'system-settings': Settings,
  'api-docs': BookOpen,
  'bffx-docs': BookOpen,
};

function iconForItem(item: AdminSiteMenuItem): LucideIcon {
  if (item.page?.startsWith('resource:')) {
    return Database;
  }
  if (item.page?.startsWith('screen:') || item.page?.startsWith('feature:')) {
    return ComponentIcon;
  }
  if (item.page && pageIcons[item.page]) {
    return pageIcons[item.page];
  }
  return Layers;
}

function menuItemVisible(
  item: AdminSiteMenuItem,
  features: AdminNavProps['features'],
  providers: AdminNavProps['providers'],
): boolean {
  switch (item.page) {
  case 'feature-flags':
    return features?.feature_flags !== false;
  case 'api-docs':
  case 'bffx-docs':
    return !!features?.api_docs;
  case 'app-features':
    return features?.app_features !== false;
  case 'performance':
    return (
      features?.liveops !== false &&
      !!providers.telemetry?.capabilities?.includes('read.summary')
    );
  case 'alerts':
    return (
      features?.liveops !== false &&
      !!providers.incident?.capabilities?.includes('read.list')
    );
  default:
    return true;
  }
}

function isItemActive(
  item: AdminSiteMenuItem,
  activePage: AdminPageId,
  selectedCollectionId: string,
  selectedScreenName: string | null,
  selectedClusterName: string | null,
): boolean {
  if (item.page?.startsWith('resource:')) {
    const id = item.page.slice('resource:'.length);
    return activePage === 'resources' && selectedCollectionId === id;
  }
  if (item.page?.startsWith('screen:')) {
    const name = item.page.slice('screen:'.length);
    return activePage === 'app-features' && selectedScreenName === name;
  }
  if (item.page?.startsWith('feature:')) {
    const name = item.page.slice('feature:'.length);
    return activePage === 'app-features' && selectedClusterName === name;
  }
  if (item.page === 'resources') {
    return activePage === 'resources';
  }
  if (item.page === 'app-features') {
    return activePage === 'app-features' && !selectedScreenName && !selectedClusterName;
  }
  return item.page === activePage;
}

export function AdminNav({
  menu,
  features,
  providers,
  activePage,
  selectedCollectionId,
  selectedScreenName,
  selectedClusterName,
  onNavigate,
  SidebarItem,
}: AdminNavProps) {
  const renderItems = (items: AdminSiteMenuItem[], depth = 0) =>
    items.map((item) => {
      if (item.page && !isSidebarPage(item.page)) {
        return null;
      }
      if (!menuItemVisible(item, features, providers)) {
        return null;
      }

      const hasChildren = (item.children?.length ?? 0) > 0;
      if (hasChildren) {
        const visibleChildren = (item.children ?? []).filter(
          (c) => isSidebarPage(c.page) && menuItemVisible(c, features, providers),
        );
        if (visibleChildren.length === 0 && !item.page) {
          return null;
        }
        return (
          <React.Fragment key={`${item.label}-${depth}`}>
            <div className={depth === 0 ? 'px-6 py-2 mt-4' : 'px-6 py-1.5 mt-2'}>
              <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest">
                {item.label}
              </p>
            </div>
            {visibleChildren.length > 0
              ? renderItems(visibleChildren, depth + 1)
              : item.page
                ? renderLeaf(item, depth)
                : null}
          </React.Fragment>
        );
      }

      return renderLeaf(item, depth);
    });

  const renderLeaf = (item: AdminSiteMenuItem, depth: number) => {
    const page = item.page ?? '';
    return (
      <SidebarItem
        key={`${item.label}-${page}-${depth}`}
        icon={iconForItem(item)}
        label={item.label}
        active={isItemActive(item, activePage, selectedCollectionId, selectedScreenName, selectedClusterName)}
        onClick={() => {
          if (page.startsWith('resource:')) {
            onNavigate('resources', page.slice('resource:'.length));
            return;
          }
          if (page.startsWith('screen:') || page.startsWith('feature:')) {
            onNavigate(page);
            return;
          }
          if (page) {
            onNavigate(page);
          }
        }}
      />
    );
  };

  if (menu.length === 0) {
    return null;
  }

  return (
    <>
      <div className="px-6 py-2">
        <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest">
          Main
        </p>
      </div>
      {renderItems(menu)}
    </>
  );
}
