package application

import (
	"strings"

	"aw/internal/domain/ports"
)

const (
	SubagentModeOff        = "off"
	SubagentModeBalanced   = "balanced"
	SubagentModeAggressive = "aggressive"
)

// SubagentInstruction builds the generic-subagent operating policy injected
// into the agent system prompt. It is prompt-level support: the agent still
// decides whether the current task has real independent work to delegate.
func SubagentInstruction(mode string) string {
	mode = normalizeSubagentMode(mode)
	if mode == SubagentModeOff {
		return ""
	}

	limit := "3"
	modeLine := "Balanced mode: use automatic generic subagents when the benefit is clear."
	if mode == SubagentModeAggressive {
		limit = "5"
		modeLine = "Aggressive mode: for large tasks, use automatic generic subagents whenever there is clear independent work, up to 5 parallel subagents."
	}

	return strings.TrimSpace(`## Generic subagents

` + modeLine + `

system.spawn { tasks: [{id, task, context?}], timeoutMs? } forks up to ` + limit + ` generic disposable subagents that run ARBITRARY tasks in parallel isolated contexts (each can read/write/edit files and run shell in the sandbox), returning {results:[{id, status, output, elapsedMs}]}. system.spawn is a real, available tool: if it appears in your aw actions, NEVER tell the user it is unavailable, that "this session" lacks it, or that you can only run tasks sequentially — just call it.

When the user EXPLICITLY asks you to spawn/launch/fire/"disparar" N subagents — even a trivial test such as running echo — obey directly: call system.spawn once with that many tasks. Do not refuse it as "too small", do not describe it in prose, and do not offer a sequential or "other means" workaround.

On your own initiative, use generic subagents to save main-context space, run genuinely independent work in parallel, or validate results. Do not spin them up for small or linear work you were not asked to parallelize, when explaining the task would cost more than doing it directly.

Subagents are always generic. Do not assign fixed roles such as frontend, backend, or tests. Each subagent gets one small task, minimal context, strict boundaries, and restricted autonomy.

When using subagents:
- The spawned tasks' live status (running -> done, with each result) is shown to the user as a card in the chat, so you do not need to narrate or hand-format their progress.
- Use at most ` + limit + ` parallel subagents.
- Keep working in the main agent while subagents run if independent work remains.
- Review and synthesize results proportionally to risk; do not redo the entire subagent task by default.
- If the user changes direction while subagents run, re-evaluate whether to stop, redirect, or ignore their results.

Subagent checklist:
- Objetivo:
- Escopo permitido:
- Fora de escopo:
- Contexto minimo:
- Autonomia: restrita; cumpra apenas esta tarefa.
- Permissoes: leitura, testes/build, e edicao somente quando explicitamente autorizada.
- Restricoes: trabalhar no estado atual da main; nao criar branches; nao trocar branch; nao fazer commits; do not install dependencies; nao criar threads, worktrees ou PRs; nao reverter mudancas alheias.
- Formato de retorno: Status, Resultado, Arquivos alterados, Testes/build executados, Bloqueios.

Editing rules:
- Subagents may edit only explicitly allowed files, modules, or areas.
- Never run two subagents that edit the same area at the same time.
- If scopes overlap, the main agent owns that shared work.
- If a subagent lacks context, it must return a compact blocker instead of improvising.

Chat visibility:
- When reporting subagent activity in chat, emit compact blocks using this exact shape:

::subagent{task="short task" status="running|completed|failed|blocked|stopped" result="one-line result"}
Prompt:
<the checklist sent to the subagent>

Return:
<compact subagent return>
::end-subagent

Keep the visible result short. Do not show token or cost usage by default.`)
}

func LoadSubagentMode(store ports.SubagentModeStore) string {
	if store == nil {
		return SubagentModeBalanced
	}
	return normalizeSubagentMode(store.LoadSubagentMode())
}

func SaveSubagentMode(store ports.SubagentModeStore, mode string) error {
	if store == nil {
		return nil
	}
	return store.SaveSubagentMode(normalizeSubagentMode(mode))
}

func normalizeSubagentMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case SubagentModeOff:
		return SubagentModeOff
	case SubagentModeAggressive:
		return SubagentModeAggressive
	default:
		return SubagentModeBalanced
	}
}
