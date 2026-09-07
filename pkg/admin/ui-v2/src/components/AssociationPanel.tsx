import { useEffect, useState } from 'react';
import { adminFetch } from '../lib/adminSession';

type AssociationPanelProps = {
  adminApiUrl: string;
  parentResource: string;
  parentId: string;
  name: string;
  label: string;
  childResource: string;
};

export function AssociationPanel({
  adminApiUrl,
  parentResource,
  parentId,
  name,
  label,
  childResource,
}: AssociationPanelProps) {
  const [rows, setRows] = useState<Record<string, unknown>[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    const url = `${adminApiUrl}/resources/${encodeURIComponent(parentResource)}/${encodeURIComponent(parentId)}/associations/${encodeURIComponent(name)}`;
    adminFetch(url)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: { rows?: Record<string, unknown>[] }) => {
        setRows(Array.isArray(data?.rows) ? data.rows : []);
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false));
  }, [adminApiUrl, parentResource, parentId, name]);

  return (
    <div className="mt-4 border border-border-subtle rounded-lg overflow-hidden">
      <div className="px-4 py-2 bg-surface-dim border-b border-border-subtle flex justify-between items-center">
        <p className="text-[10px] font-display font-bold text-[#8c90a1] uppercase tracking-widest">{label}</p>
        <span className="text-[10px] text-[#424655] font-mono">{childResource}</span>
      </div>
      {loading && <p className="p-4 text-xs text-[#8c90a1]">Loading…</p>}
      {error && <p className="p-4 text-xs text-red-300">{error}</p>}
      {!loading && !error && rows.length === 0 && (
        <p className="p-4 text-xs text-[#424655] italic">No related records.</p>
      )}
      {!loading && !error && rows.length > 0 && (
        <ul className="divide-y divide-border-subtle max-h-48 overflow-y-auto">
          {rows.map((row) => (
            <li key={String(row.id)} className="px-4 py-2 text-xs text-white font-mono">
              #{String(row.id)}
              {row.status != null && (
                <span className="ml-2 text-[#8c90a1]">{String(row.status)}</span>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
