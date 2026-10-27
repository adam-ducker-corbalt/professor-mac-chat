import * as React from 'react';
import * as lucide from 'lucide-react';
import Chat from './Chat';
import styles from './ChatWidget.module.css';

// A launcher in the bottom-right corner that opens the chat in a floating
// panel. The chat stays mounted while closed, so the conversation survives.
const ChatWidget = () => {
  const [open, setOpen] = React.useState(false);
  const panelRef = React.useRef<HTMLDivElement>(null);
  const launcherRef = React.useRef<HTMLButtonElement>(null);

  // Focus the input when opening; Escape closes and returns focus to the launcher.
  React.useEffect(() => {
    if (!open) return;
    panelRef.current?.querySelector('textarea')?.focus();
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false);
        launcherRef.current?.focus();
      }
    };
    window.addEventListener('keydown', handleKey);
    return () => window.removeEventListener('keydown', handleKey);
  }, [open]);

  return (
    <>
      <div
        ref={panelRef}
        id="chat-panel"
        className={styles.panel}
        role="dialog"
        aria-label="Ask ProfessorMAC"
        hidden={!open}
      >
        <div className={styles.titleBar}>
          <lucide.MessageSquare aria-hidden className={styles.titleIcon} />
          <span className={styles.title}>Ask ProfessorMAC</span>
          <button
            type="button"
            className={styles.close}
            aria-label="Close chat"
            onClick={() => {
              setOpen(false);
              launcherRef.current?.focus();
            }}
          >
            <lucide.X aria-hidden />
          </button>
        </div>
        <div className={styles.body}>
          <Chat />
        </div>
      </div>

      <button
        ref={launcherRef}
        type="button"
        className={styles.launcher}
        aria-expanded={open}
        aria-controls="chat-panel"
        onClick={() => setOpen(o => !o)}
      >
        {open
          ? <lucide.ChevronDown aria-hidden className={styles.launcherIcon} />
          : <lucide.MessageSquare aria-hidden className={styles.launcherIcon} />}
        {open ? 'Close' : 'Ask ProfessorMAC'}
      </button>
    </>
  );
};

export default ChatWidget;
