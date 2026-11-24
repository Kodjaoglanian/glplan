# glplan

Board kanban **AI-first** no terminal. Um binário Go com TUI operacional de alta densidade, SQLite local, API HTTP para equipe e servidor MCP para agentes.

```
┌ Backlog ─────────┐ ┌ Todo ────────────┐ ┌ In Progress ─────┐ ┌ Done ────────────┐
│ GL-1  ●          │ │ GL-2  ● high     │ │ GL-3  ● urgent   │ │ GL-5  ●          │
│ Estruturar board │ │ Revisar atalhos  │ │ Sync via API     │ │ Schema SQLite    │
└──────────────────┘ └──────────────────┘ └──────────────────┘ └──────────────────┘
```

## Instalação em uma linha

**Linux (amd64 / arm64):**

```bash
curl -fsSL https://github.com/Kodjaoglanian/glplan/raw/refs/heads/main/install.sh | bash
```

**Windows (PowerShell):**

```powershell
irm https://github.com/Kodjaoglanian/glplan/raw/refs/heads/main/install.ps1 | iex
```

O instalador detecta a arquitetura, baixa o asset da última release, valida o SHA256, instala em `~/.glplan/bin` (Linux) ou `%LOCALAPPDATA%\glplan\bin` (Windows) e configura o PATH.

Sobrescreva com `GLPLAN_INSTALL_DIR` / `GLPLAN_VERSION` se precisar.

## Começando

```bash
glplan
```

Esse é o caminho plug-and-play: se nenhuma API local estiver disponível, o próprio processo da TUI sobe um backend embutido, cria `~/.local/share/glplan/glplan.db` e semeia um board de exemplo na primeira execução (`--no-seed` desliga). Não é preciso entender a arquitetura antes de usar.

Para compartilhar o mesmo board entre pessoas, agentes e várias janelas, rode um servidor explícito:

```bash
glplan serve          # API em http://127.0.0.1:9876
glplan tui            # em outro terminal; conecta na API local
```

## TUI

| Tecla | Ação |
| --- | --- |
| `j/k`, `↑/↓` | navegar tasks |
| `h/l`, `←/→` | mudar de coluna |
| `enter` | abrir detalhes |
| `n` / `e` | nova task / editar task |
| `J / K` | reordenar dentro da coluna |
| `> / <` | mover task entre colunas |
| `/` | buscar |
| `p` / `N` | trocar projeto / novo projeto |
| `1 2 3` | board / lista / atividade |
| `r` | atualizar da API |
| `?` | ajuda · `q` sair |

Dentro do detalhe: `x` concluir subtask, `t` novo subtask, `c` comentar, `L` editar labels, `d` ciclar prazo, `p` ciclar prioridade, `a` ciclar responsável, `del` excluir.

A interface atualiza sozinha a cada 4 segundos, então várias pessoas podem ver o mesmo board.

## MCP

O servidor MCP fala stdio e usa a mesma API do TUI. Se nenhuma API local estiver disponível, ele também pode subir o backend embutido usando o mesmo SQLite:

```json
{
  "mcpServers": {
    "glplan": {
      "command": "/caminho/para/glplan",
      "args": ["mcp"],
      "env": {
        "GLPLAN_API_URL": "http://127.0.0.1:9876"
      }
    }
  }
}
```

Para time remoto, aponte `GLPLAN_API_URL` para o servidor e use `GLPLAN_TOKEN` (o `serve` aceita `--token`).

Ferramentas disponíveis: `list_projects`, `create_project`, `get_board`, `list_columns`, `create_column`, `list_tasks`, `get_task`, `create_task`, `update_task`, `move_task`, `delete_task`, `search_tasks`, `list_labels`, `create_label`, `list_members`, `create_member`, `add_subtask`, `toggle_subtask`, `add_comment`, `list_comments`, `get_activity`.

Prompts: `plan_sprint`, `triage`.

## API

`GET /api/health`, `/api/projects`, `/api/projects/{id}/board`, `/api/tasks`, `/api/tasks/{id}`, `/api/search`, `/api/projects/{id}/activity`, além de rotas para colunas, labels, membros, comentários e subtasks. Rotas de projeto aceitam ID ou key (por exemplo `GL`). Autenticação bearer opcional via `GLPLAN_TOKEN`.

## Configuração

```bash
glplan config --server http://server:9876 --token xyz
glplan init --data /caminho/glplan.db
```

Precedência: flags > variáveis de ambiente > `~/.config/glplan/config.json`.

## Build local

```bash
go test ./...
go vet ./...
go build -o glplan ./cmd/glplan
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o glplan.exe ./cmd/glplan
```

## Release

Push de uma tag `v*`:

```bash
git tag v0.1.0 && git push origin v0.1.0
```

O `.github/workflows/release.yml` cria a release, compila linux/amd64, linux/arm64, windows/amd64 e windows/arm64 com `CGO_ENABLED=0`, publica binários + `.sha256` e os instaladores. Também dá para disparar manualmente via `workflow_dispatch` informando a versão.

Licença: MIT.
