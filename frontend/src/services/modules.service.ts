import { AddModule, HideModule, HideAllModules, ListModules, MoveModule, RemoveModule, ShowModule } from '@wails/go/main/App';
import type { dto } from '@wails/go/models';

export type ModuleInfo = dto.ModuleInfo;
export type ModulesResult = dto.ModulesResult;

export const modulesService = {
  list(): Promise<ModulesResult> {
    return ListModules();
  },
  add(id: string): Promise<ModulesResult> {
    return AddModule(id);
  },
  remove(id: string): Promise<ModulesResult> {
    return RemoveModule(id);
  },
  /** Close in the sidebar — the module stays added (actions untouched). */
  hide(id: string): Promise<ModulesResult> {
    return HideModule(id);
  },
  /** Hide ALL non-core modules in one write — the "Show desktop" action. */
  hideAll(): Promise<ModulesResult> {
    return HideAllModules();
  },
  show(id: string): Promise<ModulesResult> {
    return ShowModule(id);
  },
  move(id: string, up: boolean): Promise<ModulesResult> {
    return MoveModule(id, up);
  },
};
