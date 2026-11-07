import { Button, Menu, Tooltip } from "@usememos/ui";
import { CheckSquareIcon, DownloadIcon, FileArchiveIcon, FileDownIcon, XIcon } from "lucide-react";
import { useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { getErrorMessage } from "@/lib/error";
import { type BatchExportFormat, exportMemosBatch } from "@/lib/memo-docx";
import { memoSelection, useMemoSelection } from "@/lib/memo-selection";
import { showErrorToast } from "@/lib/toast";
import { useTranslate } from "@/utils/i18n";

type Choice = { format: BatchExportFormat; merge: boolean; label: string; icon: typeof FileDownIcon };

function useChoices(): Choice[] {
  const t = useTranslate();
  return [
    { format: "md", merge: false, label: t("memo.batch-export.md-zip"), icon: FileArchiveIcon },
    { format: "docx", merge: false, label: t("memo.batch-export.docx-zip"), icon: FileArchiveIcon },
    { format: "md", merge: true, label: t("memo.batch-export.md-merged"), icon: FileDownIcon },
    { format: "docx", merge: true, label: t("memo.batch-export.docx-merged"), icon: FileDownIcon },
  ];
}

function useRunExport() {
  const t = useTranslate();
  const { userGeneralSetting } = useAuth();
  const [busy, setBusy] = useState(false);
  const run = async (target: { names: string[] } | { tag: string }, choice: Choice) => {
    setBusy(true);
    try {
      await exportMemosBatch(target, choice.format, { merge: choice.merge, tz: userGeneralSetting?.timezone });
      return true;
    } catch (error) {
      showErrorToast(getErrorMessage(error, t("memo.batch-export.failed")));
      return false;
    } finally {
      setBusy(false);
    }
  };
  return { busy, run };
}

/**
 * The export control at the right end of the filter row: exports everything
 * under the active tag, or switches the list into multi-select mode.
 */
export const MemoBatchExportButton = ({ tag }: { tag?: string }) => {
  const t = useTranslate();
  const choices = useChoices();
  const { busy, run } = useRunExport();
  const label = t("memo.batch-export.title");
  return (
    <Menu.Root modal={false} size="sm">
      <Tooltip.Root>
        <Tooltip.Trigger
          render={<Menu.Trigger render={<Button variant="ghost" size="sm" disabled={busy} aria-label={label} icon={<DownloadIcon />} />} />}
        />
        <Tooltip.Popup>{label}</Tooltip.Popup>
      </Tooltip.Root>
      <Menu.Popup align="end" className="w-60">
        {tag && (
          <>
            <Menu.Group>
              <Menu.GroupLabel>{t("memo.batch-export.tag-heading", { tag })}</Menu.GroupLabel>
              {choices.map((choice) => (
                <Menu.Item key={choice.format + String(choice.merge)} onClick={() => run({ tag }, choice)} icon={<choice.icon />}>
                  {choice.label}
                </Menu.Item>
              ))}
            </Menu.Group>
            <Menu.Separator />
          </>
        )}
        <Menu.Item onClick={() => memoSelection.start()} icon={<CheckSquareIcon />}>
          {t("memo.batch-export.select")}
        </Menu.Item>
      </Menu.Popup>
    </Menu.Root>
  );
};

/** Shown above the list while multi-select is on: count, select all, export, cancel. */
export const MemoSelectionBar = () => {
  const t = useTranslate();
  const choices = useChoices();
  const { busy, run } = useRunExport();
  const selected = useMemoSelection((s) => s.selected);
  const visibleCount = useMemoSelection((s) => s.visible.length);
  const allSelected = visibleCount > 0 && selected.size >= visibleCount;
  return (
    <div className="flex w-full min-w-0 flex-row flex-wrap items-center gap-1 text-sm text-mu-ink-muted">
      <span className="me-auto tabular-nums">{t("memo.batch-export.selected-count", { count: selected.size })}</span>
      <Button variant="ghost" size="sm" onClick={() => (allSelected ? memoSelection.clear() : memoSelection.selectAll())}>
        {allSelected ? t("memo.batch-export.select-none") : t("memo.batch-export.select-all")}
      </Button>
      <Menu.Root modal={false} size="sm">
        <Menu.Trigger render={<Button variant="ghost" size="sm" disabled={busy || selected.size === 0} icon={<DownloadIcon />} />}>
          {t("memo.batch-export.export")}
        </Menu.Trigger>
        <Menu.Popup align="end" className="w-60">
          {choices.map((choice) => (
            <Menu.Item
              key={choice.format + String(choice.merge)}
              onClick={async () => {
                if (await run({ names: [...selected] }, choice)) memoSelection.exit();
              }}
              icon={<choice.icon />}
            >
              {choice.label}
            </Menu.Item>
          ))}
        </Menu.Popup>
      </Menu.Root>
      <Button variant="ghost" size="sm" onClick={() => memoSelection.exit()} aria-label={t("common.cancel")} icon={<XIcon />} />
    </div>
  );
};
