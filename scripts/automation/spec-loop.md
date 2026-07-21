# Spec queue — persistent instructions for the implementing agent

You receive ONE spec from `docs/plans/` per run. Your job is to implement it
completely and leave the working copy CLEAN at the end. Non-negotiable rules:

1. **Read the whole spec before touching any file.** "Locked decisions" are
   not up for debate — implement what is written. When the spec points to
   AW2 reference files, open them and copy the behavior.

2. **Execute phase by phase, in the spec's order.** At the end of EACH
   phase, run the spec's gates (lint, go test, buildgate, frontend build)
   and fix whatever breaks before moving on.

3. **Small commits, one per phase, always with explicit files.** NEVER
   `git add -A`, NEVER `git add .`. Commit messages in English, in the
   style of the recent `git log`.

4. **When done:** update the spec's `> **Status:**` header to
   `implemented (<date>, commits: <hashes>)` and commit that edit too. The
   working tree MUST end clean (`git status --porcelain` empty) — the loop
   checks this and HALTS the queue if anything is left over.

5. **Abort rules (commit nothing and explain in your output):**
   - The working tree was already dirty when you started.
   - The spec appears to be already implemented in the code.
   - You need a decision the spec does not cover.
   The loop treats your no-commit exit as a stop signal — that is the
   correct behavior, not a failure.

6. Code comments and UI copy in English. Go backend changes require an app
   restart to load — mention that in your final summary.
