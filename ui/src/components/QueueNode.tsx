import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { QueueNodeData } from "../lib/fn-graph";

// Visual twin of FnNode but with a "queue" label and a 2nd line showing the
// position-window summary. Same dimensions / chrome / handle treatment so
// queues feel like first-class siblings of menus in the graph.

export default function QueueNode({ data, selected }: NodeProps & { data: QueueNodeData }) {
  const { name, queue } = data;
  const broken = !queue;
  const border = selected
    ? "border-white"
    : broken
      ? "border-red-400"
      : "border-blue-slate";
  const ring = !broken && !selected ? "ring-2 ring-blue-slate/40" : "";

  return (
    <div
      className={`bg-ink-black border-2 ${border} ${ring} rounded-lg font-mono text-sm text-white px-4 py-3 flex flex-col gap-1 cursor-pointer`}
      style={{ width: 220 }}
    >
      <Handle type="target" position={Position.Left} className="!bg-blue-slate" isConnectable={false} />
      <div className="flex items-center gap-3">
        <span className="text-blue-slate text-[10px] uppercase tracking-wider">
          {broken ? "missing" : "queue"}
        </span>
        <span className="truncate flex-1 text-right">{name}</span>
      </div>
      {queue && (
        <div className="text-[10px] text-blue-slate text-right">
          pos {queue.minpos}–{queue.maxpos} · speed {queue.speed}s · {queue.prompt.length} prompts
        </div>
      )}
      <Handle type="source" position={Position.Right} className="!bg-blue-slate" isConnectable={false} />
    </div>
  );
}
