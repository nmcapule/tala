import type { UploadedImage } from "./types";

export class ApiError extends Error {
  code?: string;
  field?: string;
  status: number;

  constructor(status: number, message: string, code?: string, field?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.field = field;
  }
}

export async function api(path: string, options: { method?: string; body?: unknown; username?: string } = {}) {
  const res = await fetch(path, {
    method: options.method || "GET",
    headers: { "Content-Type": "application/json", ...(options.username ? { "X-Tala-Username": options.username } : {}) },
    body: options.body === undefined ? undefined : JSON.stringify(options.body)
  });
  const data = await res.json();
  if (!res.ok) throw new ApiError(res.status, data.error?.message || "Request failed.", data.error?.code, data.error?.field);
  return data;
}

export async function uploadImage(file: File, username: string): Promise<UploadedImage> {
  const form = new FormData();
  form.append("image", file);
  const res = await fetch("/api/uploads/images", {
    method: "POST",
    headers: username ? { "X-Tala-Username": username } : undefined,
    body: form
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error?.message || "Unable to upload image.");
  return data;
}
