import { useEffect, useMemo, useState } from "react";
import { toast } from "react-hot-toast";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { browserTimeZone, listTimeZones } from "@/lib/timezone";
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
      toast.error(t("setting.preference.timezone-invalid"));
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
        <Select
          value={mode}
          items={modeItems}
          onValueChange={(next) => {
            const m = next as "system" | "custom";
            setMode(m);
            if (m === "system") {
              if (value) onChange("");
            } else {
              commit(draft);
            }
          }}
        >
          <SelectTrigger className="min-w-fit" disabled={disabled}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {modeItems.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
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
