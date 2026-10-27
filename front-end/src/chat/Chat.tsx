import * as React from 'react';
import { Button } from '@cmsgov/design-system';
import * as lucide from 'lucide-react';
import styles from './Chat.module.css';

// A tool the model called to get data, e.g. get_dora_metrics({"team":"MACFin"}).
type ToolUse = {
  name: string
  arguments: string
};

type Message = {
  role: 'user' | 'assistant'
  text: string
  // Lookups the model made for this answer (shown, not sent back).
  tools?: ToolUse[]
  // Failed requests are shown in the transcript but never sent back to the model.
  error?: boolean
};

const EXAMPLE_QUESTIONS = [
  'How are DevOps metrics changing over the past three months?',
  'What are our greatest sources of cost savings?',
  "How has MACFin's deployment cadence changed since they shipped in February?",
];

// Sends the conversation to the Go server, which asks OpenAI. Returns the reply
// and the tools the model used to answer it.
async function askProfessorMAC(conversation: Message[]): Promise<{ reply: string, tools: ToolUse[] }> {
  const res = await fetch('/api/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ messages: conversation.map(({ role, text }) => ({ role, text })) }),
  });
  const data = (await res.json().catch(() => ({}))) as { reply?: string, tools?: ToolUse[], error?: string };
  if (!res.ok || data.reply === undefined) {
    throw new Error(data.error ?? `HTTP ${res.status}`);
  }
  return { reply: data.reply, tools: data.tools ?? [] };
}

// "get_dora_metrics(team: MACFin, start_month: 2026-02)"
function describeTool(tool: ToolUse): string {
  let args: Record<string, unknown> = {};
  try {
    args = JSON.parse(tool.arguments || '{}') as Record<string, unknown>;
  } catch {
    return `${tool.name}(${tool.arguments})`;
  }
  const list = Object.entries(args).map(([k, v]) => `${k}: ${String(v)}`).join(', ');
  return `${tool.name}(${list})`;
}

const Chat = () => {
  const [messages, setMessages] = React.useState<Message[]>([]);
  const [draft, setDraft] = React.useState('');
  const [pending, setPending] = React.useState(false);
  const inputRef = React.useRef<HTMLTextAreaElement>(null);
  const endRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    endRef.current?.scrollIntoView({ block: 'end' });
  }, [messages, pending]);

  const submit = async () => {
    const question = draft.trim();
    if (!question || pending) return;

    const conversation: Message[] = [...messages.filter(m => !m.error), { role: 'user', text: question }];
    setMessages(m => [...m, { role: 'user', text: question }]);
    setDraft('');
    setPending(true);
    try {
      const { reply, tools } = await askProfessorMAC(conversation);
      setMessages(m => [...m, { role: 'assistant', text: reply, tools }]);
    } catch (err) {
      const reason = err instanceof Error ? err.message : String(err);
      setMessages(m => [...m, { role: 'assistant', text: `Something went wrong: ${reason}`, error: true }]);
    } finally {
      setPending(false);
      inputRef.current?.focus();
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    void submit();
  };

  // Enter sends; Shift+Enter adds a new line.
  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault();
      void submit();
    }
  };

  const useExample = (question: string) => {
    setDraft(question);
    inputRef.current?.focus();
  };

  return (
    <section className={styles.root} aria-label="Chat with ProfessorMAC">
      <div className={styles.transcript} aria-live="polite">
        {messages.length === 0
          ? (
              <div className={styles.empty}>
                <h2 className="ds-text-heading--lg ds-u-margin-top--0">Try asking</h2>
                <ul className={styles.examples}>
                  {EXAMPLE_QUESTIONS.map(q => (
                    <li key={q}>
                      <button type="button" className={styles.example} onClick={() => useExample(q)}>
                        {q}
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            )
          : messages.map((m, i) => (
              <div key={i} className={m.role === 'user' ? styles.user : m.error ? styles.error : styles.assistant}>
                {m.tools && m.tools.length > 0 && (
                  <ul className={styles.tools} aria-label="Data looked up">
                    {m.tools.map((t, j) => (
                      <li key={j}>
                        <lucide.Database aria-hidden className={styles.toolIcon} />
                        {describeTool(t)}
                      </li>
                    ))}
                  </ul>
                )}
                {m.text}
              </div>
            ))}
        {pending && <div className={styles.pending}>ProfessorMAC is thinking…</div>}
        <div ref={endRef} />
      </div>

      <form className={styles.composer} onSubmit={handleSubmit}>
        <label htmlFor="chat-input" className="ds-u-visibility--screen-reader">
          Ask ProfessorMAC a question
        </label>
        <textarea
          id="chat-input"
          ref={inputRef}
          className={`ds-c-field ${styles.input}`}
          rows={1}
          placeholder="Ask a question…"
          value={draft}
          onChange={e => setDraft(e.target.value)}
          onKeyDown={handleKeyDown}
        />
        <Button type="submit" variation="solid" disabled={!draft.trim() || pending} className={styles.send}>
          <lucide.SendHorizontal aria-hidden className={styles.sendIcon} />
          Send
        </Button>
      </form>
    </section>
  );
};

export default Chat;
