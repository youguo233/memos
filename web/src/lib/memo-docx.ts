import { getRequestToken } from "@/connect";
import { downloadFileFromUrl } from "@/lib/browser";
import { extractMemoIdFromName } from "@/lib/resource-names";

import { effectiveTimeZone } from "@/lib/timezone";

async function downloadDocx(path: string, params: Record<string, string>, fallbackName: string, tz?: string): Promise<void> {
  const query = new URLSearchParams({ tz: effectiveTimeZone(tz), ...params });
  const token = await getRequestToken();
  const response = await fetch(`${path}?${query.toString()}`, {
    credentials: "include",
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }
  const disposition = response.headers.get("Content-Disposition") ?? "";
  const match = /filename\*=UTF-8''([^;]+)/.exec(disposition);
  const filename = match ? decodeURIComponent(match[1]) : fallbackName;
  const url = URL.createObjectURL(await response.blob());
  downloadFileFromUrl(url, filename);
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// exportMemoDocx downloads one memo as a Word document.
export async function exportMemoDocx(memoName: string, tz?: string): Promise<void> {
  const uid = extractMemoIdFromName(memoName);
  await downloadDocx(`/file/memos/${encodeURIComponent(uid)}/docx`, {}, `memo-${uid}.docx`, tz);
}

// exportAllMemosDocx downloads the caller's memos as one Word document,
// optionally limited to memos created between two local dates (inclusive).
export async function exportAllMemosDocx(startDate?: string, endDate?: string, tz?: string): Promise<void> {
  const params: Record<string, string> = {};
  if (startDate) params.start = String(Math.floor(new Date(`${startDate}T00:00:00`).getTime() / 1000));
  if (endDate) {
    const end = new Date(`${endDate}T00:00:00`);
    end.setDate(end.getDate() + 1);
    params.end = String(Math.floor(end.getTime() / 1000));
  }
  await downloadDocx("/file/users/me/memos.docx", params, "memos.docx", tz);
}

// exportMemoMarkdown downloads one memo as a raw Markdown file.
export async function exportMemoMarkdown(memoName: string, tz?: string): Promise<void> {
  const uid = extractMemoIdFromName(memoName);
  await downloadDocx(`/file/memos/${encodeURIComponent(uid)}/md`, {}, `memo-${uid}.md`, tz);
}
