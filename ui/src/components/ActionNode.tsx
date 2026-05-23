import { Handle, Position, type NodeProps } from "@xyflow/react";
import { actionDetail, categoryFor, type ActionNodeData } from "../lib/fn-graph";

// Action nodes are color-coded by category so flow is readable at a glance:
//   route    → blue-slate (sends caller somewhere)
//   speak    → white outline (TTS prompt)
//   media    → dashed shadow-grey (file / livefeed)
//   service  → solid double-border (external service)
//   control  → faded shadow-grey (hangup / clear / transfer / record / dtmf)

export default function ActionNode({ data, selected }: NodeProps & { data: ActionNodeData }) {
  const { action, actionKind } = data;
  const cat = categoryFor(actionKind);
  const detail = actionDetail(action);

  const palette = paletteFor(cat);

  return (
    <div
      className={`${palette.bg} ${palette.border} ${selected ? "ring-2 ring-white" : ""} rounded font-mono text-xs text-white flex flex-col cursor-pointer`}
      style={{ width: 240 }}
    >
      <Handle type="target" position={Position.Left} className="!bg-blue-slate" isConnectable={false} />
      <div className={`px-2 py-1 flex items-center justify-between ${palette.headerBg}`}>
        <span className={`text-[10px] uppercase tracking-wider ${palette.headerText}`}>
          {actionKind ?? "empty"}
        </span>
        <span className={`text-[10px] uppercase tracking-wider ${palette.tag}`}>{cat}</span>
      </div>
      {/* One body row: the name when set (it summarizes the action),
          otherwise the kind-specific detail. Keeping a single row means
          dagre's hardcoded ACTION_NODE_HEIGHT stays accurate so nodes
          don't overlap their neighbors. The non-displayed value is
          still in the tooltip for at-a-glance inspection. */}
      {action.name ? (
        <div
          className="px-2 py-2 text-sm font-semibold truncate"
          title={detail ? `${action.name} — ${detail}` : action.name}
        >
          {action.name}
        </div>
      ) : detail ? (
        <div className="px-2 py-2 truncate" title={detail}>
          {detail}
        </div>
      ) : (
        <div className="px-2 py-2 italic text-blue-slate">no detail</div>
      )}
      <Handle type="source" position={Position.Right} className="!bg-blue-slate" isConnectable={false} />
    </div>
  );
}

type Palette = {
  bg: string;
  border: string;
  headerBg: string;
  headerText: string;
  tag: string;
};

function paletteFor(cat: ReturnType<typeof categoryFor>): Palette {
  switch (cat) {
    case "route":
      return {
        bg: "bg-gunmetal",
        border: "border-2 border-blue-slate",
        headerBg: "bg-blue-slate/30",
        headerText: "text-white",
        tag: "text-blue-slate",
      };
    case "speak":
      return {
        bg: "bg-gunmetal",
        border: "border-2 border-white/70",
        headerBg: "bg-white/10",
        headerText: "text-white",
        tag: "text-blue-slate",
      };
    case "service":
      return {
        bg: "bg-gunmetal",
        border: "border-4 border-double border-blue-slate",
        headerBg: "bg-blue-slate/20",
        headerText: "text-white",
        tag: "text-blue-slate",
      };
    case "media":
      return {
        bg: "bg-gunmetal",
        border: "border-2 border-dashed border-shadow-grey",
        headerBg: "bg-shadow-grey/60",
        headerText: "text-blue-slate",
        tag: "text-blue-slate/70",
      };
    case "control":
    default:
      return {
        bg: "bg-shadow-grey/60",
        border: "border border-shadow-grey",
        headerBg: "bg-shadow-grey",
        headerText: "text-blue-slate",
        tag: "text-blue-slate/70",
      };
  }
}
