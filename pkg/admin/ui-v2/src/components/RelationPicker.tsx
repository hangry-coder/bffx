import React, { useEffect, useState } from 'react';

export interface RelationPickerProps {
  adminApiUrl: string;
  targetResource: string;
  value: string;
  onChange: (val: string) => void;
}

export const RelationPicker: React.FC<RelationPickerProps> = ({ adminApiUrl, targetResource, value, onChange }) => {
  const [options, setOptions] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!targetResource) return;
    setLoading(true);
    fetch(`${adminApiUrl}/resources/${encodeURIComponent(targetResource)}?per_page=100`)
      .then((res) => res.json())
      .then((data) => {
        if (Array.isArray(data)) {
          setOptions(data);
        }
      })
      .catch((err) => console.error(`Failed to fetch relation ${targetResource}`, err))
      .finally(() => setLoading(false));
  }, [adminApiUrl, targetResource]);

  return (
    <select
      value={value || ''}
      onChange={(e) => onChange(e.target.value)}
      disabled={loading}
      className="w-full bg-background border border-border-subtle rounded-lg px-4 py-2.5 text-sm text-white focus:border-blue-400 outline-none"
    >
      <option value="">{loading ? 'Loading options...' : 'Select reference...'}</option>
      {options.map((opt: any) => {
        const label = opt.name || opt.title || opt.email || opt.username || opt.key || opt.id;
        return (
          <option key={opt.id} value={opt.id}>
            {label} ({opt.id})
          </option>
        );
      })}
    </select>
  );
};
