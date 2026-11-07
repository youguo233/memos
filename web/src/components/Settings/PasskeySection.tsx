import { Button } from "@usememos/ui";
import { FingerprintIcon, Trash2Icon } from "lucide-react";
import { useEffect, useState } from "react";
import { deletePasskey, listPasskeys, type Passkey, passkeySupported, registerPasskey } from "@/lib/passkey";
import { showErrorToast, toast } from "@/lib/toast";
import { useTranslate } from "@/utils/i18n";
import SettingGroup from "./SettingGroup";

const PasskeySection = () => {
  const t = useTranslate();
  const [passkeys, setPasskeys] = useState<Passkey[]>([]);
  const [busy, setBusy] = useState(false);

  const refresh = () =>
    listPasskeys()
      .then(setPasskeys)
      .catch((error: unknown) => showErrorToast(error instanceof Error ? error.message : String(error)));

  useEffect(() => {
    refresh();
  }, []);

  const handleAdd = async () => {
    const name = window.prompt(t("setting.passkey.name-prompt"), navigator.platform || "Passkey");
    if (name === null) return;
    setBusy(true);
    try {
      await registerPasskey(name.trim());
      toast.add({ title: t("setting.passkey.added"), type: "success" });
      await refresh();
    } catch (error: unknown) {
      showErrorToast(error instanceof Error ? error.message : String(error));
    }
    setBusy(false);
  };

  const handleDelete = async (passkey: Passkey) => {
    if (!window.confirm(t("setting.passkey.delete-confirm", { name: passkey.name }))) return;
    try {
      await deletePasskey(passkey.id);
      await refresh();
    } catch (error: unknown) {
      showErrorToast(error instanceof Error ? error.message : String(error));
    }
  };

  const formatTs = (ts: number) => (ts ? new Date(ts * 1000).toLocaleString() : "—");

  return (
    <SettingGroup showSeparator title={t("setting.passkey.title")} description={t("setting.passkey.description")}>
      <div className="flex flex-col gap-2">
        {passkeys.length === 0 && <p className="text-sm text-mu-ink-muted">{t("setting.passkey.empty")}</p>}
        {passkeys.map((passkey) => (
          <div key={passkey.id} className="flex items-center justify-between gap-3 rounded-lg border border-mu-line/60 px-3 py-2">
            <div className="flex min-w-0 items-center gap-2">
              <FingerprintIcon className="size-4 shrink-0 text-mu-ink-muted" />
              <div className="min-w-0">
                <div className="truncate text-sm font-medium">{passkey.name}</div>
                <div className="text-xs text-mu-ink-muted">
                  {t("setting.passkey.created")}: {formatTs(passkey.createdTs)} · {t("setting.passkey.last-used")}:{" "}
                  {formatTs(passkey.lastUsedTs)}
                </div>
              </div>
            </div>
            <Button variant="ghost" size="sm" aria-label={t("common.delete")} onClick={() => handleDelete(passkey)}>
              <Trash2Icon className="size-4" />
            </Button>
          </div>
        ))}
        <div>
          <Button size="sm" variant="neutral" disabled={busy || !passkeySupported()} onClick={handleAdd}>
            <FingerprintIcon className="mr-1.5 size-4" />
            {t("setting.passkey.add")}
          </Button>
        </div>
      </div>
    </SettingGroup>
  );
};

export default PasskeySection;
