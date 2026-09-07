import React from 'react';

export const FormField = ({ label, children }: { label: string; children: React.ReactNode }) => (
  <div className="space-y-1.5 flex-1">
    <label className="text-[10px] font-display font-bold text-[#9BA3AF] uppercase tracking-widest">{label}</label>
    {children}
  </div>
);
