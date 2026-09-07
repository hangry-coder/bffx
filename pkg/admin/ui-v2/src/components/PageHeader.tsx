import React from 'react';

export const PageHeader = ({
  title,
  description,
  actions,
}: {
  title: string;
  description: string;
  actions?: React.ReactNode;
}) => (
  <div className="flex justify-between items-end mb-8">
    <div>
      <h2 className="font-display text-4xl font-bold text-text-main tracking-tight">{title}</h2>
      <p className="font-sans text-sm text-[#9BA3AF] mt-1">{description}</p>
    </div>
    <div className="flex gap-3">{actions}</div>
  </div>
);
