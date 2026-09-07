import React, { useState } from 'react';

const EXPIRY_OPTIONS = [
  { id: '15m', label: '15 min' },
  { id: '1h', label: '1 hour' },
  { id: '', label: 'forever' },
];

export const KillSwitchControl: React.FC<{
  screenName: string;
  current?: { enabled: boolean; reason?: string; expires_at?: string };
  compact?: boolean;
  onToggle: (enabled: boolean, reason: string, expiresIn: string) => Promise<unknown>;
}> = ({ current, compact = false, onToggle }) => {
  const [reason, setReason] = useState<string>(current?.reason || '');
  const [expiresIn, setExpiresIn] = useState<string>('15m');
  const [pending, setPending] = useState<boolean>(false);
  const [err, setErr] = useState<string | null>(null);

  const active = !!current?.enabled;

  const submit = (next: boolean) => {
    if (next && !reason.trim()) {
      setErr('reason is required when killing a screen');
      return;
    }
    setErr(null);
    setPending(true);
    onToggle(next, reason, next ? expiresIn : '')
      .catch((e) => setErr(e?.message || String(e)))
      .finally(() => setPending(false));
  };

  return (
    <div className={compact ? 'text-xs' : 'text-sm'}>
      <div className="flex flex-wrap items-center gap-2">
        <span
          className={`px-2 py-0.5 rounded text-[10px] uppercase font-bold tracking-wider ${active ? 'bg-red-500/20 text-red-300' : 'bg-green-500/20 text-green-300'}`}
        >
          {active ? 'KILLED' : 'live'}
        </span>
        <input
          type="text"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          placeholder="reason (required to kill)"
          className="flex-1 min-w-[180px] bg-background border border-border-subtle rounded px-2 py-1 text-xs text-text-main outline-none focus:border-blue-400"
        />
        {!active && (
          <select
            value={expiresIn}
            onChange={(e) => setExpiresIn(e.target.value)}
            className="bg-background border border-border-subtle rounded px-2 py-1 text-xs text-text-main outline-none focus:border-blue-400"
          >
            {EXPIRY_OPTIONS.map((o) => (
              <option key={o.id} value={o.id}>
                {o.label}
              </option>
            ))}
          </select>
        )}
        {active ? (
          <button
            onClick={() => submit(false)}
            disabled={pending}
            className="px-3 py-1 rounded text-xs font-semibold bg-green-500 hover:bg-green-400 text-white disabled:opacity-60"
          >
            {pending ? '…' : 'Restore'}
          </button>
        ) : (
          <button
            onClick={() => submit(true)}
            disabled={pending}
            className="px-3 py-1 rounded text-xs font-semibold bg-red-500 hover:bg-red-400 text-white disabled:opacity-60"
          >
            {pending ? '…' : 'Kill'}
          </button>
        )}
      </div>
      {current?.expires_at && active && (
        <p className="text-[10px] text-text-muted mt-1">Auto-reverts at {new Date(current.expires_at).toLocaleString()}</p>
      )}
      {err && <p className="text-[10px] text-red-300 mt-1">{err}</p>}
    </div>
  );
};
