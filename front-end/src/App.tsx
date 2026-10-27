import styles from './App.module.css';
import Header from './Header';
import ChatWidget from './chat/ChatWidget';

const App = () => {
  return (
    <div className={styles.root}>
      <Header />
      <main className={styles.main}>
        <h1 className="ds-text-heading--2xl ds-u-margin-top--0">ProfessorMAC Chat</h1>
        <p className={`ds-text-body--lg ${styles.lede}`}>
          Ask ProfessorMAC questions about DevOps metrics and cost savings in plain language. Open the
          chat with <strong>Ask ProfessorMAC</strong> in the bottom-right corner.
        </p>
      </main>
      <ChatWidget />
    </div>
  );
};

export default App;
