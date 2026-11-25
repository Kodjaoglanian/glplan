# glplan

**Board kanban AI-first no terminal.** Um único binário Go que combina TUI operacional de alta densidade, SQLite embutido, API HTTP para times e servidor MCP para agentes de IA — humanos e agentes trabalhando no mesmo board, em tempo real.

```
glplan / Glplan [GL]                          ● local  6 abertas  1 urgente  1 concluída
──────────────────────────────────────────────────────────────────────────────────────────
╭ Backlog ─────────────────────────────────╮ ╭ In Progress ─────────────────────────────╮
│ Backlog 2                                │ │ In Progress 1/3                          │
│ volume ██░░░░░ 2                         │ │ carga ██░░░░░ 33%                        │
│ ╭──────────────────────────────────────╮ │ │ ╭──────────────────────────────────────╮ │
│ │ ▌ GL-1  ALTA  est 3p                 │ │ │ │ ▌ GL-5  ALTA  est 3p                 │ │
│ │ ▌ Desenhar fluxo de onboarding       │ │ │ │ ▌ Criar installer one-line           │ │
│ │ ▌ #design                            │ │ │ │ ▌ #infra                             │ │
│ │ ▌ resp Carla · chk 0/2               │ │ │ │ ▌ resp Bruno · chk 0/2               │ │
│ │ ▌ prazo 28/11                        │ │ │ │ ▌ prazo 27/11                        │ │
│ ╰──────────────────────────────────────╯ │ ╰──────────────────────────────────────╯ │
╰──────────────────────────────────────────╯ ╰──────────────────────────────────────────╯
 j/k selecionar · J/K ordenar · h/l coluna · enter abrir · n nova · ? ajuda
```

## Por que glplan

- **Plug-and-play de verdade**: `glplan` abre direto. Sem servidor para configurar, sem banco para provisionar — o backend embutido sobe sozinho quando necessário.
- **AI-first**: agentes de IA operam o board por MCP com as mesmas regras dos humanos. Crie tasks, mova cards, comente e faça triagem em linguagem natural.
- **Multiplayer por padrão**: uma API HTTP central compartilha o board entre pessoas, agentes e janelas. A TUI sincroniza sozinha a cada 4 segundos.
- **Um binário, zero dependências**: Go compilado estaticamente, SQLite embutido. Funciona offline, em VM, container ou bare metal.
- **Interface de terminal séria**: denso mas legível — prioridades visuais, WIP limits, estimativas, checklists, prazos, labels e busca.

## Índice

- [Instalação](#instalação)
- [Começando](#começando)
- [Atualizando](#atualizando)
- [TUI](#tui)
- [Rodando em servidor (time)](#rodando-em-servidor-time)
- [Conectando clientes](#conectando-clientes)
- [MCP para agentes de IA](#mcp-para-agentes-de-ia)
- [API HTTP](#api-http)
- [Configuração](#configuração)
- [Arquitetura](#arquitetura)
- [Desenvolvimento](#desenvolvimento)

## Instalação

**Linux (amd64 / arm64):**

```bash
curl -fsSL https://github.com/Kodjaoglanian/glplan/raw/refs/heads/main/install.sh | bash
```

**Windows (PowerShell):**

```powershell
irm https://github.com/Kodjaoglanian/glplan/raw/refs/heads/main/install.ps1 | iex
```

O instalador detecta a arquitetura, baixa o asset da última release, **valida o SHA256**, instala em `~/.glplan/bin` (Linux) ou `%LOCALAPPDATA%\glplan\bin` (Windows) e configura o PATH. Sobrescreva com `GLPLAN_INSTALL_DIR` / `GLPLAN_VERSION` se precisar.

Também é possível baixar os binários direto em [Releases](https://github.com/Kodjaoglanian/glplan/releases) — cada asset tem seu `.sha256` ao lado.

## Começando

```bash
glplan
```

É só isso. Se nenhuma API local estiver no ar, o próprio processo da TUI sobe um backend embutido e cria `~/.local/share/glplan/glplan.db` com um **projeto limpo** — você começa a criar tasks imediatamente.

Quer explorar com dados fictícios antes? Use o demo explícito:

```bash
glplan init --demo   # ou: glplan --demo
```

### Comandos

| Comando | O que faz |
| --- | --- |
| `glplan` / `glplan tui` | abre a interface terminal (padrão) |
| `glplan serve` | inicia a API HTTP para compartilhar o board |
| `glplan mcp` | inicia o servidor MCP (stdio) para agentes |
| `glplan init` | cria o banco e um projeto limpo (`--demo` para dados de exemplo) |
| `glplan update` | atualiza o binário para a última release |
| `glplan config` | salva servidor/token/paths em `~/.config/glplan/config.json` |
| `glplan version` | mostra a versão |

Flags úteis: `--server URL`, `--token TOK`, `--data PATH`, `--addr ADDR`, `--demo`.

## Atualizando

```bash
glplan update
```

Consulta a última release no GitHub, compara com a versão instalada, baixa o asset correto para a sua plataforma, **valida o SHA256** e substitui o binário de forma atômica. Se já estiver na versão mais recente, apenas confirma. No Windows, se o executável estiver em uso, reinstale com `install.ps1`.

## TUI

| Tecla | Ação |
| --- | --- |
| `j/k`, `↑/↓` | navegar tasks |
| `h/l`, `←/→` | mudar de coluna |
| `enter` | abrir detalhes |
| `n` / `e` | nova task / editar task |
| `J` / `K` | reordenar dentro da coluna |
| `>` / `<` | mover task entre colunas |
| `/` | buscar |
| `p` / `N` | trocar projeto / novo projeto |
| `1` `2` `3` | board / lista / atividade |
| `r` | atualizar da API |
| `?` | ajuda · `q` sair |

Dentro do detalhe: `x` concluir subtask, `t` novo subtask, `c` comentar, `L` editar labels, `d` ciclar prazo, `p` ciclar prioridade, `a` ciclar responsável, `del` excluir.

Os cards são autoexplicativos: `est 3p` é estimativa em pontos, `resp Carla` é o responsável, `chk 1/2` é o progresso do checklist e `prazo` é o vencimento (vermelho quando atrasado). Colunas com WIP limit mostram `carga` em percentual.

## Rodando em servidor (time)

Para um board central da equipe, rode `glplan serve` em um servidor com token de autenticação.

### 1. Instalar e preparar

```bash
curl -fsSL https://github.com/Kodjaoglanian/glplan/raw/refs/heads/main/install.sh | bash
sudo cp ~/.glplan/bin/glplan /usr/local/bin/
sudo useradd -r -s /usr/sbin/nologin glplan
sudo mkdir -p /var/lib/glplan /etc/glplan
sudo chown glplan:glplan /var/lib/glplan
```

### 2. Token

Gere um token forte e guarde em `/etc/glplan/glplan.env`:

```bash
openssl rand -hex 32
```

```
GLPLAN_TOKEN=cole-o-token-aqui
```

```bash
sudo chmod 0640 /etc/glplan/glplan.env
sudo chown root:glplan /etc/glplan/glplan.env
```

### 3. systemd — `/etc/systemd/system/glplan.service`

```ini
[Unit]
Description=glplan kanban API
After=network.target

[Service]
Type=simple
User=glplan
EnvironmentFile=/etc/glplan/glplan.env
ExecStart=/usr/local/bin/glplan serve --addr 0.0.0.0:9876 --data /var/lib/glplan/glplan.db
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now glplan
```

### 4. Firewall e verificação

```bash
sudo firewall-cmd --permanent --add-port=9876/tcp && sudo firewall-cmd --reload   # firewalld
# ou: sudo ufw allow 9876/tcp                                                    # ufw

curl -H "Authorization: Bearer SEU-TOKEN" http://IP-DO-SERVIDOR:9876/api/health
# {"ok":true,"version":"vX.Y.Z"}
```

### Segurança

- **Sem token, a API fica aberta** para qualquer um na rede. Sempre use `GLPLAN_TOKEN` em servidor compartilhado.
- A API fala **HTTP puro** — o token trafega em claro. Em LAN/VPN interna costuma ser aceitável; para qualquer exposição além disso, coloque TLS na frente:

```
# Caddyfile — e o serve pode voltar para 127.0.0.1:9876
glplan.suaempresa.com {
    reverse_proxy 127.0.0.1:9876
}
```

### Backup

O banco inteiro é um único arquivo SQLite. Backup consistente com o servidor rodando:

```bash
sqlite3 /var/lib/glplan/glplan.db ".backup '/backup/glplan-$(date +%F).db'"
```

## Conectando clientes

Em cada máquina (uma única vez):

```bash
glplan config --server http://IP-DO-SERVIDOR:9876 --token SEU-TOKEN
glplan
```

A TUI passa a operar o board central — nada fica no SQLite local enquanto o servidor estiver configurado. Várias pessoas conectam simultaneamente e veem as mudanças umas das outras em até 4 segundos. Cadastrem cada pessoa como `member` para atribuir responsáveis nas tasks.

## MCP para agentes de IA

O MCP do glplan é **stdio**: o agente não conecta pela rede — ele executa `glplan mcp` como subprocesso local, e esse processo fala HTTP com a API (local ou central). Como ele lê o mesmo `~/.config/glplan/config.json` da TUI, basta ter rodado `glplan config` uma vez e nenhuma variável extra é necessária.

### Claude Code

```bash
claude mcp add glplan -- ~/.glplan/bin/glplan mcp
```

### Cursor / Windsurf / Devin / outros com `mcp.json`

```json
{
  "mcpServers": {
    "glplan": {
      "command": "/home/usuario/.glplan/bin/glplan",
      "args": ["mcp"]
    }
  }
}
```

Devin CLI: `devin mcp add glplan -- ~/.glplan/bin/glplan mcp`.

Se preferir explícito (ou em máquina sem `glplan config`):

```json
{
  "mcpServers": {
    "glplan": {
      "command": "/caminho/para/glplan",
      "args": ["mcp"],
      "env": {
        "GLPLAN_API_URL": "http://IP-DO-SERVIDOR:9876",
        "GLPLAN_TOKEN": "SEU-TOKEN"
      }
    }
  }
}
```

### O que o agente pode fazer

**20 tools**: `list_projects`, `create_project`, `get_board`, `list_columns`, `create_column`, `list_tasks`, `get_task`, `create_task`, `update_task`, `move_task`, `delete_task`, `search_tasks`, `list_labels`, `create_label`, `list_members`, `create_member`, `add_subtask`, `toggle_subtask`, `add_comment`, `list_comments`, `get_activity`.

**Prompts prontos**: `plan_sprint` (planejamento de sprint) e `triage` (triagem do backlog).

Na prática: *"cria uma task no glplan pra revisar o deploy de sexta, prioridade alta, responsável Ana"* — e o card aparece no board de todo mundo na hora.

## API HTTP

Base: `/api`. Autenticação bearer opcional (`GLPLAN_TOKEN` no servidor). Rotas de projeto aceitam **ID ou key** (ex.: `GL`).

| Grupo | Rotas |
| --- | --- |
| Saúde | `GET /api/health` |
| Projetos | `GET/POST /api/projects`, `GET/PATCH/DELETE /api/projects/{id}` |
| Board | `GET /api/projects/{id}/board` |
| Colunas | `GET/POST /api/projects/{id}/columns`, `PATCH/DELETE /api/columns/{id}` |
| Tasks | `GET/POST /api/tasks`, `GET/PATCH/DELETE /api/tasks/{id}`, `POST /api/tasks/{id}/move` |
| Labels | `GET/POST /api/projects/{id}/labels`, `PUT /api/tasks/{id}/labels` |
| Membros | `GET/POST /api/members`, `PATCH/DELETE /api/members/{id}` |
| Checklist | `POST /api/tasks/{id}/subtasks`, `PATCH/DELETE /api/subtasks/{id}` |
| Comentários | `GET/POST /api/tasks/{id}/comments` |
| Busca | `GET /api/search?q=...` |
| Atividade | `GET /api/projects/{id}/activity` |

Exemplo:

```bash
curl -H "Authorization: Bearer $TOKEN" http://server:9876/api/projects/GL/board
```

## Configuração

Precedência: **flags > variáveis de ambiente > `~/.config/glplan/config.json`**.

| Flag | Variável | Padrão |
| --- | --- | --- |
| `--server` | `GLPLAN_API_URL` | embutido/local |
| `--token` | `GLPLAN_TOKEN` | — |
| `--data` | `GLPLAN_DATA` | `~/.local/share/glplan/glplan.db` |
| `--addr` | `GLPLAN_ADDR` | `:9876` |

```bash
glplan config --server http://server:9876 --token xyz   # salva sem digitar de novo
```

## Arquitetura

```
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│  TUI (você) │   │  MCP (IA)   │   │  scripts/   │
│  glplan     │   │  glplan mcp │   │  curl etc.  │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       └─────────────────┼─────────────────┘
                      HTTP+JSON
                         │
               ┌─────────▼─────────┐
               │  glplan serve     │      ┌─ modo solo ─────────────┐
               │  API + auth       │  ou  │ glplan: backend embutido │
               └─────────┬─────────┘      │ no próprio processo TUI  │
                         │                └───────────┬─────────────┘
                  ┌──────▼──────┐                     │
                  │  SQLite     │◄────────────────────┘
                  │  glplan.db  │
                  └─────────────┘
```

- `internal/model` — tipos de domínio; `internal/store` — SQLite, regras de posição/conclusão, seed;
- `internal/api` — servidor REST e cliente HTTP; `internal/tui` — interface Bubble Tea;
- `internal/mcp` — tools/prompts MCP; tudo passa pela mesma API, então humano e agente obedecem às mesmas regras.

## Desenvolvimento

```bash
go test ./...
go vet ./...
go build -o glplan ./cmd/glplan
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o glplan.exe ./cmd/glplan
```

Documentação de produto e design em [PRODUCT.md](PRODUCT.md) e [DESIGN.md](DESIGN.md).

### Release

Push de uma tag `v*`:

```bash
git tag vX.Y.Z && git push origin vX.Y.Z
```

O workflow valida a versão, roda formatação/vet/testes, compila linux/amd64, linux/arm64, windows/amd64 e windows/arm64 em paralelo, verifica arquivos e checksums, e só então publica a release completa com binários + `.sha256` e instaladores. Também dá para disparar manualmente via `workflow_dispatch`.

Licença: MIT.
