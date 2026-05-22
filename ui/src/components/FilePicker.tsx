import { useEffect, useMemo, useState } from "react";
import { filesApi, type R2Object } from "../lib/files";
import { buildTree, formatBytes, nodeAt, prefixOf } from "../lib/file-tree";

// Modal R2 browser. Two modes:
//   file   — click a file → onPick(key)
//   folder — at any depth, "Select this folder" → onPick(prefixWithoutTrailingSlash)
// Caller decides whether to prepend `files/` for the IVR runtime.

type Props = {
  mode: "file" | "folder";
  onPick: (key: string) => void;
  onClose: () => void;
};

export default function FilePicker({ mode, onPick, onClose }: Props) {
  const [objects, setObjects] = useState<R2Object[] | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [path, setPath] = useState<string[]>([]);

  useEffect(() => {
    filesApi.listAll()
      .then(setObjects)
      .catch((e) => setErr(String(e)));
  }, []);

  const tree = useMemo(() => (objects ? buildTree(objects) : null), [objects]);
  const node = useMemo(() => (tree ? nodeAt(tree, path) : null), [tree, path]);

  function selectFolder() {
    // R2 keys don't include a trailing slash; the caller can decide
    // whether to add one. Empty path means bucket root.
    const p = prefixOf(path);
    onPick(p.replace(/\/$/, ""));
  }

  return (
    <div
      className="fixed inset-0 z-50 bg-ink-black/80 flex items-center justify-center p-6"
      onClick={onClose}
    >
      <div
        className="bg-gunmetal border border-shadow-grey rounded w-full max-w-3xl max-h-[80vh] flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-3 p-3 border-b border-shadow-grey">
          <span className="text-xs text-blue-slate uppercase tracking-wider">
            {mode === "file" ? "Pick a file" : "Pick a folder"}
          </span>
          <Breadcrumb path={path} onJump={setPath} />
          <button
            onClick={onClose}
            className="ml-auto text-xs px-2 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded"
          >
            cancel
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-3">
          {err && <div className="text-blue-slate text-sm">{err}</div>}
          {!objects && !err && (
            <div className="text-blue-slate text-sm">loading…</div>
          )}
          {node && (
            <div className="flex flex-col gap-1 font-mono text-xs">
              {[...node.folders.entries()].map(([name, _sub]) => (
                <button
                  key={"f:" + name}
                  onClick={() => setPath([...path, name])}
                  className="flex items-center gap-2 text-left px-2 py-1 rounded hover:bg-shadow-grey text-white"
                >
                  <span className="text-blue-slate">▸</span>
                  <span>{name}/</span>
                </button>
              ))}
              {node.files.map((o) => {
                const last = o.key.split("/").pop() ?? o.key;
                return (
                  <button
                    key={"o:" + o.key}
                    onClick={() => mode === "file" && onPick(o.key)}
                    disabled={mode === "folder"}
                    className={`flex items-center gap-3 text-left px-2 py-1 rounded ${
                      mode === "file"
                        ? "hover:bg-shadow-grey text-white cursor-pointer"
                        : "text-blue-slate cursor-default opacity-60"
                    }`}
                  >
                    <span className="text-blue-slate w-3">·</span>
                    <span className="flex-1 truncate">{last}</span>
                    <span className="text-blue-slate">{formatBytes(o.size)}</span>
                  </button>
                );
              })}
              {node.folders.size === 0 && node.files.length === 0 && (
                <div className="text-blue-slate italic">empty</div>
              )}
            </div>
          )}
        </div>

        {mode === "folder" && (
          <div className="p-3 border-t border-shadow-grey flex items-center gap-3">
            <span className="text-xs text-blue-slate truncate flex-1">
              selection: <span className="text-white">{path.length ? prefixOf(path).replace(/\/$/, "") : "(bucket root)"}</span>
            </span>
            <button
              onClick={selectFolder}
              className="text-xs px-3 py-1 bg-blue-slate text-white hover:bg-shadow-grey rounded"
            >
              Select this folder
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

function Breadcrumb({
  path,
  onJump,
}: {
  path: string[];
  onJump: (p: string[]) => void;
}) {
  return (
    <div className="flex items-center gap-1 text-xs font-mono text-blue-slate truncate">
      <button
        onClick={() => onJump([])}
        className="hover:text-white"
      >
        /
      </button>
      {path.map((seg, i) => (
        <span key={i} className="flex items-center gap-1">
          <button
            onClick={() => onJump(path.slice(0, i + 1))}
            className="hover:text-white"
          >
            {seg}
          </button>
          {i < path.length - 1 && <span>/</span>}
        </span>
      ))}
    </div>
  );
}
