import { useEffect, useState } from "react";
import { Users as UsersIcon, Loader2, Pencil, Trash2, Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/shared/EmptyState";
import { useI18n } from "@/lib/i18n";
import { apiGet, apiPost, apiDelete } from "@/lib/api";
import { useAuth } from "@/stores/auth";
import { listClients } from "@/services/clients";
import type { ClientInfo } from "@/types/client";

interface User {
  id: number;
  email: string;
  name: string;
  role?: string;
  clientId?: string;
}

export const UsersPage = () => {
  const t = useI18n((s) => s.t);
  const currentUser = useAuth((s) => s.user);
  const [users, setUsers] = useState<User[]>([]);
  const [clients, setClients] = useState<ClientInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<User | null>(null);
  const [formEmail, setFormEmail] = useState("");
  const [formName, setFormName] = useState("");
  const [formPassword, setFormPassword] = useState("");
  const [formRole, setFormRole] = useState("client_admin");
  const [formClientId, setFormClientId] = useState("");
  const [error, setError] = useState("");

  const load = async () => {
    setLoading(true);
    try {
      const res = await apiGet<{ users: User[] }>("/api/users");
      setUsers(res.users ?? []);
    } catch {
      setUsers([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, []);

  useEffect(() => {
    void listClients()
      .then((c) => setClients(c))
      .catch(() => {});
  }, []);

  const openCreate = () => {
    setEditing(null);
    setFormEmail("");
    setFormName("");
    setFormPassword("");
    setFormRole("client_admin");
    setFormClientId("");
    setError("");
    setShowForm(true);
  };

  const openEdit = (u: User) => {
    setEditing(u);
    setFormEmail(u.email);
    setFormName(u.name);
    setFormPassword("");
    setFormRole(u.role ?? "client_admin");
    setFormClientId(u.clientId ?? "");
    setError("");
    setShowForm(true);
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      if (editing) {
        const body: { name?: string; password?: string } = {};
        if (formName !== editing.name) body.name = formName;
        if (formPassword) body.password = formPassword;
        if (Object.keys(body).length > 0) {
          await apiPost(`/api/users/${editing.id}`, body);
        }
      } else {
        if (!formEmail || !formPassword) {
          setError("Email y contraseña son obligatorios");
          return;
        }
        await apiPost("/api/users", {
          email: formEmail,
          name: formName,
          password: formPassword,
          role: formRole,
          clientId: formRole === "client_admin" && formClientId ? formClientId : undefined,
        });
      }
      setShowForm(false);
      load();
    } catch (err) {
      const msg = (err as Error).message;
      if (msg.includes("409")) setError("El email ya está registrado");
      else if (msg.includes("400")) setError("Datos inválidos");
      else setError("Error al guardar");
    }
  };

  const deleteUser = async (u: User) => {
    if (!confirm(`Eliminar usuario ${u.email}?`)) return;
    try {
      await apiDelete(`/api/users/${u.id}`);
      load();
    } catch {
      // ignore
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
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">{t("users_management")}</h2>
        <Button size="sm" onClick={openCreate}>
          <Plus className="mr-1 h-4 w-4" />
          {t("new_user")}
        </Button>
      </div>

      {showForm && (
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-medium">
                {editing ? t("edit_user") : t("new_user")}
              </CardTitle>
              <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => setShowForm(false)}>
                <X className="h-4 w-4" />
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            <form onSubmit={submit} className="space-y-3">
              {!editing && (
                <div className="space-y-1">
                  <Label htmlFor="email">Email</Label>
                  <Input id="email" type="email" value={formEmail} onChange={(e) => setFormEmail(e.target.value)} required />
                </div>
              )}
              <div className="space-y-1">
                <Label htmlFor="name">{t("name")}</Label>
                <Input id="name" value={formName} onChange={(e) => setFormName(e.target.value)} />
              </div>
              <div className="space-y-1">
                <Label htmlFor="password">
                  {editing ? t("new_password") : t("password")}
                  {editing && <span className="text-muted-foreground ml-1">({t("leave_empty_no_change")})</span>}
                </Label>
                <Input id="password" type="password" value={formPassword} onChange={(e) => setFormPassword(e.target.value)} required={!editing} />
              </div>
              {!editing && (
                <>
                  <div className="space-y-1">
                    <Label htmlFor="role">{t("role_label")}</Label>
                    <select
                      id="role"
                      value={formRole}
                      onChange={(e) => {
                        setFormRole(e.target.value);
                        if (e.target.value === "platform_admin") setFormClientId("");
                      }}
                      className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                    >
                      <option value="client_admin">{t("role_client")}</option>
                      <option value="platform_admin">{t("role_platform")}</option>
                    </select>
                  </div>
                  {formRole === "client_admin" && (
                    <div className="space-y-1">
                      <Label htmlFor="client">{t("client_for")}</Label>
                      <select
                        id="client"
                        value={formClientId}
                        onChange={(e) => setFormClientId(e.target.value)}
                        className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                      >
                        <option value="">{t("select_client")}</option>
                        {clients.map((c) => (
                          <option key={c.id} value={c.id}>{c.name}</option>
                        ))}
                      </select>
                    </div>
                  )}
                </>
              )}
              {error && <p className="text-sm text-destructive">{error}</p>}
              <div className="flex gap-2">
                <Button type="submit" size="sm">{t("save")}</Button>
                <Button type="button" variant="outline" size="sm" onClick={() => setShowForm(false)}>{t("cancel")}</Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {users.length === 0 ? (
        <EmptyState
          icon={<UsersIcon className="h-6 w-6" />}
          title={t("no_users")}
          description={t("no_users_desc")}
        />
      ) : (
        <div className="space-y-2">
          {users.map((u) => (
            <div key={u.id} className="flex items-center justify-between rounded-lg border px-4 py-3 transition-colors hover:bg-muted/50">
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-bold text-primary">
                  {u.name?.charAt(0)?.toUpperCase() || u.email.charAt(0).toUpperCase()}
                </div>
                <div>
                  <p className="text-sm font-medium">{u.name || u.email}</p>
                  <p className="text-xs text-muted-foreground">{u.email}</p>
                </div>
                {u.role && (
                  <Badge variant={u.role === "platform_admin" ? "secondary" : "muted"}>
                    {t(u.role === "platform_admin" ? "role_platform" : "role_client")}
                  </Badge>
                )}
                {currentUser?.id === u.id && (
                  <Badge variant="success">{t("you")}</Badge>
                )}
              </div>
              <div className="flex items-center gap-1">
                <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => openEdit(u)} title={t("edit_user")}>
                  <Pencil className="h-4 w-4" />
                </Button>
                {currentUser?.id !== u.id && (
                  <Button variant="ghost" size="icon" className="h-8 w-8 text-destructive" onClick={() => deleteUser(u)} title={t("delete")}>
                    <Trash2 className="h-4 w-4" />
                  </Button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
