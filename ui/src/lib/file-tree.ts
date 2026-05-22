// Pure helpers that turn a flat list of R2 keys (with "/" separators) into
// a directory-style tree the UI can render. R2 has no real directories —
// the tree is purely a UI affordance derived from key shape.

import type { R2Object } from "./files";

export type FileTree = {
  // Map of folder name → subtree.
  folders: Map<string, FileTree>;
  // Files immediately at this level.
  files: R2Object[];
};

export function buildTree(objects: R2Object[]): FileTree {
  const root: FileTree = { folders: new Map(), files: [] };
  for (const obj of objects) {
    const parts = obj.key.split("/");
    let node = root;
    for (let i = 0; i < parts.length - 1; i++) {
      const seg = parts[i];
      if (!seg) continue;
      let child = node.folders.get(seg);
      if (!child) {
        child = { folders: new Map(), files: [] };
        node.folders.set(seg, child);
      }
      node = child;
    }
    node.files.push(obj);
  }
  return root;
}

/**
 * Walk the tree to the node at `path` (an array of folder segments) or
 * return null if a segment doesn't exist.
 */
export function nodeAt(tree: FileTree, path: string[]): FileTree | null {
  let node: FileTree = tree;
  for (const seg of path) {
    const next = node.folders.get(seg);
    if (!next) return null;
    node = next;
  }
  return node;
}

/** Join path segments into an R2 key prefix (with trailing slash). */
export function prefixOf(path: string[]): string {
  return path.length ? path.join("/") + "/" : "";
}

/** Pretty-print bytes for the file list. */
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}
