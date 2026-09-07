import React from 'react';
import { TrendingUp, TrendingDown } from 'lucide-react';
import { motion } from 'motion/react';
import { cn } from '../lib/utils';

export const StatCard = ({
  title,
  value,
  trend,
  trendValue,
  icon: Icon,
}: {
  title: string;
  value: string;
  trend: 'up' | 'down';
  trendValue: string;
  icon: React.ComponentType<{ size?: number; className?: string }>;
}) => (
  <motion.div
    initial={{ opacity: 0, y: 20 }}
    whileInView={{ opacity: 1, y: 0 }}
    viewport={{ once: true }}
    className="bg-surface-card border border-border-subtle rounded-xl p-6 relative overflow-hidden group hover:border-[#8c90a1] transition-colors"
  >
    <div className="flex justify-between items-start mb-4">
      <span className="text-[11px] font-display font-bold uppercase tracking-wider text-[#9BA3AF]">{title}</span>
      <Icon size={18} className="text-[#8c90a1]" />
    </div>
    <div className="text-3xl font-display font-semibold text-text-main">{value}</div>
    <div className="mt-4 flex items-center gap-2">
      {trend === 'up' ? (
        <TrendingUp size={14} className="text-[#9cf132]" />
      ) : (
        <TrendingDown size={14} className="text-[#ffb4ab]" />
      )}
      <span className={cn('text-xs font-mono font-medium', trend === 'up' ? 'text-[#9cf132]' : 'text-[#ffb4ab]')}>
        {trendValue}
      </span>
    </div>
  </motion.div>
);
