// API client for /api/files/* (R2-backed file manager). All routes are
// behind Cloudflare Access — fetch sends cookies automatically.

export type R2Object = {
  key: string;
  size: number;
  uploaded: string; // ISO timestamp
  etag: string;
  contentType: string | null;
};

export type ListResponse = {
  objects: R2Object[];
  cursor: string | null;
  truncated: boolean;
};

export const filesApi = {
  async list(prefix?: string, cursor?: string): Promise<ListResponse> {
    const url = new URL("/api/files", window.location.origin);
    if (prefix) url.searchParams.set("prefix", prefix);
    if (cursor) url.searchParams.set("cursor", cursor);
    const r = await fetch(url.toString(), { credentials: "include" });
    if (!r.ok) throw new Error(`list failed: ${r.status} ${await r.text()}`);
    return r.json();
  },

  /**
   * List every object across as many pages as it takes. Fine for our
   * bucket (<1000 objects); we'd add pagination UI before this hurts.
   */
  async listAll(prefix?: string): Promise<R2Object[]> {
    const all: R2Object[] = [];
    let cursor: string | undefined;
    do {
      const page = await this.list(prefix, cursor);
      all.push(...page.objects);
      cursor = page.cursor ?? undefined;
      if (!page.truncated) break;
    } while (cursor);
    return all;
  },

  async remove(key: string): Promise<void> {
    const url = new URL("/api/files/object", window.location.origin);
    url.searchParams.set("key", key);
    const r = await fetch(url.toString(), {
      method: "DELETE",
      credentials: "include",
    });
    if (!r.ok && r.status !== 204) {
      throw new Error(`delete failed: ${r.status} ${await r.text()}`);
    }
  },

  /**
   * Upload via XHR so the caller can report progress. fetch() doesn't
   * expose upload progress in browsers as of writing.
   */
  upload(
    key: string,
    file: File,
    onProgress?: (pct: number) => void,
  ): Promise<R2Object> {
    return new Promise((resolve, reject) => {
      const url = new URL("/api/files/object", window.location.origin);
      url.searchParams.set("key", key);
      const xhr = new XMLHttpRequest();
      xhr.open("PUT", url.toString());
      xhr.withCredentials = true;
      xhr.setRequestHeader(
        "Content-Type",
        file.type || "application/octet-stream",
      );
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable && onProgress) {
          onProgress(e.loaded / e.total);
        }
      };
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            resolve(JSON.parse(xhr.responseText));
          } catch (err) {
            reject(err);
          }
        } else {
          reject(new Error(`upload failed: ${xhr.status} ${xhr.responseText}`));
        }
      };
      xhr.onerror = () => reject(new Error("upload network error"));
      xhr.send(file);
    });
  },

  objectURL(key: string): string {
    const url = new URL("/api/files/object", window.location.origin);
    url.searchParams.set("key", key);
    return url.toString();
  },
};
