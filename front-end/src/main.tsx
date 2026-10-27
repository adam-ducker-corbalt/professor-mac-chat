import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
// Same stylesheet order as ProfessorMAC: CMS Design System, then the reset
// that reverts parts of it, then our own styles.
import '@cmsgov/design-system/dist/css/index.css';
import '@cmsgov/design-system/dist/css/core-theme.css';
import './resetreset.css';
import './styles.css';
import App from './App';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
