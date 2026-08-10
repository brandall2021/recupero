import { create } from "zustand";

export type Locale = "en" | "es" | "pt";

type TranslationKeys = {
  // General
  app_name: string;
  no_accounts: string;
  no_accounts_desc: string;
  select_account: string;
  select_account_desc: string;
  save: string;
  // Session
  new_session: string;
  scan_qr: string;
  scan_qr_desc: string;
  linked: string;
  connecting: string;
  logout: string;
  // Channel token
  channel_credentials: string;
  channel_credentials_desc: string;
  channel_id_label: string;
  channel_token_label: string;
  copy: string;
  copied: string;
  // Calls
  calls_nav: string;
  dialer: string;
  call: string;
  calling: string;
  end_call: string;
  active_calls: string;
  no_active_calls: string;
  no_active_calls_desc: string;
  incoming_call: string;
  accept: string;
  reject: string;
  record: string;
  stop_recording: string;
  notes: string;
  add_note: string;
  // Devices
  devices: string;
  default_mic: string;
  default_speaker: string;
  // Quality
  quality: string;
  latency: string;
  jitter: string;
  packet_loss: string;
  bitrate: string;
  audio_flow: string;
  no_audio_flow: string;
  peer_audio_flow: string;
  no_peer_audio: string;
  // History
  history: string;
  call_history: string;
  no_history: string;
  duration: string;
  duration_min: string;
  missed: string;
  // Contacts
  contacts_nav: string;
  contacts: string;
  new_contact: string;
  edit_contact: string;
  search_contacts: string;
  no_contacts: string;
  no_results: string;
  name: string;
  phone: string;
  email: string;
  favorite: string;
  unfavorite: string;
  add: string;
  // Schedule
  schedule_nav: string;
  schedule: string;
  new_scheduled: string;
  upcoming: string;
  past: string;
  select_contact: string;
  no_schedule: string;
  mark_complete: string;
  cancel_schedule: string;
  pending: string;
  completed: string;
  cancelled: string;
  // Notes
  notes_nav: string;
  call_notes: string;
  new_note: string;
  edit_note: string;
  call_notes_placeholder: string;
  tags_placeholder: string;
  no_notes: string;
  // Recordings
  recordings_nav: string;
  recordings: string;
  no_recordings: string;
  no_recordings_desc: string;
  inbound: string;
  outbound: string;
  // Dashboard
  dashboard_nav: string;
  dashboard: string;
  active_sessions: string;
  active_calls_dashboard: string;
  avg_duration: string;
  recorded: string;
  users: string;
  registered: string;
  uptime: string;
  total: string;
  loading: string;
  // Users
  users_nav: string;
  users_management: string;
  new_user: string;
  edit_user: string;
  new_password: string;
  leave_empty_no_change: string;
  no_users: string;
  no_users_desc: string;
  you: string;
  delete: string;
  cancel: string;
  password: string;
  // Footer
  accounts: string;
  theme: string;
  language: string;
};

const translations: Record<Locale, TranslationKeys> = {
  en: {
    app_name: "WaCalls",
    no_accounts: "No accounts yet",
    no_accounts_desc: "Create your first WhatsApp account from the sidebar to start calling.",
    select_account: "Select an account",
    select_account_desc: "Choose an account from the sidebar.",
    save: "Save",
    new_session: "New session",
    scan_qr: "Scan QR code",
    scan_qr_desc: "Open WhatsApp on your phone, go to Linked Devices and scan this code.",
    linked: "Linked",
    connecting: "Connecting…",
    logout: "Log out",
    channel_credentials: "Channel credentials",
    channel_credentials_desc: "External API credentials for this channel. Keep the token private.",
    channel_id_label: "Channel ID",
    channel_token_label: "Channel token",
    copy: "Copy",
    copied: "Copied",
    calls_nav: "Calls",
    dialer: "Dialer",
    call: "Call",
    calling: "Calling…",
    end_call: "End call",
    active_calls: "{n} active call(s)",
    no_active_calls: "No active calls",
    no_active_calls_desc: "Dial a number above to start a call.",
    incoming_call: "Incoming call",
    accept: "Accept",
    reject: "Reject",
    record: "Record",
    stop_recording: "Stop recording",
    notes: "Notes",
    add_note: "Add note…",
    devices: "Devices",
    default_mic: "Default mic",
    default_speaker: "Default speaker",
    quality: "Quality",
    latency: "Latency",
    jitter: "Jitter",
    packet_loss: "Packet loss",
    bitrate: "Bitrate",
    audio_flow: "Mic OK",
    no_audio_flow: "No mic",
    peer_audio_flow: "Peer OK",
    no_peer_audio: "No peer audio",
    history: "History",
    call_history: "Call history",
    no_history: "No calls yet",
    duration: "Duration",
    duration_min: "Duration (min)",
    missed: "Missed",
    contacts_nav: "Contacts",
    contacts: "Contacts",
    new_contact: "New contact",
    edit_contact: "Edit contact",
    search_contacts: "Search contacts…",
    no_contacts: "No contacts yet",
    no_results: "No results found",
    name: "Name",
    phone: "Phone",
    email: "Email",
    favorite: "Favorite",
    unfavorite: "Unfavorite",
    add: "Add",
    schedule_nav: "Schedule",
    schedule: "Schedule",
    new_scheduled: "Schedule call",
    upcoming: "Upcoming",
    past: "Past",
    select_contact: "Select contact",
    no_schedule: "No scheduled calls",
    mark_complete: "Mark complete",
    cancel_schedule: "Cancel",
    pending: "Pending",
    completed: "Completed",
    cancelled: "Cancelled",
    notes_nav: "Notes",
    call_notes: "Call Notes",
    new_note: "New note",
    edit_note: "Edit note",
    call_notes_placeholder: "Write your notes about this call…",
    tags_placeholder: "Tags (comma separated)",
    no_notes: "No notes yet",
    recordings_nav: "Recordings",
    recordings: "Recordings",
    no_recordings: "No recordings yet",
    no_recordings_desc: "Recordings will appear here after calls with recording enabled.",
    inbound: "Inbound",
    outbound: "Outbound",
    dashboard_nav: "Dashboard",
    dashboard: "Dashboard",
    active_sessions: "Active sessions",
    active_calls_dashboard: "Active calls",
    avg_duration: "Avg duration",
    recorded: "recorded",
    users: "Users",
    registered: "registered",
    uptime: "Uptime",
    total: "Total",
    loading: "Loading…",
    users_nav: "Users",
    users_management: "Users",
    new_user: "New user",
    edit_user: "Edit user",
    new_password: "New password",
    leave_empty_no_change: "leave empty to keep current",
    no_users: "No users",
    no_users_desc: "Create the first user from the button above.",
    you: "You",
    delete: "Delete",
    cancel: "Cancel",
    password: "Password",
    accounts: "Accounts",
    theme: "Theme",
    language: "Language",
  },
  es: {
    app_name: "WaCalls",
    no_accounts: "Sin cuentas aún",
    no_accounts_desc: "Creá tu primera cuenta de WhatsApp desde la barra lateral para empezar a llamar.",
    select_account: "Elegí una cuenta",
    select_account_desc: "Elegí una cuenta de la barra lateral.",
    save: "Guardar",
    new_session: "Nueva sesión",
    scan_qr: "Escaneá el código QR",
    scan_qr_desc: "Abrí WhatsApp en tu celular, andá a Dispositivos vinculados y escaneá este código.",
    linked: "Vinculado",
    connecting: "Conectando…",
    logout: "Cerrar sesión",
    channel_credentials: "Credenciales del canal",
    channel_credentials_desc: "Credenciales de la API externa de este canal. Mantené el token en privado.",
    channel_id_label: "ID del canal",
    channel_token_label: "Token del canal",
    copy: "Copiar",
    copied: "Copiado",
    calls_nav: "Llamadas",
    dialer: "Marcador",
    call: "Llamar",
    calling: "Llamando…",
    end_call: "Colgar",
    active_calls: "{n} llamada(s) activa(s)",
    no_active_calls: "Sin llamadas activas",
    no_active_calls_desc: "Marcá un número arriba para iniciar una llamada.",
    incoming_call: "Llamada entrante",
    accept: "Aceptar",
    reject: "Rechazar",
    record: "Grabar",
    stop_recording: "Detener grabación",
    notes: "Notas",
    add_note: "Agregar nota…",
    devices: "Dispositivos",
    default_mic: "Micrófono predeterminado",
    default_speaker: "Parlante predeterminado",
    quality: "Calidad",
    latency: "Latencia",
    jitter: "Jitter",
    packet_loss: "Pérdida de paquetes",
    bitrate: "Bitrate",
    audio_flow: "Mic OK",
    no_audio_flow: "Sin mic",
    peer_audio_flow: "Par OK",
    no_peer_audio: "Sin audio par",
    history: "Historial",
    call_history: "Historial de llamadas",
    no_history: "Sin llamadas aún",
    duration: "Duración",
    duration_min: "Duración (min)",
    missed: "Perdida",
    contacts_nav: "Contactos",
    contacts: "Contactos",
    new_contact: "Nuevo contacto",
    edit_contact: "Editar contacto",
    search_contacts: "Buscar contactos…",
    no_contacts: "Sin contactos aún",
    no_results: "Sin resultados",
    name: "Nombre",
    phone: "Teléfono",
    email: "Email",
    favorite: "Favorito",
    unfavorite: "Quitar de favoritos",
    add: "Agregar",
    schedule_nav: "Agenda",
    schedule: "Agendar",
    new_scheduled: "Agendar llamada",
    upcoming: "Próximas",
    past: "Pasadas",
    select_contact: "Seleccionar contacto",
    no_schedule: "Sin llamadas agendadas",
    mark_complete: "Marcar completa",
    cancel_schedule: "Cancelar",
    pending: "Pendiente",
    completed: "Completada",
    cancelled: "Cancelada",
    notes_nav: "Notas",
    call_notes: "Notas de llamadas",
    new_note: "Nueva nota",
    edit_note: "Editar nota",
    call_notes_placeholder: "Escribí tus notas sobre esta llamada…",
    tags_placeholder: "Etiquetas (separadas por coma)",
    no_notes: "Sin notas aún",
    recordings_nav: "Grabaciones",
    recordings: "Grabaciones",
    no_recordings: "Sin grabaciones aún",
    no_recordings_desc: "Las grabaciones aparecerán aquí después de llamadas con grabación habilitada.",
    inbound: "Entrante",
    outbound: "Saliente",
    dashboard_nav: "Panel",
    dashboard: "Panel de control",
    active_sessions: "Sesiones activas",
    active_calls_dashboard: "Llamadas activas",
    avg_duration: "Duración promedio",
    recorded: "grabadas",
    users: "Usuarios",
    registered: "registrados",
    uptime: "Actividad",
    total: "Total",
    loading: "Cargando…",
    users_nav: "Usuarios",
    users_management: "Usuarios",
    new_user: "Nuevo usuario",
    edit_user: "Editar usuario",
    new_password: "Nueva contraseña",
    leave_empty_no_change: "dejar vacío para mantener la actual",
    no_users: "Sin usuarios",
    no_users_desc: "Creá el primer usuario desde el botón de arriba.",
    you: "Vos",
    delete: "Eliminar",
    cancel: "Cancelar",
    password: "Contraseña",
    accounts: "Cuentas",
    theme: "Tema",
    language: "Idioma",
  },
  pt: {
    app_name: "WaCalls",
    no_accounts: "Nenhuma conta ainda",
    no_accounts_desc: "Crie sua primeira conta WhatsApp na barra lateral para começar a ligar.",
    select_account: "Selecione uma conta",
    select_account_desc: "Escolha uma conta na barra lateral.",
    save: "Salvar",
    new_session: "Nova sessão",
    scan_qr: "Escaneie o código QR",
    scan_qr_desc: "Abra o WhatsApp no celular, vá para Dispositivos vinculados e escaneie este código.",
    linked: "Vinculado",
    connecting: "Conectando…",
    logout: "Sair",
    channel_credentials: "Credenciais do canal",
    channel_credentials_desc: "Credenciais da API externa deste canal. Mantenha o token em sigilo.",
    channel_id_label: "ID do canal",
    channel_token_label: "Token do canal",
    copy: "Copiar",
    copied: "Copiado",
    calls_nav: "Chamadas",
    dialer: "Discador",
    call: "Ligar",
    calling: "Chamando…",
    end_call: "Encerrar",
    active_calls: "{n} chamada(s) ativa(s)",
    no_active_calls: "Sem chamadas ativas",
    no_active_calls_desc: "Digite um número acima para iniciar uma chamada.",
    incoming_call: "Chamada recebida",
    accept: "Atender",
    reject: "Rejeitar",
    record: "Gravar",
    stop_recording: "Parar gravação",
    notes: "Notas",
    add_note: "Adicionar nota…",
    devices: "Dispositivos",
    default_mic: "Microfone padrão",
    default_speaker: "Alto-falante padrão",
    quality: "Qualidade",
    latency: "Latência",
    jitter: "Jitter",
    packet_loss: "Perda de pacotes",
    bitrate: "Bitrate",
    audio_flow: "Mic OK",
    no_audio_flow: "Sem mic",
    peer_audio_flow: "Par OK",
    no_peer_audio: "Sem áudio par",
    history: "Histórico",
    call_history: "Histórico de chamadas",
    no_history: "Nenhuma chamada ainda",
    duration: "Duração",
    duration_min: "Duração (min)",
    missed: "Perdida",
    contacts_nav: "Contatos",
    contacts: "Contatos",
    new_contact: "Novo contato",
    edit_contact: "Editar contato",
    search_contacts: "Buscar contatos…",
    no_contacts: "Nenhum contato ainda",
    no_results: "Sem resultados",
    name: "Nome",
    phone: "Telefone",
    email: "Email",
    favorite: "Favorito",
    unfavorite: "Remover dos favoritos",
    add: "Adicionar",
    schedule_nav: "Agenda",
    schedule: "Agendar",
    new_scheduled: "Agendar chamada",
    upcoming: "Próximas",
    past: "Passadas",
    select_contact: "Selecionar contato",
    no_schedule: "Sem chamadas agendadas",
    mark_complete: "Marcar concluída",
    cancel_schedule: "Cancelar",
    pending: "Pendente",
    completed: "Concluída",
    cancelled: "Cancelada",
    notes_nav: "Notas",
    call_notes: "Notas de chamadas",
    new_note: "Nova nota",
    edit_note: "Editar nota",
    call_notes_placeholder: "Escreva suas notas sobre esta chamada…",
    tags_placeholder: "Tags (separadas por vírgula)",
    no_notes: "Nenhuma nota ainda",
    recordings_nav: "Gravações",
    recordings: "Gravações",
    no_recordings: "Nenhuma gravação ainda",
    no_recordings_desc: "As gravações aparecerão aqui após chamadas com gravação habilitada.",
    inbound: "Recebida",
    outbound: "Enviada",
    dashboard_nav: "Painel",
    dashboard: "Painel de controle",
    active_sessions: "Sessões ativas",
    active_calls_dashboard: "Chamadas ativas",
    avg_duration: "Duração média",
    recorded: "gravadas",
    users: "Usuários",
    registered: "cadastrados",
    uptime: "Atividade",
    total: "Total",
    loading: "Carregando…",
    users_nav: "Usuários",
    users_management: "Usuários",
    new_user: "Novo usuário",
    edit_user: "Editar usuário",
    new_password: "Nova senha",
    leave_empty_no_change: "deixe vazio para manter a atual",
    no_users: "Sem usuários",
    no_users_desc: "Crie o primeiro usuário no botão acima.",
    you: "Você",
    delete: "Excluir",
    cancel: "Cancelar",
    password: "Senha",
    accounts: "Contas",
    theme: "Tema",
    language: "Idioma",
  },
};

type State = {
  locale: Locale;
  t: (key: keyof TranslationValues, params?: Record<string, string | number>) => string;
  setLocale: (l: Locale) => void;
};

type TranslationValues = TranslationKeys;

export const useI18n = create<State>((set, get) => ({
  locale: (localStorage.getItem("wacalls:locale") as Locale) || "en",
  setLocale: (l) => {
    localStorage.setItem("wacalls:locale", l);
    set({ locale: l });
  },
  t: (key, params) => {
    const { locale } = get();
    let str = translations[locale]?.[key] ?? translations.en[key] ?? key;
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        str = str.replace(new RegExp(`\\{${k}\\}`, "g"), String(v));
      }
    }
    return str;
  },
}));
