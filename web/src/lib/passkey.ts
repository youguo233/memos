import { startAuthentication, startRegistration } from "@simplewebauthn/browser";
import { setAccessToken } from "@/auth-state";
import { getRequestToken } from "@/connect";

export interface Passkey {
  id: number;
  name: string;
  createdTs: number;
  lastUsedTs: number;
}

async function call<T>(path: string, init: RequestInit = {}, authed = true): Promise<T> {
  const headers = new Headers(init.headers);
  if (authed) {
    const token = await getRequestToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }
  if (init.body) headers.set("Content-Type", "application/json");
  const response = await fetch(path, { ...init, headers, credentials: "include" });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error((data as { message?: string }).message || response.statusText);
  }
  return data as T;
}

export const passkeySupported = () => typeof window !== "undefined" && !!window.PublicKeyCredential;

export async function listPasskeys(): Promise<Passkey[]> {
  return (await call<{ passkeys: Passkey[] }>("/api/v1/passkeys")).passkeys;
}

export async function deletePasskey(id: number): Promise<void> {
  await call(`/api/v1/passkeys/${id}`, { method: "DELETE" });
}

export async function registerPasskey(name: string): Promise<Passkey> {
  // biome-ignore lint/suspicious/noExplicitAny: server returns WebAuthn options JSON
  const begin = await call<{ sessionId: string; options: { publicKey: any } }>("/api/v1/passkeys/register/begin", { method: "POST" });
  const credential = await startRegistration({ optionsJSON: begin.options.publicKey });
  const query = new URLSearchParams({ sessionId: begin.sessionId, name });
  return call<Passkey>(`/api/v1/passkeys/register/finish?${query}`, { method: "POST", body: JSON.stringify(credential) });
}

export async function signInWithPasskey(): Promise<void> {
  // biome-ignore lint/suspicious/noExplicitAny: server returns WebAuthn options JSON
  const begin = await call<{ sessionId: string; options: { publicKey: any } }>("/api/v1/passkeys/login/begin", { method: "POST" }, false);
  const assertion = await startAuthentication({ optionsJSON: begin.options.publicKey });
  const query = new URLSearchParams({ sessionId: begin.sessionId });
  const result = await call<{ accessToken: string; accessTokenExpireTime: string }>(
    `/api/v1/passkeys/login/finish?${query}`,
    { method: "POST", body: JSON.stringify(assertion) },
    false,
  );
  setAccessToken(result.accessToken, new Date(result.accessTokenExpireTime));
}
