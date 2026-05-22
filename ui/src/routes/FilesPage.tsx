import { useCallback, useEffect, useMemo, useState } from "react";
import { useDropzone } from "react-dropzone";
import { filesApi, type R2Object } from "../lib/files";
import { buildTree, formatBytes, nodeAt, prefixOf } from "../lib/file-tree";

// /files — full R2 file manager. Drag-drop upload (react-dropzone),
// tree navigation, copy-key / download / delete actions. The same
// listing logic powers FilePicker.tsx.

type Pending = { name: string; pct: number; err?: string };

export default function FilesPage() {
  const [objects, setObjects] = useState<R2Object[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [path, setPath] = useState<string[]>([]);
  const [pending, setPending] = useState<Map<string, Pending>>(new Map());
  const [busy, setBusy] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      setObjects(await filesApi.listAll());
      setErr(null);
    } catch (e) {
      setErr(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const tree = useMemo(() => buildTree(objects), [objects]);
  const node = useMemo(() => nodeAt(tree, path), [tree, path]);
  const currentPrefix = prefixOf(path);

  const onDrop = useCallback(
    (accepted: File[]) => {
      for (const file of accepted) {
        const key = currentPrefix + file.name;
        setPending((m) => new Map(m).set(key, { name: file.name, pct: 0 }));
        filesApi
          .upload(key, file, (pct) => {
            setPending((m) => {
              const next = new Map(m);
              const cur = next.get(key);
              if (cur) next.set(key, { ...cur, pct });
              return next;
            });
          })
          .then(() => {
            setPending((m) => {
              const next = new Map(m);
              next.delete(key);
              return next;
            });
            // Refresh listing once everything we kicked off settles. Cheap
            // enough — listAll is one round-trip.
            reload();
          })
          .catch((e) => {
            setPending((m) => {
              const next = new Map(m);
              const cur = next.get(key);
              if (cur) next.set(key, { ...cur, err: String(e) });
              return next;
            });
          });
      }
    },
    [currentPrefix, reload],
  );

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    noClick: true,
  });

  async function remove(key: string) {
    if (!confirm(`Delete ${key}? This can't be undone.`)) return;
    setBusy(key);
    try {
      await filesApi.remove(key);
      await reload();
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="max-w-5xl mx-auto" {...getRootProps()}>
      <input {...getInputProps()} />

      <div className="flex items-center gap-3 mb-3">
        <h1 className="font-mono text-white text-sm">files</h1>
        <Breadcrumb path={path} onJump={setPath} />
        <button
          onClick={reload}
          disabled={loading}
          className="ml-auto text-xs px-3 py-1 border border-shadow-grey text-blue-slate hover:text-white rounded disabled:opacity-50"
        >
          {loading ? "loading…" : "refresh"}
        </button>
      </div>

      {err && <div className="text-blue-slate text-sm mb-3">{err}</div>}

      <div
        className={`border-2 border-dashed rounded p-2 min-h-[60vh] ${
          isDragActive ? "border-white bg-shadow-grey/40" : "border-shadow-grey"
        }`}
      >
        <div className="text-xs text-blue-slate mb-2">
          Drop files here to upload to <span className="text-white font-mono">{currentPrefix || "(bucket root)"}</span>
        </div>

        {node && (
          <div className="flex flex-col gap-1 font-mono text-xs">
            {[...node.folders.entries()].map(([name]) => (
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
                <div
                  key={"o:" + o.key}
                  className="flex items-center gap-3 px-2 py-1 rounded hover:bg-shadow-grey/40 text-white"
                >
                  <span className="text-blue-slate w-3">·</span>
                  <span className="flex-1 truncate" title={o.key}>{last}</span>
                  <span className="text-blue-slate">{formatBytes(o.size)}</span>
                  <button
                    onClick={() => navigator.clipboard.writeText(o.key)}
                    className="text-blue-slate hover:text-white text-[10px] uppercase tracking-wider px-2 py-0.5 border border-shadow-grey rounded"
                    title="Copy key"
                  >
                    copy
                  </button>
                  <a
                    href={filesApi.objectURL(o.key)}
                    download={last}
                    className="text-blue-slate hover:text-white text-[10px] uppercase tracking-wider px-2 py-0.5 border border-shadow-grey rounded"
                  >
                    get
                  </a>
                  <button
                    onClick={() => remove(o.key)}
                    disabled={busy === o.key}
                    className="text-blue-slate hover:text-white text-[10px] uppercase tracking-wider px-2 py-0.5 border border-shadow-grey rounded disabled:opacity-50"
                  >
                    {busy === o.key ? "…" : "del"}
                  </button>
                </div>
              );
            })}
            {node.folders.size === 0 && node.files.length === 0 && !pending.size && (
              <div className="text-blue-slate italic px-2 py-1">empty</div>
            )}
          </div>
        )}

        {pending.size > 0 && (
          <div className="mt-3 pt-3 border-t border-shadow-grey">
            <div className="text-xs text-blue-slate uppercase tracking-wider mb-1">uploading</div>
            {[...pending.entries()].map(([key, p]) => (
              <div key={key} className="text-xs font-mono py-0.5 flex gap-3">
                <span className="text-white flex-1 truncate">{p.name}</span>
                <span className="text-blue-slate w-12 text-right">
                  {p.err ? "fail" : `${Math.round(p.pct * 100)}%`}
                </span>
              </div>
            ))}
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
      <button onClick={() => onJump([])} className="hover:text-white">/</button>
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
