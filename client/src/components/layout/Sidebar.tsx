import { useState } from "react";
import { Loader2, Plus, Trash2, Phone, Users, CalendarDays, StickyNote, Mic, LogOut, LayoutDashboard, Shield, KeyRound, Copy, Check } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { setActiveSession, useSessions } from "@/stores/sessions";
import { createSession, deleteSession } from "@/services/sessions";
import { useAuth } from "@/stores/auth";
import { useI18n } from "@/lib/i18n";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import type { SessionInfo, SessionState } from "@/types/session";

export type PageId = "dashboard" | "calls" | "contacts" | "schedule" | "notes" | "recordings" | "users";

const dotClass: Record<SessionState, string> = {
  open: "bg-primary",
  qr: "bg-amber-500",
  connecting: "bg-muted-foreground/50",
  logged_out: "bg-destructive",
};

const navItems: { id: PageId; icon: typeof Phone; labelKey: string }[] = [
  { id: "dashboard", icon: LayoutDashboard, labelKey: "dashboard_nav" },
  { id: "calls", icon: Phone, labelKey: "calls_nav" },
  { id: "contacts", icon: Users, labelKey: "contacts_nav" },
  { id: "schedule", icon: CalendarDays, labelKey: "schedule_nav" },
  { id: "notes", icon: StickyNote, labelKey: "notes_nav" },
  { id: "recordings", icon: Mic, labelKey: "recordings_nav" },
  { id: "users", icon: Shield, labelKey: "users_nav" },
];

export const Sidebar = ({
  onNavigate,
  activePage,
  onSetPage,
}: {
  onNavigate?: () => void;
  activePage?: PageId;
  onSetPage?: (p: PageId) => void;
}) => {
  const sessions = useSessions((s) => s.sessions);
  const activeId = useSessions((s) => s.activeId);
  const t = useI18n((s) => s.t);
  const logout = useAuth((s) => s.logout);
  const [creating, setCreating] = useState(false);
  const [toDelete, setToDelete] = useState<SessionInfo | null>(null);
  const [creds, setCreds] = useState<SessionInfo | null>(null);
  const [copied, setCopied] = useState(false);

  const copyToken = async () => {
    if (!creds?.token) return;
    await navigator.clipboard.writeText(creds.token);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  const onNew = async () => {
    setCreating(true);
    try {
      const { id } = await createSession("WhatsApp");
      setActiveSession(id);
      onNavigate?.();
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setCreating(false);
    }
  };

  const remove = async (id: string) => {
    try {
      await deleteSession(id);
    } catch (e) {
      toast.error((e as Error).message);
    }
  };

  const nav = (p: PageId) => {
    onSetPage?.(p);
    onNavigate?.();
  };

  return (
    <div className="flex h-full flex-col gap-2 p-3">
      <div className="space-y-1">
        {navItems.map((item) => (
          <button
            key={item.id}
            onClick={() => nav(item.id)}
            className={cn(
              "flex w-full items-center gap-2 rounded-md px-2 py-2 text-sm",
              activePage === item.id ? "bg-accent text-accent-foreground font-medium" : "hover:bg-muted text-muted-foreground",
            )}
          >
            <item.icon className="h-4 w-4" />
            {t(item.labelKey as any)}
          </button>
        ))}
      </div>

      <div className="my-1 h-px bg-border" />

      <p className="px-2 pt-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
        {t("accounts")}
      </p>
      <div className="flex-1 space-y-1 overflow-y-auto">
        {sessions.map((s) => (
          <div
            key={s.id}
            role="button"
            tabIndex={0}
            onClick={() => {
              setActiveSession(s.id);
              onNavigate?.();
            }}
            className={cn(
              "group flex cursor-pointer items-center gap-2 rounded-md px-2 py-2 text-sm",
              s.id === activeId ? "bg-accent text-accent-foreground" : "hover:bg-muted",
            )}
          >
            <span className={cn("h-2 w-2 shrink-0 rounded-full", dotClass[s.state])} />
            <div className="min-w-0 flex-1">
              <p className="truncate font-medium">{s.name}</p>
              {s.jid && <p className="truncate text-xs text-muted-foreground">{s.jid.split("@")[0]}</p>}
            </div>
            <button
              onClick={(e) => {
                e.stopPropagation();
                setCreds(s);
              }}
              className="text-muted-foreground opacity-0 transition-opacity hover:text-foreground group-hover:opacity-100"
              aria-label={`Channel credentials for ${s.name}`}
            >
              <KeyRound className="h-4 w-4" />
            </button>
            <button
              onClick={(e) => {
                e.stopPropagation();
                setToDelete(s);
              }}
              className="text-muted-foreground opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100"
              aria-label={`Delete ${s.name}`}
            >
              <Trash2 className="h-4 w-4" />
            </button>
          </div>
        ))}
        {sessions.length === 0 && <p className="px-2 text-sm text-muted-foreground">No accounts yet.</p>}
      </div>
      <Button variant="outline" className="w-full" onClick={onNew} disabled={creating}>
        {creating ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
        {t("new_session")}
      </Button>

      <Button variant="ghost" className="w-full gap-2 text-muted-foreground" onClick={logout}>
        <LogOut className="h-4 w-4" />
        {t("logout")}
      </Button>

      <Dialog open={!!creds} onOpenChange={(o) => !o && setCreds(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("channel_credentials")}</DialogTitle>
            <DialogDescription>{t("channel_credentials_desc")}</DialogDescription>
          </DialogHeader>
          {creds && (
            <div className="space-y-4">
              <div className="space-y-1">
                <p className="text-xs font-medium text-muted-foreground">{t("channel_id_label")}</p>
                <p className="break-all rounded-md border bg-muted/50 px-3 py-2 font-mono text-sm">{creds.id}</p>
              </div>
              <div className="space-y-1">
                <p className="text-xs font-medium text-muted-foreground">{t("channel_token_label")}</p>
                <div className="flex items-center gap-2">
                  <p className="min-w-0 flex-1 break-all rounded-md border bg-muted/50 px-3 py-2 font-mono text-sm">
                    {creds.token || "—"}
                  </p>
                  {creds.token && (
                    <Button size="sm" variant="outline" onClick={copyToken}>
                      {copied ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}
                      <span className="ml-1">{copied ? t("copied") : t("copy")}</span>
                    </Button>
                  )}
                </div>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!toDelete}
        onOpenChange={(o) => !o && setToDelete(null)}
        title="Delete account?"
        description={toDelete ? `${toDelete.name} will be logged out and removed.` : undefined}
        confirmLabel="Delete"
        destructive
        onConfirm={() => {
          if (toDelete) void remove(toDelete.id);
        }}
      />
    </div>
  );
};
