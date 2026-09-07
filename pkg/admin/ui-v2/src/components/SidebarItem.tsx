import React from 'react';
import { cn } from '../lib/utils';

export const SidebarItem: React.FC<{
  icon: React.ComponentType<{ size?: number; className?: string }>;
  label: string;
  active?: boolean;
  onClick: () => void;
}> = ({ icon: Icon, label, active = false, onClick }) => (
  <button
    onClick={onClick}
    className={cn(
      'w-full flex items-center gap-3 px-6 py-3 transition-all duration-200 group text-left',
      active
        ? 'text-blue-400 border-l-4 border-blue-400 bg-blue-400/5'
        : 'text-[#8c90a1] hover:bg-surface-card hover:text-text-main',
    )}
  >
    <Icon size={18} className={cn('transition-colors', active ? 'text-blue-400' : 'group-hover:text-white')} />
    <span className="font-sans text-sm font-medium">{label}</span>
  </button>
);
