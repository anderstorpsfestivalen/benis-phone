import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { QueueNodeData } from "../lib/fn-graph";

export default function QueueNode({ data }: NodeProps & { data: QueueNodeData }) {
  const { name, queue } = data;
  return (
    <div
      className="bg-shadow-grey border-2 border-blue-slate/60 rounded font-mono text-xs text-white px-3 py-2"
      style={{ width: 280 }}
    >
      <Handle type="target" position={Position.Left} className="!bg-blue-slate" />
      <div className="flex items-center justify-between">
        <span className="truncate">{name}</span>
        <span className="text-[10px] text-blue-slate uppercase tracking-wide">
          {queue ? "queue" : "missing"}
        </span>
      </div>
    </div>
  );
}
