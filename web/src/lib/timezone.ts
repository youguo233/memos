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
export const browserTimeZone = (): string => Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";

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
