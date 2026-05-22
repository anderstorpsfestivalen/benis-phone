import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { FnNodeData } from "../lib/fn-graph";

export default function FnNode({ data, selected }: NodeProps & { data: FnNodeData }) {
  const { fn, isEntry } = data;
  const border = selected
    ? "border-white"
    : isEntry
      ? "border-blue-slate"
      : "border-shadow-grey";
  const ring = isEntry ? "ring-2 ring-blue-slate/40" : "";

  return (
    <div
      className={`bg-ink-black border-2 ${border} ${ring} rounded-lg font-mono text-sm text-white px-4 py-3 flex items-center gap-3 cursor-pointer`}
      style={{ width: 220 }}
    >
      <Handle type="target" position={Position.Left} className="!bg-blue-slate" isConnectable={false} />
      <span className="text-blue-slate text-[10px] uppercase tracking-wider">menu</span>
      <span className="truncate flex-1 text-right">{fn.name || "(unnamed)"}</span>
      {isEntry && (
        <span className="text-[10px] text-blue-slate uppercase tracking-wider">entry</span>
      )}
      <Handle type="source" position={Position.Right} className="!bg-blue-slate" isConnectable={false} />
    </div>
  );
}
