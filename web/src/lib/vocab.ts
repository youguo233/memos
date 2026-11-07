import { getRequestToken } from "@/connect";

/** Runtime marker that survives minification. */
export const VOCAB_MARKER = "mbe-vocab-v1";

export type VocabState = 0 | 1 | 2 | 3;

export interface VocabPreview {
  dueTs: number;
  scheduledDays: number;
}

export interface VocabLog {
  rating: number;
  state: number;
  dueTs: number;
  reviewTs: number;
}

export interface VocabCard {
  uid: string;
  term: string;
  phonetic: string;
  definition: string;
  example: string;
  notes: string;
  tags: string[];
  state: VocabState;
  dueTs: number;
  reps: number;
  lapses: number;
  lastReviewTs: number;
  createdTs: number;
  updatedTs: number;
  previews?: Record<string, VocabPreview>;
  logs?: VocabLog[];
}

export interface VocabStats {
  marker?: string;
  total: number;
  new: number;
  learning: number;
  review: number;
  due: number;
}

export interface VocabCardInput {
  term: string;
  phonetic?: string;
  definition: string;
  example?: string;
  notes?: string;
  tags?: string[];
}

export class VocabExistsError extends Error {
  uid: string;
  constructor(uid: string, message: string) {
    super(message);
    this.uid = uid;
  }
}

async function call<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  const token = await getRequestToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  if (init.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  const response = await fetch(path, { ...init, headers, credentials: "include" });
  if (path.endsWith("/export") && response.ok) {
    return (await response.blob()) as T;
  }
  const data = await response.json().catch(() => ({}));
  if (response.status === 409 && (data as { uid?: string }).uid) {
    throw new VocabExistsError((data as { uid: string }).uid, (data as { message?: string }).message || "exists");
  }
  if (!response.ok) {
    throw new Error((data as { message?: string }).message || response.statusText);
  }
  return data as T;
}

export async function getVocabStats(): Promise<VocabStats> {
  return call<VocabStats>("/api/v1/vocab/stats");
}

export async function listVocabCards(params: {
  q?: string;
  filter?: string;
  sort?: string;
  limit?: number;
  offset?: number;
}): Promise<VocabCard[]> {
  const search = new URLSearchParams();
  if (params.q) search.set("q", params.q);
  if (params.filter) search.set("filter", params.filter);
  if (params.sort) search.set("sort", params.sort);
  if (params.limit) search.set("limit", String(params.limit));
  if (params.offset) search.set("offset", String(params.offset));
  const query = search.toString();
  return (await call<{ cards: VocabCard[] }>(`/api/v1/vocab/cards${query ? `?${query}` : ""}`)).cards ?? [];
}

export async function getVocabSession(): Promise<VocabCard[]> {
  return (await call<{ cards: VocabCard[] }>("/api/v1/vocab/session")).cards ?? [];
}

export async function getVocabCard(uid: string): Promise<VocabCard> {
  return call<VocabCard>(`/api/v1/vocab/cards/${uid}`);
}

export async function createVocabCard(input: VocabCardInput): Promise<VocabCard> {
  return call<VocabCard>("/api/v1/vocab/cards", { method: "POST", body: JSON.stringify(input) });
}

export async function updateVocabCard(uid: string, input: VocabCardInput): Promise<VocabCard> {
  return call<VocabCard>(`/api/v1/vocab/cards/${uid}`, { method: "PATCH", body: JSON.stringify(input) });
}

export async function deleteVocabCard(uid: string): Promise<void> {
  await call(`/api/v1/vocab/cards/${uid}`, { method: "DELETE" });
}

export async function reviewVocabCard(uid: string, rating: number): Promise<VocabCard> {
  return call<VocabCard>(`/api/v1/vocab/cards/${uid}/review`, { method: "POST", body: JSON.stringify({ rating }) });
}

export async function exportVocabCsv(): Promise<Blob> {
  return call<Blob>("/api/v1/vocab/export");
}

export async function importVocabCsv(text: string): Promise<number> {
  return (await call<{ imported: number }>("/api/v1/vocab/import", { method: "POST", body: text, headers: { "Content-Type": "text/csv" } }))
    .imported;
}

export function notifyVocabStatsChanged() {
  window.dispatchEvent(new Event("vocab-stats-changed"));
}

export interface DictionaryLookup {
  phonetic: string;
  definition: string;
  example: string;
}

export async function lookupDictionary(term: string): Promise<DictionaryLookup | null> {
  const response = await fetch(`https://api.dictionaryapi.dev/api/v2/entries/en/${encodeURIComponent(term.trim())}`);
  if (!response.ok) return null;
  const data = (await response.json()) as Array<{
    phonetic?: string;
    phonetics?: Array<{ text?: string }>;
    meanings?: Array<{ definitions?: Array<{ definition?: string; example?: string }> }>;
  }>;
  const entry = data?.[0];
  if (!entry) return null;
  const phonetic = entry.phonetic || entry.phonetics?.find((item) => item.text)?.text || "";
  const definition = entry.meanings?.[0]?.definitions?.[0]?.definition || "";
  const example = entry.meanings?.flatMap((meaning) => meaning.definitions ?? []).find((item) => item.example)?.example || "";
  if (!definition && !phonetic) return null;
  return { phonetic, definition, example };
}

export function formatInterval(dueTs: number, now = Date.now()): string {
  const sec = Math.max(0, dueTs - Math.floor(now / 1000));
  if (sec < 60) return "<1m";
  if (sec < 3600) return `${Math.round(sec / 60)}m`;
  if (sec < 86400) return `${Math.round(sec / 3600)}h`;
  const days = Math.round(sec / 86400);
  if (days < 30) return `${days}d`;
  if (days < 365) return `${Math.round(days / 30)}mo`;
  return `${(days / 365).toFixed(1)}y`;
}
