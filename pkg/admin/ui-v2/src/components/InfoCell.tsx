import React from 'react';

export const InfoCell = ({ label, value }: { label: string; value: string }) => (
  <div className="rounded border border-border-subtle bg-surface-card/50 px-3 py-2">
    <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest">{label}</p>
    <p className="text-sm text-text-main font-mono mt-1 break-all">{value}</p>
  </div>
);
