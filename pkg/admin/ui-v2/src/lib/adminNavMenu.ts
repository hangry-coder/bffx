import type { AdminSiteMenuItem } from '../AdminNav';

export function menuContainsPage(items: AdminSiteMenuItem[], page: string): boolean {
  for (const item of items) {
    if (item.page === page) {
      return true;
    }
    if (item.children?.length && menuContainsPage(item.children, page)) {
      return true;
    }
  }
  return false;
}
