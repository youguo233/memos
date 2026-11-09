import { getRequestToken } from "@/connect";

/** Runtime marker that survives minification. */
export const VOCAB_MARKER = "mbe-vocab-v10";

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

export interface VocabLookup {
  phonetic: string;
  definition: string;
  example: string;
  notes: string;
  audioUrl?: string;
  source: "llm" | "dictionary";
}

export interface VocabSettings {
  enabled: boolean;
  baseUrl: string;
  model: string;
  prompt: string;
  apiKeyMasked: string;
  configured: boolean;
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

const STATS_CACHE_MS = 1500;

let statsInflight: Promise<VocabStats> | undefined;
let statsCache: { value: VocabStats; at: number } | undefined;
let sessionInflight: Promise<VocabCard[]> | undefined;

export async function getVocabStats(options?: { fresh?: boolean }): Promise<VocabStats> {
  if (options?.fresh) {
    statsCache = undefined;
  } else if (statsCache && Date.now() - statsCache.at < STATS_CACHE_MS) {
    return statsCache.value;
  }
  if (!statsInflight) {
    statsInflight = call<VocabStats>("/api/v1/vocab/stats")
      .then((value) => {
        statsCache = { value, at: Date.now() };
        return value;
      })
      .finally(() => {
        statsInflight = undefined;
      });
  }
  return statsInflight;
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
  if (!sessionInflight) {
    sessionInflight = call<{ cards: VocabCard[] }>("/api/v1/vocab/session")
      .then((data) => data.cards ?? [])
      .finally(() => {
        sessionInflight = undefined;
      });
  }
  return sessionInflight;
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

export async function lookupVocab(term: string): Promise<VocabLookup> {
  return call<VocabLookup>("/api/v1/vocab/lookup", { method: "POST", body: JSON.stringify({ term }) });
}

export async function getVocabSettings(): Promise<VocabSettings> {
  return call<VocabSettings>("/api/v1/vocab/settings");
}

export async function saveVocabSettings(input: {
  enabled: boolean;
  baseUrl: string;
  model: string;
  prompt: string;
  apiKey?: string;
  clearApiKey?: boolean;
}): Promise<VocabSettings> {
  return call<VocabSettings>("/api/v1/vocab/settings", { method: "PUT", body: JSON.stringify(input) });
}

export function notifyVocabStatsChanged(stats?: VocabStats) {
  if (stats) {
    statsCache = { value: stats, at: Date.now() };
  } else {
    statsCache = undefined;
  }
  window.dispatchEvent(new CustomEvent<VocabStats | undefined>("vocab-stats-changed", { detail: stats }));
}

export type VocabAccent = "us" | "uk";

const ACCENT_KEY = "memos-vocab-accent";

export function getVocabAccent(): VocabAccent {
  try {
    return localStorage.getItem(ACCENT_KEY) === "uk" ? "uk" : "us";
  } catch {
    return "us";
  }
}

export function setVocabAccent(accent: VocabAccent) {
  try {
    localStorage.setItem(ACCENT_KEY, accent);
  } catch {
    /* ignore */
  }
}

function youdaoURL(term: string, type: 1 | 2): string {
  return `https://dict.youdao.com/dictvoice?audio=${encodeURIComponent(term.trim())}&type=${type}`;
}

/** Youdao dictvoice: type=1 is British (e.g. leverage /ˈliːvərɪdʒ/), type=2 is American. */
function youdaoType(accent: VocabAccent): 1 | 2 {
  return accent === "uk" ? 1 : 2;
}

let currentAudio: HTMLAudioElement | undefined;

function stopPlayback() {
  if (currentAudio) {
    currentAudio.onended = null;
    currentAudio.onerror = null;
    currentAudio.pause();
    currentAudio.src = "";
    currentAudio = undefined;
  }
  if (typeof speechSynthesis !== "undefined") {
    speechSynthesis.cancel();
  }
}

function speakTerm(term: string, accent: VocabAccent, done: () => void) {
  if (typeof speechSynthesis === "undefined") {
    done();
    return;
  }
  const utterance = new SpeechSynthesisUtterance(term);
  utterance.lang = accent === "uk" ? "en-GB" : "en-US";
  const prefix = utterance.lang.toLowerCase();
  const voice = speechSynthesis.getVoices().find((item) => item.lang.replace("_", "-").toLowerCase().startsWith(prefix));
  if (voice) utterance.voice = voice;
  utterance.onend = done;
  utterance.onerror = () => done();
  speechSynthesis.speak(utterance);
}

function playSource(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const audio = new Audio(src);
    currentAudio = audio;
    audio.onended = () => {
      if (currentAudio === audio) currentAudio = undefined;
      resolve();
    };
    audio.onerror = () => {
      if (currentAudio === audio) currentAudio = undefined;
      reject(new Error("audio error"));
    };
    void audio.play().catch((error) => {
      if (currentAudio === audio) currentAudio = undefined;
      reject(error);
    });
  });
}

export async function playPronunciation(term: string, accent: VocabAccent = getVocabAccent(), audioUrl?: string): Promise<void> {
  const trimmed = term.trim();
  if (!trimmed) return;
  stopPlayback();
  const primary = youdaoType(accent);
  const secondary = youdaoType(accent === "uk" ? "us" : "uk");
  const sources = [audioUrl, youdaoURL(trimmed, primary), youdaoURL(trimmed, secondary)].filter((item): item is string => Boolean(item));
  for (const src of sources) {
    try {
      await playSource(src);
      return;
    } catch {
      /* try the next source */
    }
  }
  await new Promise<void>((resolve) => speakTerm(trimmed, accent, resolve));
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
