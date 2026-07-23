import { useEffect, useState } from "react";
import { Download, Mic, ArrowDownLeft, ArrowUpRight, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/shared/EmptyState";
import { useI18n } from "@/lib/i18n";
import { fetchRecordings, downloadRecordingUrl, type Recording } from "@/services/recordings";

export const RecordingsPage = ({ sid }: { sid: string }) => {
  const t = useI18n((s) => s.t);
  const [recordings, setRecordings] = useState<Recording[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;
    setLoading(true);
    fetchRecordings(sid)
      .then((r) => { if (active) setRecordings(r.recordings ?? []); })
      .catch(() => { if (active) setRecordings([]); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [sid]);

  const formatDuration = (secs: number) => {
    const m = Math.floor(secs / 60);
    const s = secs % 60;
    return `${m}:${String(s).padStart(2, "0")}`;
  };

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const formatPeer = (peer: string) => {
    if (!peer) return "—";
    const num = peer.split("@")[0];
    return num.length > 12 ? `+${num.slice(-10)}` : num || "—";
  };

  const formatDate = (iso: string) => {
    const d = new Date(iso);
    return d.toLocaleDateString(undefined, { day: "2-digit", month: "short", year: "numeric" })
      + " " + d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (recordings.length === 0) {
    return (
      <EmptyState
        icon={<Mic className="h-6 w-6" />}
        title={t("no_recordings")}
        description={t("no_recordings_desc")}
      />
    );
  }

  return (
    <div className="space-y-3">
      <h2 className="text-lg font-semibold">{t("recordings")}</h2>
      <div className="space-y-2">
        {recordings.map((r) => (
          <div
            key={r.id}
            className="flex items-center gap-3 rounded-lg border p-3 transition-colors hover:bg-muted/50"
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10">
              {r.direction === "inbound" ? (
                <ArrowDownLeft className="h-4 w-4 text-primary" />
              ) : (
                <ArrowUpRight className="h-4 w-4 text-primary" />
              )}
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="font-medium">{formatPeer(r.peer)}</span>
                <span className="text-xs text-muted-foreground">
                  {r.direction === "inbound" ? t("inbound") : t("outbound")}
                </span>
              </div>
              <div className="flex items-center gap-3 text-xs text-muted-foreground">
                <span>{formatDate(r.created_at)}</span>
                <span>{formatDuration(r.duration)}</span>
                <span>{formatSize(r.file_size)}</span>
              </div>
            </div>
            <Button
              variant="ghost"
              size="icon"
              asChild
            >
              <a href={downloadRecordingUrl(r.id)} download>
                <Download className="h-4 w-4" />
              </a>
            </Button>
          </div>
        ))}
      </div>
    </div>
  );
};
