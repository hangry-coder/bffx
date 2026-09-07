import { useEffect, useState } from 'react';
import { adminFetch } from './lib/adminSession';

type GuideEntry = { id: string; title: string; path: string };

type GuideDoc = { path: string; title: string; markdown: string };

function renderMarkdown(md: string) {
  const lines = md.split('\n');
  return lines.map((line, i) => {
    const trimmed = line.trimEnd();
    if (trimmed.startsWith('### ')) {
      return (
        <h3 key={i} className="text-base font-semibold text-text-main mt-4 mb-2">
          {trimmed.slice(4)}
        </h3>
      );
    }
    if (trimmed.startsWith('## ')) {
      return (
        <h2 key={i} className="text-lg font-semibold text-text-main mt-6 mb-2">
          {trimmed.slice(3)}
        </h2>
      );
    }
    if (trimmed.startsWith('# ')) {
      return (
        <h1 key={i} className="text-2xl font-display font-bold text-text-main mt-2 mb-3">
          {trimmed.slice(2)}
        </h1>
      );
    }
    if (trimmed.startsWith('- ')) {
      return (
        <li key={i} className="text-sm text-[#9BA3AF] ml-4 list-disc">
          {trimmed.slice(2)}
        </li>
      );
    }
    if (trimmed.startsWith('```')) {
      return (
        <pre
          key={i}
          className="text-xs font-mono bg-surface-dim border border-border-subtle rounded-lg p-3 overflow-x-auto my-2 text-[#c8cdd8]"
        >
          {trimmed.replace(/^```\w*/, '').replace(/```$/, '')}
        </pre>
      );
    }
    if (trimmed === '') {
      return <div key={i} className="h-2" />;
    }
    return (
      <p key={i} className="text-sm text-[#9BA3AF] leading-relaxed my-1">
        {trimmed}
      </p>
    );
  });
}

type BffxDocsPanelProps = {
  adminApiUrl: string;
};

export function BffxDocsPanel({ adminApiUrl }: BffxDocsPanelProps) {
  const [entries, setEntries] = useState<GuideEntry[]>([]);
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [doc, setDoc] = useState<GuideDoc | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    adminFetch(`${adminApiUrl}/docs/guide/index.json`)
      .then((res) => (res.ok ? res.json() : { entries: [] }))
      .then((data: { entries?: GuideEntry[] }) => {
        const list = Array.isArray(data?.entries) ? data.entries : [];
        setEntries(list);
        if (list.length > 0 && !selectedPath) {
          setSelectedPath(list[0].path);
        }
      })
      .catch(() => setError('Failed to load guide index'));
  }, [adminApiUrl]);

  useEffect(() => {
    if (!selectedPath) {
      setDoc(null);
      return;
    }
    setError(null);
    adminFetch(`${adminApiUrl}/docs/guide/${encodeURI(selectedPath)}`)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: GuideDoc) => setDoc(data))
      .catch(() => {
        setDoc(null);
        setError('Failed to load document');
      });
  }, [adminApiUrl, selectedPath]);

  return (
    <div className="flex gap-6 flex-1 min-h-[520px] mt-4">
      <div className="w-64 shrink-0 border-r border-border-subtle pr-2 overflow-y-auto">
        <p className="text-[10px] font-display font-bold text-[#424655] uppercase tracking-widest px-2 mb-3">
          BFFX documentation
        </p>
        {entries.length === 0 && (
          <p className="text-xs text-text-muted px-2">
            No guide bundled. Run <span className="font-mono text-blue-400">bffx update framework --vendor-only</span>{' '}
            to copy docs into <span className="font-mono">.bffx/docs/</span>.
          </p>
        )}
        <ul className="space-y-0.5">
          {entries.map((e) => (
            <li key={e.id}>
              <button
                type="button"
                onClick={() => setSelectedPath(e.path)}
                className={`w-full text-left px-3 py-2 rounded text-sm truncate ${
                  selectedPath === e.path
                    ? 'bg-blue-400/10 text-blue-400'
                    : 'text-text-muted hover:bg-surface-card/60'
                }`}
              >
                {e.title}
              </button>
            </li>
          ))}
        </ul>
      </div>
      <div className="flex-1 overflow-y-auto min-w-0 pr-4">
        {error && (
          <p className="text-sm text-red-300 border border-red-500/30 rounded-lg px-4 py-3">{error}</p>
        )}
        {doc && <article className="max-w-3xl">{renderMarkdown(doc.markdown)}</article>}
      </div>
    </div>
  );
}
