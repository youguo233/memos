import { CheckSquareIcon, DownloadIcon, FileArchiveIcon, FileDownIcon, XIcon } from "lucide-react";
import { useState } from "react";
import toast from "react-hot-toast";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useAuth } from "@/contexts/AuthContext";
import { getErrorMessage } from "@/lib/error";
import { type BatchExportFormat, exportMemosBatch } from "@/lib/memo-docx";
import { memoSelection, useMemoSelection } from "@/lib/memo-selection";
import { useTranslate } from "@/utils/i18n";

const ICON_BUTTON = "size-7 shrink-0 px-0 text-muted-foreground hover:text-foreground";

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
      toast.error(getErrorMessage(error, t("memo.batch-export.failed")));
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
    <DropdownMenu>
      <Tooltip>
        <TooltipTrigger
          render={
            <DropdownMenuTrigger render={<Button variant="quiet" size="sm" className={ICON_BUTTON} disabled={busy} aria-label={label} />} />
          }
        >
          <DownloadIcon className="size-4" />
        </TooltipTrigger>
        <TooltipContent>{label}</TooltipContent>
      </Tooltip>
      <DropdownMenuContent align="end" className="w-60">
        {tag && (
          <>
            <DropdownMenuLabel>{t("memo.batch-export.tag-heading", { tag })}</DropdownMenuLabel>
            {choices.map((c) => (
              <DropdownMenuItem key={c.format + c.merge} onClick={() => run({ tag }, c)}>
                <c.icon />
                {c.label}
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
          </>
        )}
        <DropdownMenuItem onClick={() => memoSelection.start()}>
          <CheckSquareIcon />
          {t("memo.batch-export.select")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
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
    <div className="flex w-full min-w-0 flex-row flex-wrap items-center gap-1 text-sm text-muted-foreground">
      <span className="me-auto tabular-nums">{t("memo.batch-export.selected-count", { count: selected.size })}</span>
      <Button variant="quiet" size="sm" onClick={() => (allSelected ? memoSelection.clear() : memoSelection.selectAll())}>
        {allSelected ? t("memo.batch-export.select-none") : t("memo.batch-export.select-all")}
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="quiet" size="sm" className="gap-1" disabled={busy || selected.size === 0} />}>
          <DownloadIcon className="size-4" />
          {t("memo.batch-export.export")}
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-60">
          {choices.map((c) => (
            <DropdownMenuItem
              key={c.format + c.merge}
              onClick={async () => {
                if (await run({ names: [...selected] }, c)) memoSelection.exit();
              }}
            >
              <c.icon />
              {c.label}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      <Button variant="quiet" size="sm" className={ICON_BUTTON} onClick={() => memoSelection.exit()} aria-label={t("common.cancel")}>
        <XIcon className="size-4" />
      </Button>
    </div>
  );
};
