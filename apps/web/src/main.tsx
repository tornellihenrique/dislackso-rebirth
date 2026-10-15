import '@fontsource-variable/inter';
import '@fontsource-variable/jetbrains-mono';
import './styles/index.css';

import { startApp } from './app/start';

const container = document.getElementById('root');
if (!container) throw new Error('index.html is missing #root');
void startApp(container);
