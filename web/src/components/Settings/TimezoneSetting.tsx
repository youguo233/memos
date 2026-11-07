import { Input, Select } from "@usememos/ui";
import { useEffect, useMemo, useState } from "react";
import { browserTimeZone, listTimeZones } from "@/lib/timezone";
import { showErrorToast } from "@/lib/toast";
import { useTranslate } from "@/utils/i18n";
import { SettingListItem } from "./SettingList";

interface Props {
  value: string;
  disabled?: boolean;
  onChange: (timezone: string) => void;
}

const TimezoneSetting = ({ value, disabled, onChange }: Props) => {
  const t = useTranslate();
  const zones = useMemo(() => listTimeZones(), []);
  const [mode, setMode] = useState<"system" | "custom">(value ? "custom" : "system");
  const [draft, setDraft] = useState(value || browserTimeZone());

  useEffect(() => {
    setMode(value ? "custom" : "system");
    if (value) setDraft(value);
  }, [value]);

  const modeItems = [
    { value: "system", label: t("setting.preference.timezone-system") },
    { value: "custom", label: t("setting.preference.timezone-custom") },
  ];

  const commit = (zone: string) => {
    const tz = zone.trim();
    if (!zones.includes(tz)) {
      showErrorToast(t("setting.preference.timezone-invalid"));
      return;
    }
    if (tz !== value) onChange(tz);
  };

  return (
    <>
      <SettingListItem
        label={t("setting.preference.timezone")}
        description={t("setting.preference.timezone-description", { zone: browserTimeZone() })}
      >
        <Select.Root
          value={mode}
          items={modeItems}
          disabled={disabled}
          onValueChange={(next) => {
            if (next === null) return;
            const m = next as "system" | "custom";
            setMode(m);
            if (m === "system") {
              if (value) onChange("");
            } else {
              commit(draft);
            }
          }}
        >
          <Select.Trigger className="min-w-fit">
            <Select.Value />
          </Select.Trigger>
          <Select.Popup>
            {modeItems.map((item) => (
              <Select.Item key={item.value} value={item.value}>
                {item.label}
              </Select.Item>
            ))}
          </Select.Popup>
        </Select.Root>
      </SettingListItem>
      {mode === "custom" && (
        <SettingListItem label={t("setting.preference.timezone-custom")} description={t("setting.preference.timezone-search")}>
          <Input
            className="w-60"
            list="memos-timezones"
            value={draft}
            disabled={disabled}
            onChange={(e) => {
              setDraft(e.target.value);
              if (zones.includes(e.target.value) && e.target.value !== value) onChange(e.target.value);
            }}
            onBlur={() => commit(draft)}
            onKeyDown={(e) => {
              if (e.key === "Enter") commit(draft);
            }}
          />
          <datalist id="memos-timezones">
            {zones.map((zone) => (
              <option key={zone} value={zone} />
            ))}
          </datalist>
        </SettingListItem>
      )}
    </>
  );
};

export default TimezoneSetting;
