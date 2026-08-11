import { useEffect, useState } from "react";
import { Building2, Loader2, Pencil, Trash2, Plus, X, Link2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/shared/EmptyState";
import { useI18n } from "@/lib/i18n";
import {
  listClients,
  createClient,
  updateClient,
  deleteClient,
  setClientStatus,
  setClientLimits,
} from "@/services/clients";
import { listPlatformSessions, assignSessionClient } from "@/services/sessions";
import type { ClientInfo, ClientStatus } from "@/types/client";
import type { SessionInfo } from "@/types/session";

const statusVariant: Record<ClientStatus, "success" | "muted" | "destructive"> = {
  active: "success",
  suspended: "muted",
  disabled: "destructive",
};

export const ClientsPage = () => {
  const t = useI18n((s) => s.t);
  const [clients, setClients] = useState<ClientInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<ClientInfo | null>(null);
  const [formName, setFormName] = useState("");
  const [formSlug, setFormSlug] = useState("");
  const [formMaxSessions, setFormMaxSessions] = useState("10");
  const [adminName, setAdminName] = useState("");
  const [adminEmail, setAdminEmail] = useState("");
  const [adminPassword, setAdminPassword] = useState("");
  const [error, setError] = useState("");
  const [orphans, setOrphans] = useState<SessionInfo[]>([]);
  const [orphanTargets, setOrphanTargets] = useState<Record<string, string>>({});
  const [assigning, setAssigning] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    try {
      const [clients, sessions] = await Promise.all([listClients(), listPlatformSessions()]);
      setClients(clients);
      setOrphans(sessions.filter((s) => !s.clientId));
    } catch {
      setClients([]);
      setOrphans([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, []);

  const resetForm = () => {
    setEditing(null);
    setFormName("");
    setFormSlug("");
    setFormMaxSessions("10");
    setAdminName("");
    setAdminEmail("");
    setAdminPassword("");
    setError("");
  };

  const openCreate = () => {
    resetForm();
    setShowForm(true);
  };

  const openEdit = (c: ClientInfo) => {
    setEditing(c);
    setFormName(c.name);
    setFormSlug(c.slug);
    setFormMaxSessions(String(c.maxSessions));
    setAdminName("");
    setAdminEmail("");
    setAdminPassword("");
    setError("");
    setShowForm(true);
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    const maxSessions = Math.max(0, parseInt(formMaxSessions || "0", 10));
    try {
      if (editing) {
        await updateClient(editing.id, { name: formName, maxSessions });
      } else {
        if (!formName || !adminEmail || !adminPassword) {
          setError("Name, admin email and password are required");
          return;
        }
        await createClient({
          name: formName,
          slug: formSlug || undefined,
          maxSessions,
          admin: { name: adminName, email: adminEmail, password: adminPassword },
        });
      }
      setShowForm(false);
      load();
    } catch (err) {
      const msg = (err as Error).message;
      if (msg.includes("409")) setError("Slug or admin email already exists");
      else if (msg.includes("422")) setError("Limit below current usage");
      else if (msg.includes("400")) setError("Invalid data");
      else setError("Error saving");
    }
  };

  const changeStatus = async (c: ClientInfo, status: ClientStatus) => {
    try {
      await setClientStatus(c.id, status);
      load();
    } catch (err) {
      setError((err as Error).message);
    }
  };

  const changeLimits = async (c: ClientInfo, maxSessions: string) => {
    const n = Math.max(0, parseInt(maxSessions || "0", 10));
    if (Number.isNaN(n)) return;
    try {
      await setClientLimits(c.id, n);
      load();
    } catch (err) {
      const msg = (err as Error).message;
      if (msg.includes("422")) setError("Limit below current usage");
      else setError((err as Error).message);
    }
  };

  const removeClient = async (c: ClientInfo) => {
    if (!confirm(`${t("delete")} ${c.name}?\n\n${t("delete_client_warn")}`)) return;
    try {
      await deleteClient(c.id);
      load();
    } catch {
      // ignore
    }
  };

  const assignOrphan = async (s: SessionInfo) => {
    const clientId = orphanTargets[s.id];
    if (!clientId) return;
    setAssigning(s.id);
    try {
      await assignSessionClient(s.id, clientId);
      setOrphanTargets((prev) => {
        const next = { ...prev };
        delete next[s.id];
        return next;
      });
      load();
    } catch (err) {
      const msg = (err as Error).message;
      if (msg.includes("409")) setError("Session limit reached or name already used in that client");
      else setError((err as Error).message);
    } finally {
      setAssigning(null);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("clients_management")}</h2>
        <Button size="sm" onClick={openCreate}>
          <Plus className="mr-1 h-4 w-4" />
          {t("new_client")}
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {orphans.length > 0 && (
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center gap-2">
              <Link2 className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-sm font-medium">
                {t("orphaned_sessions")} ({orphans.length})
              </CardTitle>
            </div>
            <p className="text-xs text-muted-foreground">{t("orphaned_sessions_desc")}</p>
          </CardHeader>
          <CardContent className="space-y-2">
            {orphans.map((s) => (
              <div key={s.id} className="flex flex-wrap items-center justify-between gap-2 rounded-md border px-3 py-2">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{s.name}</p>
                  <p className="truncate text-xs font-mono text-muted-foreground">{s.jid}</p>
                </div>
                <div className="flex items-center gap-2">
                  <select
                    value={orphanTargets[s.id] ?? ""}
                    onChange={(e) => setOrphanTargets((prev) => ({ ...prev, [s.id]: e.target.value }))}
                    className="h-8 rounded-md border border-input bg-transparent px-2 text-xs shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                  >
                    <option value="">{t("select_client")}…</option>
                    {clients.map((c) => (
                      <option key={c.id} value={c.id}>{c.name}</option>
                    ))}
                  </select>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!orphanTargets[s.id] || assigning === s.id}
                    onClick={() => assignOrphan(s)}
                  >
                    {assigning === s.id ? (
                      <Loader2 className="h-3 w-3 animate-spin" />
                    ) : (
                      <Link2 className="h-3 w-3" />
                    )}
                    <span className="ml-1">{t("assign")}</span>
                  </Button>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {showForm && (
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-medium">
                {editing ? t("edit_client") : t("new_client")}
              </CardTitle>
              <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => setShowForm(false)}>
                <X className="h-4 w-4" />
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            <form onSubmit={submit} className="space-y-3">
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="space-y-1">
                  <Label htmlFor="client-name">{t("client_name")}</Label>
                  <Input id="client-name" value={formName} onChange={(e) => setFormName(e.target.value)} required />
                </div>
                <div className="space-y-1">
                  <Label htmlFor="client-slug">{t("client_slug")}</Label>
                  <Input id="client-slug" value={formSlug} onChange={(e) => setFormSlug(e.target.value)} placeholder="auto" />
                </div>
              </div>
              <div className="space-y-1">
                <Label htmlFor="client-max">{t("max_sessions")}</Label>
                <Input
                  id="client-max"
                  type="number"
                  min={0}
                  value={formMaxSessions}
                  onChange={(e) => setFormMaxSessions(e.target.value)}
                />
              </div>
              {!editing && (
                <>
                  <div className="space-y-1">
                    <Label htmlFor="admin-email">{t("admin_email")}</Label>
                    <Input id="admin-email" type="email" value={adminEmail} onChange={(e) => setAdminEmail(e.target.value)} required />
                  </div>
                  <div className="space-y-1">
                    <Label htmlFor="admin-name">{t("admin_name")}</Label>
                    <Input id="admin-name" value={adminName} onChange={(e) => setAdminName(e.target.value)} />
                  </div>
                  <div className="space-y-1">
                    <Label htmlFor="admin-password">{t("admin_password")}</Label>
                    <Input id="admin-password" type="password" value={adminPassword} onChange={(e) => setAdminPassword(e.target.value)} required />
                  </div>
                </>
              )}
              <div className="flex gap-2">
                <Button type="submit" size="sm">{t("save")}</Button>
                <Button type="button" variant="outline" size="sm" onClick={() => setShowForm(false)}>{t("cancel")}</Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {clients.length === 0 ? (
        <EmptyState
          icon={<Building2 className="h-6 w-6" />}
          title={t("no_clients")}
          description={t("no_clients_desc")}
        />
      ) : (
        <div className="space-y-3">
          {clients.map((c) => (
            <div key={c.id} className="rounded-lg border px-4 py-3 transition-colors hover:bg-muted/50">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-3">
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-bold text-primary">
                    {c.name.charAt(0)?.toUpperCase()}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <p className="text-sm font-medium">{c.name}</p>
                      <Badge variant={statusVariant[c.status]}>{t(`status_${c.status}` as any)}</Badge>
                    </div>
                    <p className="text-xs text-muted-foreground font-mono">/{c.slug}</p>
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <span className="text-xs text-muted-foreground">
                    {t("sessions_used", { used: c.usedSessions, max: c.maxSessions })}
                  </span>
                  <div className="flex items-center gap-1">
                    <select
                      value={c.status}
                      onChange={(e) => changeStatus(c, e.target.value as ClientStatus)}
                      title={t("status")}
                      className="h-8 rounded-md border border-input bg-transparent px-2 text-xs shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                    >
                      <option value="active">{t("status_active")}</option>
                      <option value="suspended">{t("status_suspended")}</option>
                      <option value="disabled">{t("status_disabled")}</option>
                    </select>
                    <Input
                      type="number"
                      min={0}
                      defaultValue={c.maxSessions}
                      onBlur={(e) => {
                        const n = parseInt(e.target.value, 10);
                        if (!Number.isNaN(n) && n !== c.maxSessions) changeLimits(c, String(n));
                      }}
                      title={t("max_sessions")}
                      className="h-8 w-20 px-2 text-xs"
                    />
                    <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => openEdit(c)} title={t("edit_client")}>
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" onClick={() => removeClient(c)} title={t("delete")}>
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
