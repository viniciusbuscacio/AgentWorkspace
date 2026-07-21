import { describe, expect, it, vi } from 'vitest';
import { modulesService } from './modules.service';

const ListModules = vi.fn();
const AddModule = vi.fn();
const RemoveModule = vi.fn();

vi.mock('@wails/go/main/App', () => ({
  ListModules: (...args: unknown[]) => ListModules(...args),
  AddModule: (...args: unknown[]) => AddModule(...args),
  RemoveModule: (...args: unknown[]) => RemoveModule(...args),
}));

describe('modulesService', () => {
  it('maps the Go catalog result', async () => {
    ListModules.mockResolvedValue({
      success: true,
      modules: [{ id: 'chat', name: 'Chat', icon: 'chat', description: '...', core: true, comingSoon: false, added: true }],
    });
    const result = await modulesService.list();
    expect(result.success).toBe(true);
    expect(result.modules[0].id).toBe('chat');
  });

  it('passes add and remove ids through to the bindings', async () => {
    AddModule.mockResolvedValue({ success: true, modules: [] });
    RemoveModule.mockResolvedValue({ success: true, modules: [] });
    await modulesService.add('notes');
    await modulesService.remove('notes');
    expect(AddModule).toHaveBeenCalledWith('notes');
    expect(RemoveModule).toHaveBeenCalledWith('notes');
  });
});
