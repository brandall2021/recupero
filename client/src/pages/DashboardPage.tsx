import { useEffect, useState } from "react";
import { PhoneCall, PhoneIncoming, PhoneOutgoing, Users, Mic, Clock, Activity } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { apiGet } from "@/lib/api";
import { useI18n } from "@/lib/i18n";

interface DashboardStats {
  totalSessions: number;
  activeSessions: number;
  totalUsers: number;
  totalRecordings: number;
  totalRecordingsSize: number;
  totalCallsAll: number;
  activeCalls: number;
  inboundCalls: number;
  outboundCalls: number;
  avgDuration: number;
  totalRecordingDuration: number;
}

interface RecentCall {
  sessionId: string;
  callId: string;
  owner: string | null;
  direction: string;
  peer: string;
  startedAt: number;
  status: string;
  endedAt?: number;
  endReason?: string;
}

interface DashboardData {
  stats: DashboardStats;
  recentCalls: RecentCall[];
  sessions: { id: string; name: string; jid: string; state: string; paired: boolean }[];
}

const fmtDuration = (sec: number) => {
  if (sec < 60) return `${Math.round(sec)}s`;
  const m = Math.floor(sec / 60);
  const s = Math.round(sec % 60);
  return `${m}m ${s}s`;
};

const fmtSize = (bytes: number) => {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1048576).toFixed(1)} MB`;
};

const fmtTime = (ms: number) => {
  const d = new Date(ms);
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
};

export const DashboardPage = () => {
  const t = useI18n((s) => s.t);
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState("");

  const load = async () => {
    try {
      const res = await apiGet<DashboardData>("/api/dashboard");
      setData(res);
      setError("");
    } catch {
      setError("Error loading dashboard");
    }
  };

  useEffect(() => {
    load();
    const i = setInterval(load, 10000);
    return () => clearInterval(i);
  }, []);

  if (error) return <p className="text-sm text-destructive">{error}</p>;
  if (!data) return <p className="text-sm text-muted-foreground">{t("loading")}</p>;

  const s = data.stats;

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <h2 className="text-lg font-semibold">{t("dashboard")}</h2>

      {/* Stats cards */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        <StatCard icon={<Users className="h-4 w-4" />} label={t("active_sessions")} value={s.activeSessions} sub={`${s.totalSessions} ${t("total").toLowerCase()}`} />
        <StatCard icon={<PhoneCall className="h-4 w-4" />} label={t("active_calls_dashboard")} value={s.activeCalls} sub={`${s.totalCallsAll} ${t("total").toLowerCase()}`} />
        <StatCard icon={<Mic className="h-4 w-4" />} label={t("recordings")} value={s.totalRecordings} sub={fmtSize(s.totalRecordingsSize)} />
        <StatCard icon={<Clock className="h-4 w-4" />} label={t("avg_duration")} value={fmtDuration(s.avgDuration)} sub={`${fmtDuration(s.totalRecordingDuration)} ${t("recorded").toLowerCase()}`} />
        <StatCard icon={<PhoneIncoming className="h-4 w-4" />} label={t("inbound")} value={s.inboundCalls} sub={t("inbound").toLowerCase()} />
        <StatCard icon={<PhoneOutgoing className="h-4 w-4" />} label={t("outbound")} value={s.outboundCalls} sub={t("outbound").toLowerCase()} />
        <StatCard icon={<Users className="h-4 w-4" />} label={t("users")} value={s.totalUsers} sub={t("registered")} />
        <StatCard icon={<Activity className="h-4 w-4" />} label={t("uptime")} value={fmtDuration((Date.now() - 0) / 1000)} sub="" />
      </div>

      {/* Sessions */}
      {data.sessions.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium">{t("accounts")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {data.sessions.map((sess) => (
                <div key={sess.id} className="flex items-center justify-between rounded-md border px-3 py-2">
                  <div className="flex items-center gap-2">
                    <span className={`h-2 w-2 rounded-full ${sess.paired ? "bg-primary" : "bg-muted-foreground/50"}`} />
                    <span className="text-sm font-medium">{sess.name}</span>
                    {sess.jid && <span className="text-xs text-muted-foreground">{sess.jid.split("@")[0]}</span>}
                  </div>
                  <Badge variant={sess.paired ? "success" : "muted"}>{sess.state}</Badge>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Recent calls */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">{t("call_history")}</CardTitle>
        </CardHeader>
        <CardContent>
          {data.recentCalls.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("no_history")}</p>
          ) : (
            <div className="space-y-1">
              {data.recentCalls.map((c) => (
                <div key={c.callId} className="flex items-center justify-between rounded-md px-3 py-2 hover:bg-muted/50">
                  <div className="flex items-center gap-3">
                    {c.direction === "inbound" ? (
                      <PhoneIncoming className="h-4 w-4 text-green-500" />
                    ) : (
                      <PhoneOutgoing className="h-4 w-4 text-blue-500" />
                    )}
                    <div>
                      <p className="text-sm font-medium">{c.peer.split("@")[0]}</p>
                      <p className="text-xs text-muted-foreground">{fmtTime(c.startedAt)}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {c.endedAt && c.startedAt && (
                      <span className="text-xs text-muted-foreground">{fmtDuration((c.endedAt - c.startedAt) / 1000)}</span>
                    )}
                    <Badge variant={c.status === "ended" ? "secondary" : c.status === "connected" ? "success" : "outline"}>
                      {c.status}
                    </Badge>
                    {c.endReason && <Badge variant="muted">{c.endReason}</Badge>}
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
};

function StatCard({ icon, label, value, sub }: { icon: React.ReactNode; label: string; value: string | number; sub: string }) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center gap-2 text-muted-foreground">
          {icon}
          <span className="text-xs font-medium">{label}</span>
        </div>
        <p className="mt-2 text-2xl font-bold">{value}</p>
        {sub && <p className="text-xs text-muted-foreground">{sub}</p>}
      </CardContent>
    </Card>
  );
}
