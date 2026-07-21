import { useEffect } from 'react';
import { ChatModule } from '@modules/chat/ChatModule';
import { initChatSendState } from '@modules/chat/chat-send-state';
import { initSubagentTasks } from '@modules/chat/subagent-tasks';

// Full-window chat for the PiP process: the exact same ChatModule as the main
// window (isPip only hides the pop-out button). Sidebar/settings stay in the
// main window, so chat switching is a no-op here.
export function PipChatWindow({ chatId }: { chatId: string }) {
  // The PiP window renders outside AppShell, so the app-lifetime chat globals
  // (send/queue state, subagent spawn cards) must be installed here too —
  // without them chat:subagent events arrive but nothing consumes them.
  useEffect(() => initChatSendState(), []);
  useEffect(() => initSubagentTasks(), []);
  return (
    <div className="pip-window">
      <ChatModule
        chatId={chatId}
        isPip
        onChatsChanged={async () => []}
        onSelectChat={() => {}}
      />
    </div>
  );
}
