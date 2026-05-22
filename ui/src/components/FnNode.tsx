import { Handle, Position, type NodeProps } from "@xyflow/react";
import { actionSummary, dtmfLabel, type FnNodeData } from "../lib/fn-graph";

export default function FnNode({ data, selected }: NodeProps & { data: FnNodeData }) {
  const { fn, isEntry } = data;
  const borderColor = selected
    ? "border-white"
    : isEntry
      ? "border-blue-slate"
      : "border-shadow-grey";
  const ring = isEntry ? "ring-1 ring-blue-slate/60" : "";

  return (
    <div
      className={`bg-gunmetal border-2 ${borderColor} ${ring} rounded font-mono text-xs text-white`}
      style={{ width: 280 }}
    >
      <Handle type="target" position={Position.Left} className="!bg-blue-slate" />
      <div className="px-3 py-2 border-b border-shadow-grey flex items-center justify-between">
        <span className="truncate">{fn.name || "(unnamed)"}</span>
        {isEntry && (
          <span className="text-[10px] text-blue-slate uppercase tracking-wide">entry</span>
        )}
      </div>
      <div className="px-3 py-2 flex flex-col gap-0.5">
        {fn.actions.length === 0 && (
          <span className="text-blue-slate italic">no actions</span>
        )}
        {fn.actions.map((a, i) => (
          <div key={i} className="flex gap-2">
            <span className="text-blue-slate w-4">{dtmfLabel(a.num)}</span>
            <span className="truncate">{actionSummary(a)}</span>
          </div>
        ))}
      </div>
      <Handle type="source" position={Position.Right} className="!bg-blue-slate" />
    </div>
  );
}
