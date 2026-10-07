import { authorizeSensitiveAccess } from "./sensitive";
import type { ChunkUploadTask } from "./chunkUpload";

// The current server exposes a multipart backup endpoint, not chunk/merge APIs.
export function createBackupUploadTask(): ChunkUploadTask {
  let current: XMLHttpRequest | null = null;
  let cancelled = false;
  return {
    cancel() { cancelled = true; current?.abort(); },
    async upload<T>(purpose: string, file: File, onProgress: (value: number) => void): Promise<T | undefined> {
      if (purpose !== "backup" || file.size <= 0 || file.size >= 1024 ** 3) throw new Error("备份必须为小于 1 GiB 的非空 ZIP 文件");
      await authorizeSensitiveAccess();
      if (cancelled) throw new DOMException("Upload cancelled", "AbortError");
      try {
        return await new Promise<T | undefined>((resolve, reject) => {
          const xhr = new XMLHttpRequest(); current = xhr;
          xhr.upload.addEventListener("progress", event => {
            if (event.lengthComputable) onProgress(Math.min(99, Math.round(event.loaded / event.total * 100)));
          });
          xhr.addEventListener("load", () => {
            try {
              const payload = JSON.parse(xhr.responseText);
              if (xhr.status < 200 || xhr.status >= 300 || payload.status !== "success") throw new Error(payload.message || `HTTP ${xhr.status}`);
              onProgress(100); resolve(payload.data);
            } catch (error) { reject(error); }
          });
          xhr.addEventListener("error", () => reject(new Error("备份上传失败，服务端处理状态未知，请先检查服务状态")));
          xhr.addEventListener("abort", () => reject(new DOMException("Upload cancelled", "AbortError")));
          const body = new FormData(); body.append("backup", file, file.name);
          xhr.open("POST", "/api/admin/upload/backup"); xhr.send(body);
        });
      } finally { current = null; }
    },
  };
}
