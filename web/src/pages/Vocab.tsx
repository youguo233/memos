import { BookAIcon, DownloadIcon, PencilIcon, Trash2Icon, UploadIcon, Volume2Icon } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "react-hot-toast";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { isTypingTarget } from "@/lib/hotkeys";
import { getUserTimezone } from "@/lib/timezone";
import { cn } from "@/lib/utils";
import {
  createVocabCard,
  deleteVocabCard,
  exportVocabCsv,
  formatInterval,
  getVocabAccent,
  getVocabSession,
  getVocabSettings,
  getVocabStats,
  importVocabCsv,
  listVocabCards,
  lookupVocab,
  notifyVocabStatsChanged,
  playPronunciation,
  reviewVocabCard,
  saveVocabSettings,
  setVocabAccent,
  updateVocabCard,
  VOCAB_MARKER,
  VocabAccent,
  VocabCard,
  VocabCardInput,
  VocabExistsError,
  VocabSettings,
  VocabStats,
} from "@/lib/vocab";
import { useTranslate } from "@/utils/i18n";

type Tab = "review" | "library" | "add" | "settings";

const emptyForm = (): VocabCardInput => ({
  term: "",
  phonetic: "",
  definition: "",
  example: "",
  notes: "",
  tags: [],
});

const emptySettingsForm = () => ({
  enabled: false,
  baseUrl: "",
  model: "",
  prompt: "",
  apiKey: "",
});

type FormattedLineKind = "common" | "other" | "en" | "zh" | "heading" | "bullet" | "plain";

const formattedLinePattern = /^(【常用】|【也作】|EN:|中:|【[^】]+】|[·•\-—]\s)/;

function VocabFormattedText({ text, variant }: { text: string; variant?: "definition" | "example" | "notes" }) {
  const trimmedText = text.trim();
  if (!trimmedText) return null;
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  const hasMarkers = lines.some((line) => formattedLinePattern.test(line.trim()));
  if (!hasMarkers) {
    if (variant === "example") {
      return <p className="text-sm text-muted-foreground italic whitespace-pre-wrap leading-6">{text}</p>;
    }
    return <p className={cn("whitespace-pre-wrap", variant === "notes" ? "text-sm leading-6" : "text-base leading-7")}>{text}</p>;
  }

  let prev: FormattedLineKind | "" = "";
  return (
    <div className="flex flex-col gap-1">
      {lines.map((line, index) => {
        const trimmed = line.trim();
        if (!trimmed) {
          prev = "";
          return <div key={index} className="h-1.5" />;
        }
        const common = trimmed.match(/^【常用】(.*)$/);
        if (common) {
          prev = "common";
          return (
            <div key={index} className="flex items-start gap-2 mt-1 first:mt-0">
              <span className="shrink-0 mt-0.5 text-[11px] font-semibold px-1.5 py-0.5 rounded bg-amber-500/15 text-amber-700 dark:text-amber-400">
                常用
              </span>
              <span className="font-medium leading-7">{common[1].trim()}</span>
            </div>
          );
        }
        const other = trimmed.match(/^【也作】(.*)$/);
        if (other) {
          prev = "other";
          return (
            <div key={index} className="flex items-start gap-2 mt-2">
              <span className="shrink-0 mt-0.5 text-[11px] px-1.5 py-0.5 rounded bg-muted text-muted-foreground">也作</span>
              <span className="text-muted-foreground leading-7">{other[1].trim()}</span>
            </div>
          );
        }
        const en = trimmed.match(/^EN:\s*(.*)$/i);
        if (en) {
          prev = "en";
          return (
            <p key={index} className="text-sm italic leading-6 mt-2 first:mt-0">
              “{en[1]}”
            </p>
          );
        }
        const zh = trimmed.match(/^中:\s*(.*)$/);
        if (zh) {
          prev = "zh";
          return (
            <p key={index} className="text-sm text-muted-foreground leading-6">
              {zh[1]}
            </p>
          );
        }
        const heading = trimmed.match(/^【([^】]+)】$/);
        if (heading) {
          prev = "heading";
          return (
            <div key={index} className="text-xs font-semibold text-muted-foreground mt-3 first:mt-0">
              {heading[1]}
            </div>
          );
        }
        const bullet = trimmed.match(/^[·•\-—]\s*(.*)$/);
        if (bullet) {
          prev = "bullet";
          return (
            <div key={index} className="flex gap-2 text-sm leading-6">
              <span className="text-muted-foreground">·</span>
              <span>{bullet[1]}</span>
            </div>
          );
        }
        const continuation =
          prev === "common" ? "text-base leading-7" : prev === "other" ? "text-sm text-muted-foreground leading-6" : "text-sm leading-6";
        prev = "plain";
        return (
          <p key={index} className={continuation}>
            {trimmed}
          </p>
        );
      })}
    </div>
  );
}

const Vocab = () => {
  const t = useTranslate();
  const [tab, setTab] = useState<Tab>("review");
  const [stats, setStats] = useState<VocabStats>({ total: 0, new: 0, learning: 0, review: 0, due: 0 });
  const [session, setSession] = useState<VocabCard[]>([]);
  const [index, setIndex] = useState(0);
  const [revealed, setRevealed] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const [cards, setCards] = useState<VocabCard[]>([]);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("");
  const [sort, setSort] = useState("added");
  const [form, setForm] = useState<VocabCardInput>(emptyForm());
  const [editingUid, setEditingUid] = useState<string>();
  const [saving, setSaving] = useState(false);
  const [lookingUp, setLookingUp] = useState(false);
  const [loadingLibrary, setLoadingLibrary] = useState(false);
  const [accent, setAccentState] = useState<VocabAccent>(() => getVocabAccent());
  const [settings, setSettings] = useState<VocabSettings>();
  const [settingsForm, setSettingsForm] = useState(emptySettingsForm);
  const [savingSettings, setSavingSettings] = useState(false);
  const [lookupAudioUrl, setLookupAudioUrl] = useState("");

  const changeAccent = (next: VocabAccent) => {
    setVocabAccent(next);
    setAccentState(next);
  };

  const refreshStats = useCallback(async () => {
    try {
      const next = await getVocabStats();
      setStats(next);
      notifyVocabStatsChanged();
    } catch {
      /* keep previous */
    }
  }, []);

  const refreshSession = useCallback(async () => {
    try {
      const next = await getVocabSession();
      setSession(next);
      setIndex(0);
      setRevealed(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("vocab.loadFailed"));
    }
  }, [t]);

  const refreshLibrary = useCallback(async () => {
    setLoadingLibrary(true);
    try {
      setCards(await listVocabCards({ q: query, filter, sort, limit: 200 }));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("vocab.loadFailed"));
    } finally {
      setLoadingLibrary(false);
    }
  }, [filter, query, sort, t]);

  const refreshSettings = useCallback(async () => {
    try {
      const next = await getVocabSettings();
      setSettings(next);
      setSettingsForm({
        enabled: next.enabled,
        baseUrl: next.baseUrl,
        model: next.model,
        prompt: next.prompt,
        apiKey: "",
      });
    } catch {
      /* keep previous */
    }
  }, []);

  useEffect(() => {
    void refreshStats();
    void refreshSettings();
  }, [refreshSettings, refreshStats]);

  useEffect(() => {
    if (tab === "review") void refreshSession();
    if (tab === "library") void refreshLibrary();
    if (tab === "settings") void refreshSettings();
  }, [refreshLibrary, refreshSession, refreshSettings, tab]);

  const current = session[index];

  const handleReview = useCallback(
    async (rating: number) => {
      if (!current || reviewing) return;
      setReviewing(true);
      try {
        await reviewVocabCard(current.uid, rating);
        setSession((prev) => prev.filter((_, i) => i !== index));
        setRevealed(false);
        await refreshStats();
      } catch (error) {
        toast.error(error instanceof Error ? error.message : t("vocab.reviewFailed"));
      } finally {
        setReviewing(false);
      }
    },
    [current, index, refreshStats, reviewing, t],
  );

  useEffect(() => {
    if (tab !== "review" || !current) return;
    const onKey = (event: KeyboardEvent) => {
      if (isTypingTarget(event.target)) return;
      if (!revealed && (event.key === " " || event.key === "Enter")) {
        event.preventDefault();
        setRevealed(true);
        return;
      }
      if (!revealed) return;
      if (event.key === "1") void handleReview(1);
      if (event.key === "2") void handleReview(2);
      if (event.key === "3") void handleReview(3);
      if (event.key === "4") void handleReview(4);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [current, handleReview, revealed, tab]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return;
      if (event.isComposing || event.keyCode === 229) return;
      if (isTypingTarget(event.target)) return;
      const key = event.key.toLowerCase();
      if (key === "r") {
        event.preventDefault();
        setTab("review");
        return;
      }
      if (key === "v") {
        event.preventDefault();
        setTab("library");
        return;
      }
      if (key === "a") {
        event.preventDefault();
        setTab("add");
        return;
      }
      if (key === "s") {
        event.preventDefault();
        setTab("settings");
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const saveCard = async () => {
    if (!form.term.trim() || !form.definition.trim()) {
      toast.error(t("vocab.required"));
      return;
    }
    setSaving(true);
    try {
      if (editingUid) {
        await updateVocabCard(editingUid, form);
        toast.success(t("vocab.saved"));
      } else {
        await createVocabCard(form);
        toast.success(t("vocab.saved"));
      }
      setForm(emptyForm());
      setEditingUid(undefined);
      setLookupAudioUrl("");
      await refreshStats();
      setTab("library");
    } catch (error) {
      if (error instanceof VocabExistsError) {
        toast.error(t("vocab.exists"));
      } else {
        toast.error(error instanceof Error ? error.message : t("vocab.saveFailed"));
      }
    } finally {
      setSaving(false);
    }
  };

  const startEdit = (card: VocabCard) => {
    setEditingUid(card.uid);
    setLookupAudioUrl("");
    setForm({
      term: card.term,
      phonetic: card.phonetic,
      definition: card.definition,
      example: card.example,
      notes: card.notes,
      tags: card.tags ?? [],
    });
    setTab("add");
  };

  const removeCard = async (card: VocabCard) => {
    if (!window.confirm(t("vocab.confirmDelete", { term: card.term }))) return;
    try {
      await deleteVocabCard(card.uid);
      setCards((prev) => prev.filter((item) => item.uid !== card.uid));
      await refreshStats();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("vocab.deleteFailed"));
    }
  };

  const lookup = async () => {
    if (!form.term.trim()) return;
    setLookingUp(true);
    try {
      const result = await lookupVocab(form.term);
      setLookupAudioUrl(result.audioUrl ?? "");
      setForm((prev) => ({
        ...prev,
        phonetic: prev.phonetic || result.phonetic,
        definition: prev.definition || result.definition,
        example: prev.example || result.example,
        notes: prev.notes || result.notes,
      }));
      toast.success(result.source === "llm" ? t("vocab.lookupLLMSource") : t("vocab.lookupDictionarySource"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("vocab.lookupFailed"));
    } finally {
      setLookingUp(false);
    }
  };

  const persistSettings = async (clearApiKey = false) => {
    const enabled = clearApiKey ? false : settingsForm.enabled;
    if (enabled && (!settingsForm.baseUrl.trim() || !settingsForm.model.trim() || (!settingsForm.apiKey.trim() && !settings?.configured))) {
      toast.error(t("vocab.llmRequired"));
      return;
    }
    setSavingSettings(true);
    try {
      const next = await saveVocabSettings({
        enabled,
        baseUrl: settingsForm.baseUrl,
        model: settingsForm.model,
        prompt: settingsForm.prompt,
        apiKey: settingsForm.apiKey,
        clearApiKey,
      });
      setSettings(next);
      setSettingsForm({
        enabled: next.enabled,
        baseUrl: next.baseUrl,
        model: next.model,
        prompt: next.prompt,
        apiKey: "",
      });
      toast.success(t("vocab.llmSaved"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("vocab.llmSaveFailed"));
    } finally {
      setSavingSettings(false);
    }
  };

  const exportCsv = async () => {
    try {
      const blob = await exportVocabCsv();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = "vocab.csv";
      link.click();
      URL.revokeObjectURL(url);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("vocab.exportFailed"));
    }
  };

  const importCsv = async (file?: File) => {
    if (!file) return;
    try {
      const imported = await importVocabCsv(await file.text());
      toast.success(t("vocab.imported", { count: imported }));
      await refreshStats();
      await refreshLibrary();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("vocab.importFailed"));
    }
  };

  const ratingButtons = useMemo(
    () =>
      [
        { rating: 1, label: t("vocab.again"), className: "bg-red-500/90 hover:bg-red-500 text-white" },
        { rating: 2, label: t("vocab.hard"), className: "bg-orange-500/90 hover:bg-orange-500 text-white" },
        { rating: 3, label: t("vocab.good"), className: "bg-emerald-600/90 hover:bg-emerald-600 text-white" },
        { rating: 4, label: t("vocab.easy"), className: "bg-sky-600/90 hover:bg-sky-600 text-white" },
      ] as const,
    [t],
  );

  const PronounceButton = ({ term, audioUrl }: { term: string; audioUrl?: string }) => {
    const [busy, setBusy] = useState(false);
    if (!term.trim()) return null;
    return (
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="shrink-0"
        disabled={busy}
        aria-label={t("vocab.playAudio")}
        onClick={() => {
          setBusy(true);
          void playPronunciation(term, accent, audioUrl).finally(() => setBusy(false));
        }}
      >
        <Volume2Icon className="size-4" />
      </Button>
    );
  };

  return (
    <section
      data-vocab-marker={VOCAB_MARKER}
      className="@container w-full max-w-5xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8"
    >
      <div className="w-full px-4 sm:px-6">
        <div className="w-full border border-border flex flex-col rounded-xl bg-background text-foreground overflow-hidden">
          <div className="w-full px-4 py-4 border-b border-border flex flex-col gap-3">
            <div className="flex flex-row items-center gap-2">
              <BookAIcon className="w-5 h-auto text-muted-foreground" />
              <h1 className="text-xl font-semibold">{t("common.vocab")}</h1>
              <div className="ml-auto flex items-center gap-1 text-xs">
                <span className="text-muted-foreground mr-1">{t("vocab.accent")}</span>
                <Button type="button" size="sm" variant={accent === "us" ? "default" : "outline"} onClick={() => changeAccent("us")}>
                  {t("vocab.accentUS")}
                </Button>
                <Button type="button" size="sm" variant={accent === "uk" ? "default" : "outline"} onClick={() => changeAccent("uk")}>
                  {t("vocab.accentUK")}
                </Button>
              </div>
            </div>
            <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
              <span>{t("vocab.dueCount", { count: stats.due })}</span>
              <span>{t("vocab.newCount", { count: stats.new })}</span>
              <span>{t("vocab.totalCount", { count: stats.total })}</span>
            </div>
            <Tabs value={tab} onValueChange={(value) => setTab(value as Tab)}>
              <TabsList>
                <TabsTrigger value="review">{t("vocab.review")}</TabsTrigger>
                <TabsTrigger value="library">{t("vocab.library")}</TabsTrigger>
                <TabsTrigger value="add">{editingUid ? t("vocab.edit") : t("vocab.add")}</TabsTrigger>
                <TabsTrigger value="settings">{t("vocab.settings")}</TabsTrigger>
              </TabsList>
            </Tabs>
            <p className="text-xs text-muted-foreground">{t("vocab.tabShortcutHint")}</p>
          </div>

          {tab === "review" && (
            <div className="w-full p-4 sm:p-6 flex flex-col gap-4">
              {!current ? (
                <div className="text-sm text-muted-foreground py-12 text-center">{t("vocab.emptyReview")}</div>
              ) : (
                <>
                  <div className="rounded-lg border border-border p-5 min-h-44 flex flex-col gap-3">
                    <div className="flex items-start justify-between gap-2">
                      <div className="text-2xl font-semibold tracking-tight">{current.term}</div>
                      <PronounceButton term={current.term} />
                    </div>
                    {current.phonetic && <div className="text-sm text-muted-foreground">{current.phonetic}</div>}
                    {revealed ? (
                      <>
                        <VocabFormattedText text={current.definition} variant="definition" />
                        {current.example && <VocabFormattedText text={current.example} variant="example" />}
                        {current.notes && <VocabFormattedText text={current.notes} variant="notes" />}
                        {current.tags?.length > 0 && (
                          <div className="flex flex-wrap gap-1">
                            {current.tags.map((tag) => (
                              <span key={tag} className="text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground">
                                {tag}
                              </span>
                            ))}
                          </div>
                        )}
                      </>
                    ) : (
                      <Button className="mt-auto self-start" onClick={() => setRevealed(true)}>
                        {t("vocab.showAnswer")}
                      </Button>
                    )}
                  </div>
                  {revealed && (
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                      {ratingButtons.map((item) => (
                        <Button
                          key={item.rating}
                          className={cn("h-12 flex-col gap-0.5", item.className)}
                          disabled={reviewing}
                          onClick={() => void handleReview(item.rating)}
                        >
                          <span>{item.label}</span>
                          <span className="text-[11px] opacity-80">
                            {formatInterval(current.previews?.[String(item.rating)]?.dueTs ?? current.dueTs)}
                          </span>
                        </Button>
                      ))}
                    </div>
                  )}
                  <div className="text-xs text-muted-foreground">{t("vocab.shortcutHint")}</div>
                </>
              )}
            </div>
          )}

          {tab === "library" && (
            <div className="w-full p-4 sm:p-6 flex flex-col gap-3">
              <div className="flex flex-col sm:flex-row gap-2">
                <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t("vocab.search")} />
                <div className="flex gap-2">
                  <Button variant="outline" onClick={() => void refreshLibrary()}>
                    {t("vocab.search")}
                  </Button>
                  <Button variant="outline" onClick={() => void exportCsv()}>
                    <DownloadIcon className="size-4" />
                    {t("vocab.exportCsv")}
                  </Button>
                  <label className="inline-flex">
                    <input
                      type="file"
                      accept=".csv,text/csv"
                      className="hidden"
                      onChange={(event) => void importCsv(event.target.files?.[0])}
                    />
                    <Button
                      variant="outline"
                      type="button"
                      onClick={(event) => (event.currentTarget.previousElementSibling as HTMLInputElement | null)?.click()}
                    >
                      <UploadIcon className="size-4" />
                      {t("vocab.importCsv")}
                    </Button>
                  </label>
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                {[
                  { value: "", label: t("vocab.filterAll") },
                  { value: "due", label: t("vocab.filterDue") },
                  { value: "new", label: t("vocab.filterNew") },
                  { value: "learning", label: t("vocab.filterLearning") },
                  { value: "review", label: t("vocab.filterReview") },
                ].map((item) => (
                  <Button
                    key={item.value || "all"}
                    size="sm"
                    variant={filter === item.value ? "default" : "outline"}
                    onClick={() => setFilter(item.value)}
                  >
                    {item.label}
                  </Button>
                ))}
                <select
                  className="h-8 rounded-md border border-border bg-transparent px-2 text-sm"
                  value={sort}
                  onChange={(event) => setSort(event.target.value)}
                >
                  <option value="added">{t("vocab.sortAdded")}</option>
                  <option value="term">{t("vocab.sortTerm")}</option>
                  <option value="due">{t("vocab.sortDue")}</option>
                </select>
              </div>
              {loadingLibrary ? (
                <div className="text-sm text-muted-foreground py-8 text-center">{t("vocab.loading")}</div>
              ) : cards.length === 0 ? (
                <div className="text-sm text-muted-foreground py-8 text-center">{t("vocab.emptyLibrary")}</div>
              ) : (
                <div className="divide-y divide-border">
                  {cards.map((card) => (
                    <div key={card.uid} className="py-3 flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="flex items-center gap-1">
                          <div className="font-medium">{card.term}</div>
                          <PronounceButton term={card.term} />
                        </div>
                        {card.phonetic && <div className="text-xs text-muted-foreground">{card.phonetic}</div>}
                        <div className="text-sm text-muted-foreground line-clamp-2">{card.definition}</div>
                        <div className="text-xs text-muted-foreground mt-1">
                          {t(
                            (["vocab.stateNew", "vocab.stateLearning", "vocab.stateReview", "vocab.stateRelearning"] as const)[
                              card.state
                            ] ?? "vocab.stateNew",
                          )}{" "}
                          · {new Date(card.dueTs * 1000).toLocaleString(undefined, { timeZone: getUserTimezone() })}
                        </div>
                      </div>
                      <div className="flex shrink-0">
                        <Button variant="ghost" size="icon" onClick={() => startEdit(card)} aria-label={t("vocab.edit")}>
                          <PencilIcon className="size-4" />
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => void removeCard(card)} aria-label={t("vocab.delete")}>
                          <Trash2Icon className="size-4" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {tab === "add" && (
            <div className="w-full p-4 sm:p-6 flex flex-col gap-3">
              <label className="text-sm font-medium">{t("vocab.term")}</label>
              <div className="flex gap-2">
                <Input
                  value={form.term}
                  onChange={(event) => setForm((prev) => ({ ...prev, term: event.target.value }))}
                  onKeyDown={(event) => {
                    if (event.key !== "Enter") return;
                    if (event.nativeEvent.isComposing || event.keyCode === 229) return;
                    event.preventDefault();
                    if (!lookingUp) void lookup();
                  }}
                />
                <PronounceButton term={form.term ?? ""} audioUrl={lookupAudioUrl} />
                <Button variant="outline" disabled={lookingUp} onClick={() => void lookup()}>
                  {lookingUp
                    ? t("vocab.lookupInProgress")
                    : settings?.enabled && settings.configured
                      ? t("vocab.lookupLLM")
                      : t("vocab.lookup")}
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">{t("vocab.lookupHint")}</p>
              <label className="text-sm font-medium">{t("vocab.phonetic")}</label>
              <Input value={form.phonetic} onChange={(event) => setForm((prev) => ({ ...prev, phonetic: event.target.value }))} />
              <label className="text-sm font-medium">{t("vocab.definition")}</label>
              <Textarea value={form.definition} onChange={(event) => setForm((prev) => ({ ...prev, definition: event.target.value }))} />
              <label className="text-sm font-medium">{t("vocab.example")}</label>
              <Textarea value={form.example} onChange={(event) => setForm((prev) => ({ ...prev, example: event.target.value }))} />
              <label className="text-sm font-medium">{t("vocab.notes")}</label>
              <Textarea value={form.notes} onChange={(event) => setForm((prev) => ({ ...prev, notes: event.target.value }))} />
              <label className="text-sm font-medium">{t("vocab.tags")}</label>
              <Input
                value={(form.tags ?? []).join(", ")}
                placeholder={t("vocab.tagsHint")}
                onChange={(event) =>
                  setForm((prev) => ({
                    ...prev,
                    tags: event.target.value
                      .split(",")
                      .map((item) => item.trim())
                      .filter(Boolean),
                  }))
                }
              />
              <div className="flex gap-2 pt-2">
                <Button disabled={saving} onClick={() => void saveCard()}>
                  {saving ? t("vocab.saving") : t("vocab.save")}
                </Button>
                {editingUid && (
                  <Button
                    variant="outline"
                    onClick={() => {
                      setEditingUid(undefined);
                      setLookupAudioUrl("");
                      setForm(emptyForm());
                    }}
                  >
                    {t("vocab.cancel")}
                  </Button>
                )}
              </div>
            </div>
          )}

          {tab === "settings" && (
            <div className="w-full p-4 sm:p-6 flex flex-col gap-3">
              <p className="text-sm text-muted-foreground">
                {settings?.configured ? t("vocab.llmConfigured") : t("vocab.llmNotConfigured")}
              </p>
              <label className="flex items-center gap-2 text-sm font-medium">
                <input
                  type="checkbox"
                  checked={settingsForm.enabled}
                  onChange={(event) => setSettingsForm((prev) => ({ ...prev, enabled: event.target.checked }))}
                />
                {t("vocab.llmEnable")}
              </label>
              <p className="text-xs text-muted-foreground">{t("vocab.llmEnableHint")}</p>
              <p className="text-xs text-muted-foreground">{t("vocab.llmHelp")}</p>
              <label className="text-sm font-medium">{t("vocab.llmBaseUrl")}</label>
              <Input
                value={settingsForm.baseUrl}
                placeholder="https://api.deepseek.com/v1"
                onChange={(event) => setSettingsForm((prev) => ({ ...prev, baseUrl: event.target.value }))}
              />
              <label className="text-sm font-medium">{t("vocab.llmModel")}</label>
              <Input
                value={settingsForm.model}
                placeholder="deepseek-chat"
                onChange={(event) => setSettingsForm((prev) => ({ ...prev, model: event.target.value }))}
              />
              <label className="text-sm font-medium">{t("vocab.llmApiKey")}</label>
              <Input
                type="password"
                autoComplete="off"
                value={settingsForm.apiKey}
                placeholder={settings?.apiKeyMasked || t("vocab.llmApiKeyHint")}
                onChange={(event) => setSettingsForm((prev) => ({ ...prev, apiKey: event.target.value }))}
              />
              {settings?.configured && (
                <p className="text-xs text-muted-foreground">{t("vocab.llmApiKeySaved", { masked: settings.apiKeyMasked })}</p>
              )}
              <label className="text-sm font-medium">{t("vocab.llmPrompt")}</label>
              <Textarea
                value={settingsForm.prompt}
                placeholder={t("vocab.llmPromptHint")}
                onChange={(event) => setSettingsForm((prev) => ({ ...prev, prompt: event.target.value }))}
              />
              <div className="flex flex-wrap gap-2 pt-2">
                <Button disabled={savingSettings} onClick={() => void persistSettings(false)}>
                  {t("vocab.llmSave")}
                </Button>
                {settings?.configured && (
                  <Button variant="outline" disabled={savingSettings} onClick={() => void persistSettings(true)}>
                    {t("vocab.llmClearKey")}
                  </Button>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </section>
  );
};

export default Vocab;
