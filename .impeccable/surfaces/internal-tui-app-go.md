---
version: 1
slug: "internal-tui-app-go"
primary_target: "internal/tui/app.go"
related_targets: ["internal/tui/styles.go"]
---

## Scope

Terminal board surface (`internal/tui/app.go` and related TUI files). Visitor mode: Operate. Audience: developers and AI-agent operators in a keyboard-first terminal. Job: scan project state, triage, create/edit/reorder/move tasks, inspect detail, and trust that humans and agents share the same backend. Constraints: variable terminal sizes, no images, no mouse dependence, concise Portuguese copy, fast deterministic keyboard interaction.

## Direction contract

THESIS: O board é um catálogo vivo de trabalho, não um Kanban genérico: cada task é uma entrada numerada com trilha de estado, e a coluna revela sua carga como um pulso de capacidade. A interface recusa a grade cinzenta previsível de cards iguais; seleção “decodifica” a entrada ativa em cor e contexto sem sacrificar leitura operacional.

OWN-WORLD: Preto fosco quase absoluto, painéis grafite, tinta branca fria, hairlines discretas e tipografia monospace do próprio terminal usada com disciplina de catálogo. O sistema de cor é industrial e funcional: ciano para foco/seleção, verde para concluído, âmbar para atenção/alta prioridade, vermelho para urgente/erro, violeta para revisão. Densidade alta, mas com ritmo: cabeçalhos compactos, cards respirando em blocos alinhados, metadata em microtipografia.

STORY: Em cinco segundos o usuário entende projeto, trabalho aberto, colunas sob pressão e próxima ação. Ele navega sem manual, abre detalhes, edita, reordena e move trabalho com confiança. Erros e estados vazios explicam o próximo passo. O board parece uma ferramenta profissional de operação, não uma demo de terminal.

FIRST VIEWPORT: Uma barra superior fina mostra produto, projeto, saúde da conexão e contagem de trabalho aberto. Abaixo, tabs Board/List/Activity. O Board ocupa quase toda a tela em colunas adaptativas; cada coluna tem nome, contagem/WIP e um medidor de pulso segmentado. Cards mostram referência, prioridade como trilha lateral colorida, título, labels, responsável, checklist e due date. O card selecionado recebe borda ciano e faixa decodificada. Um footer contextual mostra apenas comandos válidos no modo atual.

FORM: Terceiro candidato grounded de sete, sistema de catálogo industrial matte-black inspirado em capas de selo/catalogação numerada; seed:8516d5f3bae29e4f.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
