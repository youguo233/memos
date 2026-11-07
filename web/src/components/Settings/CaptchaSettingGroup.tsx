import { create } from "@bufbuild/protobuf";
import { Button, Input, Switch } from "@usememos/ui";
import { useEffect, useState } from "react";
import { instanceServiceClient } from "@/connect";
import { useInstance } from "@/contexts/InstanceContext";
import { handleError } from "@/lib/error";
import { showErrorToast, toast } from "@/lib/toast";
import { InstanceSetting_CaptchaSettingSchema, InstanceSetting_Key, InstanceSettingSchema } from "@/types/proto/api/instance_service_pb";
import { useTranslate } from "@/utils/i18n";
import SettingGroup from "./SettingGroup";
import { SettingList, SettingListItem } from "./SettingList";
import { buildInstanceSettingName } from "./useInstanceSettingUpdater";

const CaptchaSettingGroup = () => {
  const t = useTranslate();
  const { initialize } = useInstance();
  const [enabled, setEnabled] = useState(false);
  const [siteKey, setSiteKey] = useState("");
  const [secretKey, setSecretKey] = useState("");
  const [secretHint, setSecretHint] = useState("");
  const [saving, setSaving] = useState(false);
  const name = buildInstanceSettingName(InstanceSetting_Key.CAPTCHA);

  const load = async () => {
    const setting = await instanceServiceClient.getInstanceSetting({ name });
    if (setting.value.case === "captchaSetting") {
      const c = setting.value.value;
      setEnabled(c.enabled);
      setSiteKey(c.siteKey);
      setSecretHint(c.secretKeySet ? c.secretKeyHint || "••••" : "");
    }
  };

  useEffect(() => {
    load().catch((error: unknown) => handleError(error, showErrorToast, { context: "Load captcha setting" }));
  }, []);

  const handleSave = async () => {
    if (enabled && (!siteKey.trim() || (!secretKey.trim() && !secretHint))) {
      showErrorToast(t("setting.captcha.keys-required"));
      return;
    }
    setSaving(true);
    try {
      await instanceServiceClient.updateInstanceSetting({
        instanceSetting: create(InstanceSettingSchema, {
          name,
          value: {
            case: "captchaSetting",
            value: create(InstanceSetting_CaptchaSettingSchema, { enabled, siteKey: siteKey.trim(), secretKey: secretKey.trim() }),
          },
        }),
      });
      setSecretKey("");
      await load();
      await initialize();
      toast.add({ title: t("message.update-succeed"), type: "success" });
    } catch (error: unknown) {
      await handleError(error, showErrorToast, { context: "Update captcha setting" });
    }
    setSaving(false);
  };

  return (
    <SettingGroup title={t("setting.captcha.title")} description={t("setting.captcha.description")} showSeparator>
      <SettingList>
        <SettingListItem label={t("setting.captcha.enable")}>
          <Switch checked={enabled} onCheckedChange={setEnabled} />
        </SettingListItem>
        <SettingListItem label={t("setting.captcha.site-key")}>
          <Input className="w-72" value={siteKey} onChange={(e) => setSiteKey(e.target.value)} placeholder="0x4AAAAAAA..." />
        </SettingListItem>
        <SettingListItem
          label={t("setting.captcha.secret-key")}
          description={secretHint ? `${t("setting.captcha.secret-set")}: ${secretHint}` : undefined}
        >
          <Input
            className="w-72"
            type="password"
            autoComplete="new-password"
            value={secretKey}
            onChange={(e) => setSecretKey(e.target.value)}
            placeholder={secretHint ? t("setting.captcha.secret-keep") : "0x4AAAAAAA..."}
          />
        </SettingListItem>
      </SettingList>
      <div className="flex justify-end">
        <Button disabled={saving} onClick={handleSave}>
          {t("common.save")}
        </Button>
      </div>
    </SettingGroup>
  );
};

export default CaptchaSettingGroup;
