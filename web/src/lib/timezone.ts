import dayjs, { type ConfigType, type Dayjs } from "dayjs";
import timezonePlugin from "dayjs/plugin/timezone";
import utcPlugin from "dayjs/plugin/utc";
import { useSyncExternalStore } from "react";

const OriginalDateTimeFormat = Intl.DateTimeFormat;

const FALLBACK_ZONES = [
  "UTC",
  "Africa/Cairo",
  "America/Chicago",
  "America/Los_Angeles",
  "America/New_York",
  "America/Sao_Paulo",
  "Asia/Hong_Kong",
  "Asia/Kolkata",
  "Asia/Seoul",
  "Asia/Shanghai",
  "Asia/Singapore",
  "Asia/Taipei",
  "Asia/Tokyo",
  "Australia/Sydney",
  "Europe/Berlin",
  "Europe/London",
  "Europe/Paris",
];

/** The device's IANA time zone. */
export const browserTimeZone = (): string => new OriginalDateTimeFormat().resolvedOptions().timeZone || "UTC";

/** All IANA zones the browser knows, with a short fallback list. */
export function listTimeZones(): string[] {
  const intl = Intl as unknown as { supportedValuesOf?: (key: string) => string[] };
  let zones: string[] = [];
  try {
    zones = intl.supportedValuesOf?.("timeZone") ?? [];
  } catch {
    zones = [];
  }
  const set = new Set(zones.length ? zones : FALLBACK_ZONES);
  set.add("UTC");
  set.add(browserTimeZone());
  return [...set].sort();
}

/** The zone used for exports: the user's custom zone if set, else the device zone. */
export const effectiveTimeZone = (custom?: string): string => custom || browserTimeZone();

// ---------------------------------------------------------------------------
// UI time zone (mbe-tz-ui-v1): the user's preferred zone for every time shown in the web UI.
// "System timezone" (empty) leaves the browser default untouched. Storage stays UTC.

dayjs.extend(utcPlugin);
dayjs.extend(timezonePlugin);

export const TZ_UI_MARKER = "mbe-tz-ui-v2";
/** Request header carrying the viewer's zone for server-side filter date functions. */
export const TIMEZONE_HEADER = "X-Memos-Timezone";

let customZone = "";
let version = 0;
const listeners = new Set<() => void>();

const isValidZone = (zone: string): boolean => {
  try {
    new OriginalDateTimeFormat("en-US", { timeZone: zone });
    return true;
  } catch {
    return false;
  }
};

/** The custom zone, or "" when the user follows the system zone. */
export const getCustomTimeZone = (): string => customZone;
/** The zone the UI shows times in. */
export const getUserTimezone = (): string => customZone || browserTimeZone();

export function setUserTimeZone(zone: string | undefined): void {
  const next = zone && isValidZone(zone) ? zone : "";
  if (typeof document !== "undefined") document.documentElement.dataset.tzUi = `${TZ_UI_MARKER}:${next || "system"}`;
  if (next === customZone) return;
  customZone = next;
  version++;
  for (const l of listeners) l();
}

const subscribe = (l: () => void) => {
  listeners.add(l);
  return () => listeners.delete(l);
};
/** Re-renders the caller when the preferred zone changes; returns the zone. */
export function useUserTimeZone(): string {
  useSyncExternalStore(
    subscribe,
    () => version,
    () => version,
  );
  return getUserTimezone();
}

/** dayjs in the preferred zone (plain local dayjs on the system zone). */
export const zdayjs = (input?: ConfigType): Dayjs => (customZone ? dayjs(input).tz(customZone) : dayjs(input));
/** A wall-clock string ("YYYY-MM-DD", "YYYY-MM-DD HH:mm:ss", ...) read in the preferred zone. */
export const zparse = (value: string): Dayjs => (customZone ? dayjs.tz(value, customZone) : dayjs(value));

// Default the zone of every Intl / toLocale*String call that doesn't name one, so all
// formatted times (including <relative-time>) follow the preference.
const withZone = (options?: Intl.DateTimeFormatOptions): Intl.DateTimeFormatOptions | undefined =>
  customZone && !options?.timeZone ? { ...(options ?? {}), timeZone: customZone } : options;

let installed = false;
export function installUiTimeZone(): void {
  if (installed) return;
  installed = true;
  const Patched = function (this: unknown, locales?: string | string[], options?: Intl.DateTimeFormatOptions) {
    return new OriginalDateTimeFormat(locales, withZone(options));
  } as unknown as typeof Intl.DateTimeFormat;
  Object.defineProperty(Patched, "prototype", { value: OriginalDateTimeFormat.prototype });
  Patched.supportedLocalesOf = OriginalDateTimeFormat.supportedLocalesOf.bind(OriginalDateTimeFormat);
  Intl.DateTimeFormat = Patched;
  const proto = Date.prototype;
  const { toLocaleString: origString, toLocaleDateString: origDate, toLocaleTimeString: origTime } = proto;
  proto.toLocaleString = function (locales?: string | string[], options?: Intl.DateTimeFormatOptions) {
    return origString.call(this, locales, withZone(options));
  };
  proto.toLocaleDateString = function (locales?: string | string[], options?: Intl.DateTimeFormatOptions) {
    return origDate.call(this, locales, withZone(options));
  };
  proto.toLocaleTimeString = function (locales?: string | string[], options?: Intl.DateTimeFormatOptions) {
    return origTime.call(this, locales, withZone(options));
  };
  setUserTimeZone(customZone);
}
