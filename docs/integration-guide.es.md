# Guía de integración de GoJira

[English](integration-guide.md) · **Español**

GoJira se conecta a Jira Cloud con **tu propio token de API de Atlassian**, guarda ese token en tu equipo y funciona desde una terminal, desde un agente de IA que pueda ejecutar comandos o desde cualquier cliente de IA compatible con MCP. Esta guía te lleva de cero a una configuración que funciona. No hace falta saber programar; basta con poder abrir una terminal.

> El README y los mensajes del programa están en inglés. En esta guía se mantienen tal como los verás en pantalla.

## Camino rápido

| # | Qué haces | Qué deberías ver |
|---|---|---|
| 1 | [Instalar GoJira](../README.md#installation) | `gojira version` muestra una versión |
| 2 | [Crear un token de API de Atlassian](#2-crear-un-token-de-api-de-atlassian) | un token largo, mostrado **una sola vez** |
| 3 | `gojira config init` | las preguntas de dominio, correo y token |
| 4 | `gojira context` | una línea `Identity: <tu nombre en Jira> (active: true)` |
| 5 | `gojira projects` | los proyectos que puedes ver |

> **¿Por qué el paso 4 y no `gojira config test`?** `config test`, y la línea de "connected" que imprime `config init`, consultan el endpoint de información del servidor de Jira, que responde sin credenciales. Confirman que la **dirección del sitio** es correcta, no que el **token** sea válido. `gojira context` le pregunta a Jira quién eres, así que falla cuando el correo o el token son incorrectos.

## 1. Instalar

Sigue la sección [Installation](../README.md#installation) del README: un comando para Linux y macOS, otro para PowerShell en Windows, y no hace falta tener Go instalado. Si prefieres revisar el script antes de ejecutarlo, lee [Installer trust and integrity](../README.md#installer-trust-and-integrity).

## 2. Crear un token de API de Atlassian

1. Inicia sesión con la cuenta de Atlassian que usas para Jira y abre <https://id.atlassian.com/manage-profile/security/api-tokens>.
2. Si inicias sesión con contraseña o con un acceso de terceros, Atlassian puede enviarte por correo un código de un solo uso para verificar tu identidad.
3. Elige **Create API token**. Elige el normal, **no** el de "with scopes": Atlassian documenta una dirección base distinta para los tokens con scopes, y GoJira usa la dirección de tu sitio (`https://<tu-sitio>.atlassian.net`).
4. Ponle un nombre que diga para qué sirve (por ejemplo `GoJira laptop`) y elige una **fecha de caducidad**. Atlassian permite de 1 a 365 días y propone un año por defecto.
5. Elige **Copy to clipboard** y guarda el token en un gestor de contraseñas. **Atlassian no vuelve a mostrarlo.**

| Conviene saber | Detalle |
|---|---|
| Trátalo como una contraseña | Quien tenga el token puede llamar a Jira como tú. No lo pongas nunca en un commit, un issue, un chat ni una captura de pantalla. |
| Caduca | Cuando caduca, las solicitudes se rechazan. Crea un token nuevo y ejecuta otra vez `gojira config init`. Un recordatorio en el calendario ayuda. |
| Puedes revocarlo | En la misma página, elige **Revoke** junto al token cuando dejes de usarlo. |

## 3. Configurar GoJira

Ejecuta la configuración una vez:

```bash
gojira config init
```

Te pide tres datos:

| Pregunta | Qué escribir |
|---|---|
| `Jira domain` | El dominio de tu sitio **sin** `https://`, por ejemplo `miempresa.atlassian.net` |
| `Jira email` | El correo de la cuenta de Atlassian que creó el token |
| `Jira API token` | El token que copiaste |

> La pregunta del token **no oculta** lo que escribes o pegas. Hazlo donde nadie vea tu pantalla y limpia la terminal después.

Después verifica con `gojira context`. Si no hay un proyecto seleccionado, la primera línea será `Project: not configured; ask the human for a project.`; es normal. Lo que importa es la segunda línea, `Identity: <tu nombre en Jira> (active: true)`: demuestra que el correo y el token son válidos.

**Dónde se guarda cada cosa**

| Elemento | Dónde |
|---|---|
| Dominio y correo | `~/.config/gojira/config.yaml` (ejecuta `gojira config path` para ver la ubicación exacta) |
| Token de API | El llavero (keychain) de tu sistema operativo. Si no hay llavero disponible, usa el mismo archivo YAML, pidiendo el modo `0600` cuando el archivo se crea; **ese archivo no está cifrado**. |

**Variables de entorno en lugar del asistente** (útiles para scripts y CI). Tienen prioridad sobre la configuración guardada:

```bash
export JIRA_DOMAIN="miempresa.atlassian.net"
export JIRA_EMAIL="tu@ejemplo.com"
export JIRA_API_TOKEN="..."
```

Evita `gojira config init --token <TOKEN>` en equipos compartidos: el historial del shell conserva la línea de comandos completa. GoJira también lee un archivo `.env` en la carpeta donde lo ejecutas.

## 4. Usarlo

| Quiero… | Ejecuto | Más información |
|---|---|---|
| Trabajar en una terminal | `gojira tui` para el tablero, o comandos como `gojira search "assignee = currentUser()"` | [CLI commands](../README.md#cli-commands), [TUI keyboard shortcuts](../README.md#tui-keyboard-shortcuts) |
| Que lo use un agente de IA que ejecuta comandos | Añade `--json` a un comando: el agente recibe JSON estable por stdout y los diagnósticos por stderr | [Agent usage examples](../README.md#agent-usage-examples), [TicketJSON v1](../README.md#ticketjson-v1-contract) |
| Conectar un cliente de IA mediante MCP | `gojira mcp config` y pegar la entrada que imprime en el cliente | [Read-only MCP server](../README.md#read-only-mcp-server) |
| Darle a un agente instrucciones de uso sin crear archivos | `gojira guide` las imprime; el recurso MCP `gojira://guide` las sirve | [Optional agent guide](../README.md#optional-agent-guide) |

### Conectar un cliente de IA (MCP)

Cualquier cliente MCP que pueda lanzar un comando local puede usar GoJira. Deja que GoJira imprima la entrada por ti, ya apuntando a tu copia del programa:

```bash
gojira mcp config                  # el JSON mcpServers habitual (por defecto)
gojira mcp config --client codex   # la entrada de config.toml de Codex
```

- La salida **no contiene credenciales**. El servidor las lee de la configuración que hiciste con `gojira config init`, así que el fragmento es seguro de pegar o compartir.
- Por defecto el agente **solo puede leer**. Añade `--allow-writes` al comando **solo si** quieres que el agente cree tickets, comente, registre trabajo o mueva incidencias con **tu** cuenta: consulta [Opt-in guarded writes](../README.md#opt-in-guarded-writes).
- El texto de los tickets es un dato no confiable. Reserva el acceso de escritura para agentes y clientes en los que confíes.
- Dónde pegar la entrada depende del cliente. El README indica las ubicaciones de Codex; para otros clientes, revisa su documentación para encontrar el archivo de configuración de MCP.

## 5. Lista de seguridad

- [ ] El token está en un gestor de contraseñas, no en una nota, un commit ni un chat.
- [ ] Elegí una caducidad y sé cuándo renovarlo.
- [ ] Reviso y revoco los tokens que ya no uso.
- [ ] Uso la pregunta de `config init` o las variables de entorno en lugar de `--token`.
- [ ] Mi entrada de MCP es de solo lectura, salvo que necesite escrituras.

## Solución de problemas

| Qué ves | Causa probable | Qué hacer |
|---|---|---|
| `no credentials found (set env vars or run: gojira config init)` | No hay nada configurado, o solo algunas variables de entorno | Ejecuta `gojira config init`, o define las tres variables |
| `incomplete credentials (run: gojira config init)` | El dominio, el correo o el token están vacíos | Ejecuta otra vez `gojira config init` |
| `connection failed: … no such host` | Dominio incorrecto o sin conexión | Usa `miempresa.atlassian.net` sin `https://` y revisa tu conexión |
| `config test` dice que se conectó, pero otros comandos fallan con un error de autorización | Token incorrecto o caducado, o el correo no corresponde al token | Ejecuta `gojira context`; crea un token nuevo y ejecuta otra vez `gojira config init`. Consulta [Jira access errors for agents](../README.md#jira-access-errors-for-agents) |
| `MCP credentials unavailable; configure credentials with gojira config init` | El servidor MCP no encuentra credenciales | Ejecuta `gojira config init` con el mismo usuario del sistema operativo que lanza el cliente |
| Los resultados vienen de otra cuenta o de otro sitio | Variables `JIRA_*` o un archivo `.env` en la carpeta actual anulan la configuración guardada | Revisa tu entorno y la carpeta desde la que ejecutas GoJira |

## Siguientes pasos

- [CLI commands](../README.md#cli-commands) y [Search completeness](../README.md#search-completeness)
- [Project context](../README.md#project-context): recordar un proyecto de Jira por cada clon del repositorio
- [User candidates and time reports](../README.md#user-candidates-and-time-reports)
