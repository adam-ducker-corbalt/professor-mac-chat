import * as lucide from 'lucide-react';
import styles from './Header.module.css';
import logo from './logo.svg';

// The menus mirror ProfessorMAC's global nav. They're display-only for now.
const MENUS = ['Savings Insights', 'DevOps Center'];

const Header = () => {
  return (
    <header className={styles.root}>
      <a href="/" className={styles.logo}>
        <img src={logo} alt="ProfessorMAC" />
      </a>
      <nav className={styles.nav}>
        <ul className={styles.list}>
          {MENUS.map(label => (
            <li key={label}>
              <button type="button" className={styles.trigger}>
                {label}
                <lucide.ChevronDown className={styles.icon} strokeWidth={3} />
              </button>
            </li>
          ))}
        </ul>
      </nav>
      <a
        href="https://confluenceent.cms.gov/x/cWpsJQ"
        target="_blank"
        rel="noreferrer"
        className={styles.help}
        aria-label="Help"
      >
        <lucide.HelpCircle strokeWidth={2} />
      </a>
    </header>
  );
};

export default Header;
