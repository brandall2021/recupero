<div align="center">

# 📞 WaCalls

**Llamadas de voz nativas de WhatsApp desde el navegador.**
VoIP nativo, multi-cuenta, multi-sesión, con cliente moderno en React.

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](https://react.dev)
[![whatsmeow](https://img.shields.io/badge/whatsmeow-VoIP-25D366?logo=whatsapp&logoColor=white)](https://github.com/tulir/whatsmeow)
[![pion](https://img.shields.io/badge/pion-WebRTC-FF6B6B)](https://github.com/pion/webrtc)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](#licencia)

</div>

---

## Resumen

WaCalls vincula una o más cuentas de WhatsApp mediante **código QR** y permite
**realizar y recibir llamadas de voz 1:1** desde cualquier navegador en la red local.
El micrófono del navegador se envía como **PCM raw de 16 kHz por un canal de datos
WebRTC** al servidor Go, que lo codifica con el códec **MLow** de Meta e inyecta
el medio en el **relé SRTP** de WhatsApp — y el camino inverso devuelve el audio
del interlocutor al navegador.

Todo el stack VoIP corre **de forma nativa en Go puro**: el códec de voz MLow,
**RTP/SRTP**, **STUN**, el transporte **WebRTC/SCTP** y la señalización `<call>`,
integrados con [**whatsmeow**](https://github.com/tulir/whatsmeow) y servidos a
un cliente **React 19**. No hay **cgo ni DLLs nativas** — el códec MLow es un
paquete Go puro vendoreado, así que un `go build` simple produce un binario
autocontenido con audio en vivo.

Se pueden vincular y operar múltiples cuentas de WhatsApp en paralelo, cada una con su
propio QR de vinculación, estado de conexión e historial. Una sola cuenta también puede
mantener **varias llamadas 1:1 simultáneas** — una por cada operador del navegador —
enrutadas independientemente por ID de llamada.

> **Estado:** estable. Llamadas salientes y entrantes 1:1 alcanzan `ACTIVE` con audio
> bidireccional, grabación server-side WAV, autenticación JWT, y PostgreSQL para
> persistencia de sesiones, usuarios y grabaciones.

---

## Funcionalidades

### 🔐 Autenticación
- Registro e inicio de sesión con email + contraseña
- JWT (HS256, expiración 72h) para todas las rutas protegidas
- Ruta `GET /api/auth/me` para obtener el usuario actual
- Usuarios de prueba precargados (seed automático al iniciar — se resetean en cada reinicio):
  - `admin@wacalls.com` / `admin123` — Administrador
  - `operador@wacalls.com` / `operador123` — Operador
  - `demo@wacalls.com` / `demo123` — Demo
- Variable de entorno `JWT_SECRET` para firmar tokens (default: `wacalls-default-secret-change-me`)

### 👤 Gestión de usuarios
- CRUD completo: listar, crear, editar, eliminar usuarios
- Cambio de contraseña (reset) desde el panel de administración
- Edición de nombre
- El usuario actual no puede eliminarse a sí mismo
- API: `GET /api/users`, `POST /api/users`, `PUT /api/users/{id}`, `DELETE /api/users/{id}`
- Panel accesible desde la barra lateral (icono Shield)

### 🎙️ Grabación de llamadas (server-side)
- Grabación automática de todas las llamadas (salientes y entrantes)
- Formato WAV — 16 kHz mono PCM, codificación estándar
- Captura audio del micrófono del navegador + audio remoto del interlocutor
- Archivos WAV guardados en `/data/recordings` (volumen Docker)
- Tabla PostgreSQL `recordings` con metadata (session_id, call_id, peer, direction, duration, file_path, file_size)
- API de descarga: `GET /api/recordings/{id}/download?token=<jwt>` (token via query param para descarga directa desde el navegador)
- Página de grabaciones en el frontend con lista y botón de descarga

### 👥 Contactos
- ABM completo de contactos (alta, baja, modificación)
- Campos: nombre, teléfono, email, notas
- Marcar contactos como favoritos con estrella
- Búsqueda por nombre, teléfono o email
- Ordenados por favoritos primero, luego por fecha de creación
- Persistencia en localStorage

### 📅 Agenda de llamadas
- Programar llamadas para fecha y hora específica
- Selección de contacto de la agenda
- Duración estimada en minutos
- Notas asociadas a cada llamada programada
- Estados: pendiente, completada, cancelada
- Vista separada de próximas y pasadas
- Persistencia en localStorage

### 📝 Notas por llamada
- Agregar notas a cualquier llamada desde la tarjeta de llamada activa
- Rating con estrellas (1-5) por llamada
- Tags personalizados (separados por coma)
- Texto libre de notas
- Vista historial de todas las notas ordenadas por fecha
- Edición y eliminación de notas existentes
- Persistencia en localStorage

### 🌐 Multiidioma
- Soporte completo para **Español (ES)**, **Portugués (PT)** e **Inglés (EN)**
- Selector de idioma persistente con `localStorage`
- Todas las funcionalidades traducidas incluidas las nuevas

### 🔊 Sonidos de llamada
- Tono de llamada entrante (Web Audio API)
- Tono de ocupado
- Tono de desconexión
- Sin archivos externos, generados programáticamente

### 📊 Métricas de calidad
- Latencia (RTT) en tiempo real
- Jitter
- Pérdida de paquetes
- Bitrate en kbps
- Indicador visual de calidad (señal alta/media/baja)

### 🔍 Diagnóstico de audio
- Indicadores visuales en la tarjeta de llamada: **Mic OK** / **Sin mic** y **Par OK** / **Sin audio par**
- Buffer de PCM mientras el relay o SRTP se conectan (máx. 2 segundos)
- Logs detallados en el servidor: codec, rtpSession, srtpStatus, relay, frames enviados
- Contadores de diagnóstico: `totalPCMRecv`, `totalFramesSent`, `totalRelayRecv`
- Flush automático del buffer cuando el relay se conecta

---

## Arquitectura

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    NAVEGADOR (React client)                              │
│   mic + speaker · WebRTC data channel (16 kHz PCM) · HTTP + SSE         │
│   + Grabación · Contactos · Agenda · Notas (localStorage)               │
│   + Auth (JWT en localStorage)                                          │
└───────────────────────────────┬──────────────────────────────────────────┘
                                │  POST /api/sessions/{sid}/calls/{id}/webrtc
                                │  GET  /api/events?token=...
                                │  Authorization: Bearer <token>
                                ▼
┌────────────────────────── SERVIDOR GO (cmd/server) ──────────────────────┐
│  SessionManager   registro de cuentas (client + CallManager + bridge)    │
│  Broker           hub SSE (sesiones, auth, ciclo de vida de llamadas)    │
│  Bridge           puente pion WebRTC (PCM 16 kHz ⇄ call core)          │
│  AuthStore        usuarios + JWT (bcrypt + HS256)                       │
│  RecordingStore   grabaciones WAV server-side (PostgreSQL)              │
│  PostgreSQL       sesiones, usuarios, grabaciones                        │
│                                                                          │
│  internal/wa      adaptador VoipSocket sobre whatsmeow                   │
│  internal/voip    call · signaling · media · transport · core · wanode   │
│  internal/recording  WAV writer 16 kHz mono PCM                         │
└───────────────┬──────────────────────────────────────┬───────────────────┘
                │ señalización <call> (Signal/USync)    │ SRTP media
                ▼                                       ▼
        ┌───────────────┐                    ┌──────────────────────┐
        │  WhatsApp WS  │                    │   Relé WhatsApp      │
        │  (whatsmeow)  │                    │  (SRTP over SCTP/DC) │
        └───────────────┘                    └──────────────────────┘
```

### Estructura de archivos

| Ruta | Responsabilidad |
|---|---|
| `cmd/server` | Broker HTTP/SSE, gestor de sesiones, puente WebRTC, auth, grabaciones |
| `cmd/server/auth.go` | Store de usuarios (PostgreSQL), bcrypt, JWT, handlers login/register/me |
| `cmd/server/auth_middleware.go` | Middleware `withAuth` — valida JWT en todas las rutas protegidas |
| `cmd/server/dashboard.go` | Endpoint `GET /api/dashboard` — stats agregadas + historial reciente |
| `cmd/server/recordingstore.go` | Store de grabaciones PostgreSQL |
| `internal/recording` | WAV writer 16 kHz mono PCM, header finalization |
| `internal/wa` | `VoipSocket` — envía/recibe stanzas `<call>` vía whatsmeow |
| `internal/voip/core` | Tipos de dominio, constantes, interfaz `VoipSocket` |
| `internal/voip/wanode` | Helpers compartidos de nodo WhatsApp y JID |
| `internal/voip/media` | Códec MLow (Go puro), RTP, SRTP, SSRC, PCM, derivación de claves |
| `internal/voip/transport` | Relé SCTP, STUN, codificación de suscripciones |
| `internal/voip/signaling` | Build/parse de stanzas `<call>`, crypto de claves de llamada |
| `internal/voip/call` | `CallManager` — orquesta una llamada de principio a fin |
| `client/` | React 19 + Vite + Tailwind v4 + shadcn/ui |
| `client/src/pages/DashboardPage.tsx` | Panel de control — stats, sesiones, historial reciente |

### Stores del cliente (Zustand + localStorage)

| Store | Datos |
|---|---|
| `stores/auth.ts` | Token JWT + usuario (email, name) |
| `stores/contacts.ts` | Contactos con CRUD, favoritos, búsqueda |
| `stores/schedule.ts` | Llamadas programadas con estados |
| `stores/callNotes.ts` | Notas por llamada con rating y tags |
| `stores/calls.ts` | Estado de llamadas activas (del servidor) |
| `stores/sessions.ts` | Sesiones de WhatsApp (del servidor) |
| `stores/devices.ts` | Dispositivos de audio del navegador |
| `stores/theme.ts` | Tema claro/oscuro |

> **Nota:** el dashboard carga datos directamente via `GET /api/dashboard` (no usa store Zustand).

---

## Flujo de una llamada

El núcleo es `internal/voip/call.CallManager`, que maneja una llamada de
principio a fin. Secuencia de llamada saliente:

```
1. POST .../calls            → CallManager.StartCall(peerJid)
                               genera un callID, construye la oferta <call>, la envía

2. Navegador abre WebRTC     → POST .../calls/{id}/webrtc (oferta SDP)
                               el puente responde con una respuesta SDP (pion)

3. Par acepta               → events.CallAccept → HandleCallAccept
                               servidor recibe <relay> + claves hop-by-hop

4. Transporte relay          → binding/allocate STUN en relés de WhatsApp
                               ICE + DTLS + SCTP DataChannel conectan (pion)

5. SRTP fluyendo             → el estado pasa a ACTIVE
   ├── subida   (vos → par): PCM 16 kHz del navegador (data channel) → MLow encode → SRTP → relay
   └── bajada   (par → vos): relay → SRTP → MLow decode → PCM 16 kHz (data channel) → navegador

6. Grabación server-side     → WAV 16 kHz mono PCM → /data/recordings

7. Teardown                  → DELETE .../calls/{id} o events.CallTerminate
                               CallManager.EndCall + limpieza del puente
```

---

## Requisitos

- **Go 1.26+**
- **Node 22+** y **npm** (solo para compilar/ejecutar el cliente React)
- **PostgreSQL** (para sesiones, usuarios y grabaciones)

No se necesita compilador C, cgo ni bibliotecas nativas — el códec MLow es Go
puro vendoreado (`internal/voip/media/mlow`).

---

## Inicio rápido

```bash
# clonar y entrar al proyecto
git clone <url-del-repo> wacalls
cd wacalls

# dependencias de Go
go mod download

# dependencias del cliente React
cd client && npm install && cd ..
```

### Variables de entorno

| Variable | Requerida | Descripción |
|---|---|---|
| `DATABASE_URL` | Sí | URL de conexión PostgreSQL (ej: `postgresql://user:pass@host:5432/dbname?sslmode=disable`) |
| `JWT_SECRET` | No | Secreto para firmar tokens JWT (default: `wacalls-default-secret-change-me`) |

### Ejecutar

```bash
export DATABASE_URL="postgresql://brandall:pass@host:5432/wacall2?sslmode=disable"
go run ./cmd/server -addr :8080          # agregar -debug para logs verbosos
```

El audio en vivo funciona directamente — el códec MLow es Go puro, así que una
compilación simple lo incluye. Sin build tags, sin `CGO_ENABLED`, sin DLLs.

Abrí `http://localhost:8080`, registrate o iniciá sesión con las credenciales
del seed (ej: `admin@wacalls.com` / `admin123`), hacé clic en **New session** y
escaneá el QR que aparece en el navegador con **WhatsApp → Dispositivos vinculados**.

### Cliente React en modo desarrollo

```bash
cd client
npm run dev      # Vite en :5173, proxea /api → http://localhost:8080
```

Para producción, compilá el cliente estático y servilo desde el servidor Go:

```bash
cd client && npm run build && cd ..
go run ./cmd/server -static client/dist -addr :8080
```

### Docker

```bash
docker build -t wacalls .
docker run -e DATABASE_URL="postgresql://..." -e JWT_SECRET="mi-secreto" -p 8080:8080 wacalls
```

### Flags del servidor

| Flag | Valor por defecto | Descripción |
|---|---|---|
| `-addr` | `:8080` | Dirección de escucha HTTP |
| `-database-url` | (requerido) | URL de conexión PostgreSQL (o usar `DATABASE_URL`) |
| `-static` | `client/dist` | Directorio del cliente estático (opcional) |
| `-debug` | `false` | Logging verboso (incluye el log interno de whatsmeow) |
| `-max-calls-per-session` | `8` | Máximo de llamadas concurrentes por sesión (`0` = sin límite) |

---

## API

### Autenticación (rutas públicas)

| Método | Ruta | Propósito |
|---|---|---|
| `POST` | `/api/auth/register` | Crear cuenta (`{ email, name, password }`) |
| `POST` | `/api/auth/login` | Iniciar sesión (`{ email, password }`) |
| `GET` | `/api/auth/me` | Obtener usuario actual (requiere `Authorization: Bearer <token>`) |

### Sesiones (requiere JWT)

Todas las rutas requieren header `Authorization: Bearer <token>`.

| Método | Ruta | Propósito |
|---|---|---|
| `GET` | `/api/sessions` | Listar cuentas (id, nombre, jid, estado, vinculada) |
| `POST` | `/api/sessions` | Crear una cuenta e iniciar vinculación por QR |
| `DELETE` | `/api/sessions/{sid}` | Cerrar sesión y eliminar una cuenta |
| `POST` | `/api/sessions/{sid}/logout` | Desconectar una cuenta (mantener para re-vinculación) |
| `POST` | `/api/sessions/{sid}/pair` | Re-vincular una cuenta (emitir QR nuevo) |
| `POST` | `/api/sessions/{sid}/calls` | Iniciar llamada saliente (`{ phone }`) |
| `POST` | `/api/sessions/{sid}/calls/{id}/webrtc` | Intercambiar SDP WebRTC del navegador |
| `POST` | `/api/sessions/{sid}/calls/{id}/accept` | Aceptar llamada entrante |
| `POST` | `/api/sessions/{sid}/calls/{id}/reject` | Rechazar llamada entrante |
| `DELETE` | `/api/sessions/{sid}/calls/{id}` | Finalizar llamada activa |
| `GET` | `/api/sessions/{sid}/history` | Historial de llamadas recientes (hasta 50 registros) |
| `GET` | `/api/sessions/{sid}/recordings` | Listar grabaciones de la sesión |
| `GET` | `/api/recordings/{id}/download` | Descargar archivo WAV (`?token=<jwt>`) |
| `GET` | `/api/dashboard` | Dashboard: stats agregadas + sesiones + llamadas recientes |
| `GET` | `/api/users` | Listar usuarios |
| `POST` | `/api/users` | Crear usuario (`{ email, name, password }`) |
| `PUT` | `/api/users/{id}` | Actualizar usuario (`{ name?, password? }`) |
| `DELETE` | `/api/users/{id}` | Eliminar usuario |
| `GET` | `/api/events` | Eventos server-sent (`?token=<jwt>&clientId=<id>`) |

---

## Dashboard

Al iniciar sesión se muestra el **panel de control** con:

- **8 cards de resumen**: sesiones activas, llamadas activas, grabaciones (count + tamaño), duración promedio, llamadas entrantes, salientes, usuarios registrados, uptime
- **Sesiones**: lista de cuentas WhatsApp con estado (vinculada/no vinculada)
- **Historial reciente**: últimas 20 llamadas con dirección, peer, duración, status y reason
- Auto-refresh cada 10 segundos
- Traducciones en/es/pt
- **Página por defecto** al iniciar sesión

---

## Navegación del cliente

El cliente tiene 7 secciones accesibles desde la barra lateral:

| Sección | Ícono | Descripción |
|---|---|---|
| **Dashboard** | 📊 | Panel de control con estadísticas y actividad reciente |
| **Calls** | 📞 | Marcador, llamadas activas, calidad, notas |
| **Contacts** | 👥 | ABM de contactos con favoritos y búsqueda |
| **Schedule** | 📅 | Agenda de llamadas programadas |
| **Notes** | 📝 | Historial de notas con rating y tags |
| **Recordings** | 🎙️ | Lista de grabaciones con descarga |
| **Users** | 🛡️ | Gestión de usuarios (CRUD, reset contraseña) |

---

## Persistencia

| Store | Base de datos | Contenido |
|---|---|---|
| `sessions` | PostgreSQL | Cuentas WhatsApp vinculadas (id, name, jid) |
| `users` | PostgreSQL | Usuarios del sistema (email, name, password bcrypt) |
| `recordings` | PostgreSQL | Metadata de grabaciones WAV |
| `/data/recordings/` | Disco | Archivos WAV de grabaciones |
| whatsmeow store | PostgreSQL | Estado de sesiones WhatsApp (cifrado) |
| localStorage | Navegador | Contactos, agenda, notas, preferencias, token JWT |

---

## Tests

```bash
go test ./...                 # stack de media: SRTP, STUN, RTP, relay-ack, códec, estado
cd client && npm run build    # type-check del cliente + build de producción
```

---

## Seguridad

La API utiliza **JWT** para autenticación — todas las rutas `/api/*` (excepto
`/api/auth/login` y `/api/auth/register`) requieren un token válido.

- Los tokens JWT expiran a las 72 horas
- Las contraseñas se almacenan con **bcrypt**
- Las rutas de login/register son públicas (no envían token)
- El EventSource (SSE) no se conecta sin token válido
- Configurá `JWT_SECRET` en producción para firmar tokens con un secreto seguro
- PostgreSQL contiene credenciales de sesión de WhatsApp (secretos): **no lo subas a
  un repositorio** y mantenlo protegido

---

## Solución de problemas de audio

Si el otro teléfono no escucha audio, revisá los logs del servidor con `-debug`:

| Mensaje de log | Significado | Solución |
|---|---|---|
| `codec nil` | El códec MLow no se inicializó | Verificar que `internal/voip/media/mlow/` esté compilado |
| `srtpSession nil, buffering` | Las claves SRTP no se derivaron | Revisar que la llamada tenga `EncryptionKey` y `ParticipantJids` |
| `relay not connected, buffering` | El relay de WhatsApp no conectó | Verificar conectividad de red (ICE/DTLS a puertos 3478) |
| `srtp protect error` | Error al encriptar el paquete | Las claves SRTP no coinciden con el peer |
| `audio frame sent` | Audio fluyendo correctamente | Todo funciona, revisar del lado del peer |
| `flushing buffered audio` | Buffer liberado tras conectar relay | Funcionando, el delay inicial es normal |

En el navegador, los indicadores muestran:
- **Mic OK** (verde) = el micrófono está capturando audio
- **Sin mic** (amarillo) = el navegador no tiene permiso de micrófono o no hay dispositivo
- **Par OK** (verde) = se está recibiendo audio del interlocutor
- **Sin audio par** (amarillo) = no se recibe audio del relay

---

## Contribuidores

Este proyecto se construye sobre el trabajo de:

<div align="center">

<a href="https://github.com/jotadev66"><img src="https://github.com/jotadev66.png" width="72" height="72" style="border-radius:50%" alt="jotadev66"/></a>
<a href="https://github.com/jobasfernandes"><img src="https://github.com/jobasfernandes.png" width="72" height="72" style="border-radius:50%" alt="jobasfernandes"/></a>
<a href="https://github.com/edgardmessias"><img src="https://github.com/edgardmessias.png" width="72" height="72" style="border-radius:50%" alt="edgardmessias"/></a>
<a href="https://github.com/w3nder"><img src="https://github.com/w3nder.png" width="72" height="72" style="border-radius:50%" alt="w3nder"/></a>

[**@jotadev66**](https://github.com/jotadev66) · [**@jobasfernandes**](https://github.com/jobasfernandes) · [**@edgardmessias**](https://github.com/edgardmessias) · [**@w3nder**](https://github.com/w3nder)

</div>

---

## Agradecimientos

- [**whatsmeow**](https://github.com/tulir/whatsmeow) — Librería Go para el protocolo WhatsApp Web
- [**pion/webrtc**](https://github.com/pion/webrtc) — Stack WebRTC en Go puro (ICE + DTLS + SCTP)
- [**whatsapp-rust**](https://github.com/oxidezap/whatsapp-rust) — Implementación de referencia del códec MLow (portado al `internal/voip/media/mlow` Go puro)
- [**zapo**](https://github.com/w3nder/zapo) — Referencia del stack de media VoIP

---

## Licencia

[MIT](./LICENSE)
