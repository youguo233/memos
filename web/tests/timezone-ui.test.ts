import { afterEach, describe, expect, it } from "vitest";
import { getLocalDayTimestampRange, parseLocalDate } from "@/lib/calendar-utils";
import { installUiTimeZone, setUserTimeZone, zdayjs } from "@/lib/timezone";

installUiTimeZone();
afterEach(() => setUserTimeZone(""));

describe("UI time zone (mbe-tz-ui-v1)", () => {
  it("formats in the preferred zone", () => {
    const d = new Date("2026-01-01T00:30:00Z");
    setUserTimeZone("Asia/Tokyo");
    expect(d.toLocaleString("en-US", { hour: "2-digit", hourCycle: "h23" })).toBe("09");
    expect(new Intl.DateTimeFormat("en-US", { day: "numeric" }).format(d)).toBe("1");
    expect(zdayjs(d).format("YYYY-MM-DD HH:mm")).toBe("2026-01-01 09:30");
    setUserTimeZone("America/Los_Angeles");
    expect(zdayjs(d).format("YYYY-MM-DD")).toBe("2025-12-31");
    expect(d.toLocaleDateString("en-US", { day: "numeric" })).toBe("31");
  });
  it("uses zone day boundaries", () => {
    setUserTimeZone("Asia/Shanghai");
    expect(parseLocalDate("2026-03-05")?.toISOString()).toBe("2026-03-04T16:00:00.000Z");
    const r = getLocalDayTimestampRange("2026-03-05");
    expect(r && r.endTimestamp - r.startTimestamp).toBe(86400);
    expect(parseLocalDate("2026-02-30")).toBeUndefined();
  });
  it("ignores invalid zones", () => {
    setUserTimeZone("Not/AZone");
    expect(zdayjs(new Date(0)).isValid()).toBe(true);
  });
});
