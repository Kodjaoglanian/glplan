# glplan — direção de design

## Tese

O glplan é tratado como um **catálogo operacional de trabalho**, não como um Kanban genérico. Cada task é uma entrada numerada, com prioridade legível como uma trilha lateral; cada coluna expõe carga e pressão por meio de contagem, WIP e um medidor explicitamente rotulado. A interface deve parecer uma ferramenta profissional de operação: densa, precisa, silenciosa e rápida.

## Sistema visual

- **Base:** preto fosco/grafite, sem cromia excessiva.
- **Texto:** branco frio e cinzas operacionais.
- **Foco/seleção:** ciano.
- **Concluído:** verde.
- **Alta prioridade/atenção:** âmbar.
- **Urgente/erro:** vermelho.
- **Revisão:** violeta.
- **Estrutura:** hairlines finas, bordas arredondadas discretas e alinhamento por colunas tipográficas.

A tipografia é a monospace nativa do terminal. Não há dependência de ícones especiais; onde há glifos Unicode, eles são decorativos e nunca são o único portador de significado.

## Primeira leitura

Em poucos segundos, o primeiro viewport deve responder:

1. qual produto/projeto está aberto;
2. se a conexão está saudável e qual backend está em uso;
3. quanto trabalho está aberto, urgente e concluído;
4. quais colunas estão sob pressão;
5. qual task está selecionada e o que pode ser feito agora.

## Composição

- Cabeçalho compacto com produto, projeto, backend, contadores e erro operacional.
- Tabs claras para `Board`, `List` e `Activity`.
- Board adaptativo com no máximo quatro colunas visíveis, roláveis/paginadas conforme a largura; legibilidade vence densidade.
- Cards usam metadados explícitos (`est`, `resp`, `chk`, `prazo`) e títulos com até duas linhas antes de truncar.
- Cards com referência, prioridade, título, labels, responsável, checklist, estimativa e prazo.
- Detail e pickers como overlays contidos no viewport.
- Footer contextual: apenas comandos válidos no modo atual.
- Estados vazios e erros explicam o próximo passo em vez de apenas dizer que falhou.

## Interação

A operação é keyboard-first e previsível:

- `h/j/k/l` e setas navegam;
- `J/K` reordenam;
- `>/<` movem entre colunas;
- `enter` abre detalhe;
- `n/e` criam/editam;
- `p/N` trocam/criam projeto;
- `?` revela ajuda completa.

As cores são redundantes com texto: prioridade, erro e conclusão nunca dependem exclusivamente de cor.

## Responsividade e robustez

- Larguras e truncamento usam medidas de display width/ANSI-aware.
- Terminais estreitos reduzem colunas visíveis e truncam sem quebrar bordas.
- Terminais muito pequenos mostram um estado explícito.
- API indisponível, lista vazia, projeto vazio, loading, confirmação e erros têm representação própria.
- Backend embutido é apenas uma conveniência local; falha de API remota configurada não é mascarada.

## Verificação de acabamento

- `impeccable detect` nos arquivos da TUI: nenhum problema reportado.
- `go test ./...`: passa.
- `go vet ./...`: passa.
- `go build ./cmd/glplan`: passa.
- Smoke tests cobrem renderização em terminais estreitos e largos, overlay de detalhe, label picker e estados vazios.

## Princípio de produto

Humanos e agentes operam a mesma verdade pela mesma API. A TUI é a superfície principal; MCP e REST não são implementações paralelas com semântica divergente.
