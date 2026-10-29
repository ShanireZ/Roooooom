import {StrictMode} from 'react';
import {createRoot} from 'react-dom/client';
import {App} from './App';
import './style.css';
import {readTheme} from './theme/theme';
document.documentElement.dataset.theme=readTheme();
createRoot(document.getElementById('root')!).render(<StrictMode><App/></StrictMode>);
