type RelationLinkProps = {
  targetResource: string;
  recordId: string;
  label?: string;
  onOpen: (resource: string, id: string) => void;
};

export function RelationLink({ targetResource, recordId, label, onOpen }: RelationLinkProps) {
  if (!recordId) {
    return <span className="text-[#8c90a1]">—</span>;
  }
  const text = label && label !== recordId ? label : `#${recordId}`;
  return (
    <button
      type="button"
      onClick={() => onOpen(targetResource, String(recordId))}
      className="text-blue-400 hover:text-blue-300 font-mono text-xs underline-offset-2 hover:underline"
    >
      {text}
    </button>
  );
}
