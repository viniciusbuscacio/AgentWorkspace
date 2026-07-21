import { useState } from 'react';
import type { ModuleRuntimeProps } from '@modules/module-contract';
import { ModuleHelpButton } from '@/components/patterns/ModuleHelpButton';
import { SkillsPage } from './SkillsPage';
import { readLastSkillId } from './skills-nav-storage';

// SkillsModule is the workspace-module home of skill management (it lived in
// Settings until skills were promoted to a first-class app). The catalog/editor
// logic is SkillsPage; this wrapper owns the module chrome: the
// `Skills › <skill>` breadcrumb and the session restore of the last open skill.
export function SkillsModule({ registerLeaveGuard }: ModuleRuntimeProps) {
  const [detailTitle, setDetailTitle] = useState<string | null>(null);
  const [listRequest, setListRequest] = useState(0);
  // One-shot restore: reopening the module lands on the skill that was open
  // when the user navigated away (sessionStorage; cleared on app restart).
  const [restoredSkillId] = useState<string | null>(() => readLastSkillId());

  const textButtonClass =
    'breadcrumb-link m-0 min-w-0 truncate border-0 bg-transparent p-0 text-left font-inherit text-current focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50';

  return (
    <section className="home-screen" data-awid="skills-screen">
      <div className="home-header">
        <div className="flex items-start justify-between gap-3">
          <h1 className="home-title">
            {detailTitle ? (
              <span className="inline-flex min-w-0 items-baseline gap-2">
                <button
                  type="button"
                  className={textButtonClass}
                  onClick={() => setListRequest((current) => current + 1)}
                >
                  Skills
                </button>
                <span className="shrink-0 text-muted-foreground" aria-hidden="true">›</span>
                <span className="min-w-0 truncate">{detailTitle}</span>
              </span>
            ) : (
              'Skills'
            )}
          </h1>
          <ModuleHelpButton module="Skills" />
        </div>
        <p className="home-subtitle">
          Procedural skills the agent loads on demand — browse, create, import and edit them.
        </p>
      </div>
      <SkillsPage
        onSkillDetailTitleChange={setDetailTitle}
        listRequest={listRequest}
        onRegisterLeaveGuard={registerLeaveGuard}
        initialSkillId={restoredSkillId ?? undefined}
      />
    </section>
  );
}
