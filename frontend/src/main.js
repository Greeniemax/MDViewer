import './style.css';
import './app.css';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { 
    OpenFileDialog, 
    SaveFileDialog, 
    ReadFile, 
    GetCurrentFile,
    GetContent,
    SetActiveDocument,
    SetWatchedFilePaths,
    SpeakLine,
    Stop, 
    IsSpeaking,
    GetAvailableVoices 
} from '../wailsjs/go/main/App';

const RECENT_FILES_KEY = 'mdviewer-recent-files-v1';
const MAX_RECENT_FILES = 10;

// State
let content = '';
let isSpeaking = false;
let selectedLineIndex = 0;
let currentLines = [];
let shouldStopSpeaking = false;

/** @type {{ id: number, path: string | null, title: string, value: string }[]} */
let docTabs = [];
let activeDocTabId = null;
let docTabIdSeq = 0;

// Elements
const modeTabs = document.querySelectorAll('.tab');
const editorContent = document.getElementById('editorContent');
const viewerContent = document.getElementById('viewerContent');
const viewerToolbar = document.getElementById('viewerToolbar');
const editor = document.getElementById('editor');
const lineNumbers = document.getElementById('lineNumbers');
const viewer = document.getElementById('viewer');
const openBtn = document.getElementById('openBtn');
const saveBtn = document.getElementById('saveBtn');
const speakBtn = document.getElementById('speakBtn');
const stopBtn = document.getElementById('stopBtn');
const voiceSelect = document.getElementById('voiceSelect');
const rateSlider = document.getElementById('rateSlider');
const rateValue = document.getElementById('rateValue');
const filePathEl = document.getElementById('filePath');
const wordCountEl = document.getElementById('wordCount');
const readingStatusEl = document.getElementById('readingStatus');
const loadingIndicator = document.getElementById('loadingIndicator');

const findToolbarBtn = document.getElementById('findToolbarBtn');
const findDialogOverlay = document.getElementById('findDialogOverlay');
const findInput = document.getElementById('findInput');
const findMatchCase = document.getElementById('findMatchCase');
const findWholeWord = document.getElementById('findWholeWord');
const findRunBtn = document.getElementById('findRunBtn');
const findCancelBtn = document.getElementById('findCancelBtn');
const findResultsPanel = document.getElementById('findResultsPanel');
const findResultsList = document.getElementById('findResultsList');
const findResultsTitle = document.getElementById('findResultsTitle');
const findResultsClose = document.getElementById('findResultsClose');

const docTabsBar = document.getElementById('docTabsBar');
const recentMenuWrap = document.getElementById('recentMenuWrap');
const recentMenuBtn = document.getElementById('recentMenuBtn');
const recentMenu = document.getElementById('recentMenu');

let findMatches = [];

// Context menu
const contextMenu = document.createElement('div');
contextMenu.id = 'contextMenu';
contextMenu.className = 'context-menu';
contextMenu.innerHTML = `
    <div class="context-menu-item" data-action="copy">Copy</div>
    <div class="context-menu-item" data-action="cut">Cut</div>
    <div class="context-menu-item" data-action="paste">Paste</div>
    <div class="context-menu-item" data-action="delete">Delete</div>
    <div class="context-menu-separator"></div>
    <div class="context-menu-item" data-action="speak">Speak</div>
`;
document.body.appendChild(contextMenu);

function pathsEqual(a, b) {
    if (!a || !b) return false;
    if (a === b) return true;
    const na = a.replace(/\\/g, '/').toLowerCase();
    const nb = b.replace(/\\/g, '/').toLowerCase();
    return na === nb;
}

function fileTitleFromPath(p) {
    if (!p) return 'Untitled';
    const clean = p.replace(/\\/g, '/');
    const parts = clean.split('/');
    return parts[parts.length - 1] || 'Untitled';
}

function getActiveDocTab() {
    return docTabs.find((t) => t.id === activeDocTabId);
}

function flushActiveTabEditor() {
    const tab = getActiveDocTab();
    if (tab) tab.value = editor.value;
}

function syncGoActiveDocument() {
    const tab = getActiveDocTab();
    if (!tab) return;
    SetActiveDocument(tab.path || '', tab.value);
}

function shouldReuseEmptyUntitledTab() {
    if (docTabs.length !== 1) return false;
    const t = docTabs[0];
    return !t.path && !t.value.trim();
}

function renderDocTabsBar() {
    if (!docTabsBar) return;
    docTabsBar.innerHTML = '';
    docTabs.forEach((tab) => {
        const el = document.createElement('div');
        el.className = 'doc-tab' + (tab.id === activeDocTabId ? ' active' : '');
        el.title = tab.path || 'Untitled';
        el.dataset.docTabId = String(tab.id);

        const label = document.createElement('span');
        label.className = 'doc-tab-label';
        label.textContent = tab.title;

        const closeBtn = document.createElement('button');
        closeBtn.type = 'button';
        closeBtn.className = 'doc-tab-close';
        closeBtn.setAttribute('aria-label', 'Close tab');
        closeBtn.textContent = '×';

        el.appendChild(label);
        el.appendChild(closeBtn);

        el.addEventListener('click', (e) => {
            if (e.target.closest('.doc-tab-close')) return;
            switchToDocTab(tab.id);
        });
        closeBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            closeDocTab(tab.id);
        });

        docTabsBar.appendChild(el);
    });
    void syncDiskWatchers();
}

async function syncDiskWatchers() {
    const paths = [...new Set(docTabs.map((t) => t.path).filter(Boolean))];
    try {
        await SetWatchedFilePaths(paths);
    } catch (e) {
        console.error('SetWatchedFilePaths:', e);
    }
}

function switchToDocTab(id) {
    closeFindPanel();
    flushActiveTabEditor();
    activeDocTabId = id;
    const tab = getActiveDocTab();
    if (!tab) return;

    editor.value = tab.value;
    content = tab.value;
    updateLineNumbers();
    filePathEl.textContent = tab.path || 'No file open';
    wordCountEl.textContent = `${countWords(tab.value)} words`;
    saveBtn.disabled = false;
    speakBtn.disabled = !tab.value.trim();

    if (viewerContent.classList.contains('active')) {
        renderMarkdown(tab.value);
    }
    syncGoActiveDocument();
    renderDocTabsBar();
}

function closeDocTab(id) {
    flushActiveTabEditor();
    const idx = docTabs.findIndex((t) => t.id === id);
    if (idx === -1) return;

    if (docTabs.length === 1) {
        docTabs[0].path = null;
        docTabs[0].title = 'Untitled';
        docTabs[0].value = '';
        editor.value = '';
        content = '';
        updateLineNumbers();
        filePathEl.textContent = 'No file open';
        wordCountEl.textContent = '0 words';
        speakBtn.disabled = true;
        if (viewerContent.classList.contains('active')) {
            renderMarkdown('');
        }
        syncGoActiveDocument();
        renderDocTabsBar();
        return;
    }

    docTabs.splice(idx, 1);
    if (activeDocTabId === id) {
        const next = docTabs[Math.min(idx, docTabs.length - 1)];
        switchToDocTab(next.id);
    } else {
        renderDocTabsBar();
    }
}

function openOrFocusDocument(filePath, fileContent) {
    const existing = docTabs.find((t) => t.path && pathsEqual(t.path, filePath));
    if (existing) {
        addToRecentFiles(filePath);
        switchToDocTab(existing.id);
        return;
    }

    addToRecentFiles(filePath);
    flushActiveTabEditor();

    if (shouldReuseEmptyUntitledTab()) {
        const t = docTabs[0];
        t.path = filePath;
        t.title = fileTitleFromPath(filePath);
        t.value = fileContent;
        switchToDocTab(t.id);
        return;
    }

    docTabIdSeq += 1;
    const newId = docTabIdSeq;
    docTabs.push({
        id: newId,
        path: filePath,
        title: fileTitleFromPath(filePath),
        value: fileContent,
    });
    switchToDocTab(newId);
}

function ensureDefaultUntitledTab() {
    if (docTabs.length > 0) return;
    docTabIdSeq += 1;
    docTabs.push({
        id: docTabIdSeq,
        path: null,
        title: 'Untitled',
        value: '',
    });
    activeDocTabId = docTabIdSeq;
    editor.value = '';
    content = '';
    updateLineNumbers();
    filePathEl.textContent = 'No file open';
    wordCountEl.textContent = '0 words';
    saveBtn.disabled = false;
    speakBtn.disabled = true;
    syncGoActiveDocument();
    renderDocTabsBar();
}

function loadRecentFiles() {
    try {
        const raw = localStorage.getItem(RECENT_FILES_KEY);
        const arr = raw ? JSON.parse(raw) : [];
        return Array.isArray(arr) ? arr.filter((x) => typeof x === 'string' && x.trim()) : [];
    } catch {
        return [];
    }
}

function saveRecentFiles(list) {
    localStorage.setItem(RECENT_FILES_KEY, JSON.stringify(list.slice(0, MAX_RECENT_FILES)));
}

function addToRecentFiles(filePath) {
    if (!filePath || !String(filePath).trim()) return;
    let list = loadRecentFiles();
    list = list.filter((p) => !pathsEqual(p, filePath));
    list.unshift(filePath);
    saveRecentFiles(list);
    renderRecentMenu();
}

function closeRecentMenu() {
    if (!recentMenu) return;
    recentMenu.hidden = true;
}

function renderRecentMenu() {
    if (!recentMenu) return;
    recentMenu.innerHTML = '';
    const paths = loadRecentFiles().slice(0, MAX_RECENT_FILES);
    if (paths.length === 0) {
        const empty = document.createElement('div');
        empty.className = 'dropdown-menu-empty';
        empty.textContent = 'No recent files yet';
        recentMenu.appendChild(empty);
        return;
    }
    paths.forEach((p) => {
        const item = document.createElement('div');
        item.className = 'dropdown-menu-item';
        item.textContent = p;
        item.title = p;
        item.setAttribute('role', 'menuitem');
        item.addEventListener('click', () => openRecentFilePath(p));
        recentMenu.appendChild(item);
    });
}

async function openRecentFilePath(filePath) {
    closeRecentMenu();
    const existing = docTabs.find((t) => t.path && pathsEqual(t.path, filePath));
    if (existing) {
        addToRecentFiles(filePath);
        switchToDocTab(existing.id);
        return;
    }
    try {
        const text = await ReadFile(filePath);
        openOrFocusDocument(filePath, text);
    } catch (err) {
        console.error(err);
        alert('Could not open file:\n' + filePath);
        let list = loadRecentFiles().filter((x) => !pathsEqual(x, filePath));
        saveRecentFiles(list);
        renderRecentMenu();
    }
}

recentMenuBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    const open = recentMenu.hidden;
    closeRecentMenu();
    if (open) {
        renderRecentMenu();
        recentMenu.hidden = false;
    }
});

// Tab switching (MD Editor / MD Viewer)
modeTabs.forEach(tab => {
    tab.addEventListener('click', () => {
        const tabName = tab.dataset.tab;
        
        modeTabs.forEach(t => t.classList.remove('active'));
        tab.classList.add('active');
        
        if (tabName === 'editor') {
            editorContent.classList.add('active');
            viewerContent.classList.remove('active');
            viewerToolbar.style.display = 'none';
        } else {
            editorContent.classList.remove('active');
            viewerContent.classList.add('active');
            viewerToolbar.style.display = 'flex';
            
            // Update viewer with current editor content
            content = editor.value;
            renderMarkdown(content);
        }
    });
});

// File operations using native dialogs
openBtn.addEventListener('click', async () => {
    try {
        const filePath = await OpenFileDialog();
        if (!filePath) return;

        const existing = docTabs.find((t) => t.path && pathsEqual(t.path, filePath));
        if (existing) {
            addToRecentFiles(filePath);
            switchToDocTab(existing.id);
            return;
        }

        const fileContent = await GetContent();
        openOrFocusDocument(filePath, fileContent);
    } catch (err) {
        console.error('Error opening file:', err);
        alert('Error opening file: ' + err);
    }
});

saveBtn.addEventListener('click', async () => {
    try {
        flushActiveTabEditor();
        const tab = getActiveDocTab();
        const contentToSave = tab ? tab.value : editor.value;
        const savedPath = await SaveFileDialog(contentToSave);
        
        if (savedPath && tab) {
            tab.path = savedPath;
            tab.title = fileTitleFromPath(savedPath);
            tab.value = contentToSave;
            editor.value = contentToSave;
            content = contentToSave;
            addToRecentFiles(savedPath);
            filePathEl.textContent = savedPath;
            syncGoActiveDocument();
            renderDocTabsBar();
            alert('File saved successfully!');
        }
    } catch (err) {
        console.error('Error saving file:', err);
        alert('Error saving file: ' + err);
    }
});

// Editor line numbers
function updateLineNumbers() {
    const lineCount = editor.value.split('\n').length;
    let numbers = '';
    for (let i = 1; i <= lineCount; i++) {
        numbers += i + '\n';
    }
    lineNumbers.textContent = numbers;
}

editor.addEventListener('input', () => {
    const tab = getActiveDocTab();
    if (tab) tab.value = editor.value;
    updateLineNumbers();
    const text = editor.value;
    wordCountEl.textContent = `${countWords(text)} words`;
    content = text;
    speakBtn.disabled = !text.trim();
});

editor.addEventListener('scroll', () => {
    lineNumbers.scrollTop = editor.scrollTop;
});

// Word count
function countWords(text) {
    return text.trim().split(/\s+/).filter(w => w.length > 0).length;
}

// Improved Markdown Parser
function renderMarkdown(text) {
    if (!text.trim()) {
        viewer.innerHTML = `
            <div class="no-file">
                <div class="no-file-icon">📄</div>
                <div class="no-file-text">No content</div>
                <div class="no-file-hint">Start typing in the Editor tab or open a file</div>
            </div>
        `;
        return;
    }

    const lines = text.split('\n');
    let html = '';
    let inCodeBlock = false;
    let codeContent = '';
    let codeLanguage = '';
    let inList = false;
    let listType = '';
    let inTable = false;
    let tableHeaders = [];
    let tableRows = [];
    let tableStartLine = -1;

    for (let i = 0; i < lines.length; i++) {
        let line = lines[i];
        const lineNum = i;
        
        // Code blocks
        if (line.startsWith('```')) {
            if (inCodeBlock) {
                html += `<pre data-line="${lineNum}"><code>${escapeHtml(codeContent.trim())}</code></pre>`;
                codeContent = '';
                codeLanguage = '';
                inCodeBlock = false;
            } else {
                codeLanguage = line.slice(3).trim();
                inCodeBlock = true;
            }
            continue;
        }

        if (inCodeBlock) {
            codeContent += line + '\n';
            continue;
        }

        // Table detection
        const isTableRow = line.includes('|') && line.trim().startsWith('|') && line.trim().endsWith('|');
        const isTableSeparator = /^\|[\s\-:|]+\|$/.test(line.trim());
        
        if (isTableRow || isTableSeparator) {
            if (!inTable) {
                // Start of table
                inTable = true;
                tableStartLine = lineNum;
                tableHeaders = line.split('|').map(cell => cell.trim()).filter(cell => cell !== '');
            } else if (isTableSeparator) {
                // Skip separator line
                continue;
            } else {
                // Table data row
                const cells = line.split('|').map(cell => cell.trim()).filter(cell => cell !== '');
                tableRows.push(cells);
            }
            continue;
        } else if (inTable) {
            // End of table - render it
            html += renderTable(tableHeaders, tableRows, tableStartLine);
            inTable = false;
            tableHeaders = [];
            tableRows = [];
            tableStartLine = -1;
        }

        // Headers
        if (line.startsWith('###### ')) {
            html += `<h6 data-line="${lineNum}" class="clickable-line">${processInline(line.slice(7))}</h6>`;
        } else if (line.startsWith('##### ')) {
            html += `<h5 data-line="${lineNum}" class="clickable-line">${processInline(line.slice(6))}</h5>`;
        } else if (line.startsWith('#### ')) {
            html += `<h4 data-line="${lineNum}" class="clickable-line">${processInline(line.slice(5))}</h4>`;
        } else if (line.startsWith('### ')) {
            html += `<h3 data-line="${lineNum}" class="clickable-line">${processInline(line.slice(4))}</h3>`;
        } else if (line.startsWith('## ')) {
            html += `<h2 data-line="${lineNum}" class="clickable-line">${processInline(line.slice(3))}</h2>`;
        } else if (line.startsWith('# ')) {
            html += `<h1 data-line="${lineNum}" class="clickable-line">${processInline(line.slice(2))}</h1>`;
        }
        // Blockquote
        else if (line.startsWith('> ')) {
            html += `<blockquote data-line="${lineNum}" class="clickable-line">${processInline(line.slice(2))}</blockquote>`;
        }
        // Horizontal rule
        else if (line.match(/^[\-\*_]{3,}$/)) {
            html += `<hr data-line="${lineNum}">`;
        }
        // Unordered list
        else if (line.match(/^[\-\*\+] /)) {
            const item = line.slice(2);
            if (!inList || listType !== 'ul') {
                if (inList) html += listType === 'ul' ? '</ul>' : '</ol>';
                html += '<ul>';
                inList = true;
                listType = 'ul';
            }
            html += `<li data-line="${lineNum}" class="clickable-line">${processInline(item)}</li>`;
        }
        // Ordered list
        else if (line.match(/^\d+\. /)) {
            const item = line.replace(/^\d+\. /, '');
            if (!inList || listType !== 'ol') {
                if (inList) html += listType === 'ul' ? '</ul>' : '</ol>';
                html += '<ol>';
                inList = true;
                listType = 'ol';
            }
            html += `<li data-line="${lineNum}" class="clickable-line">${processInline(item)}</li>`;
        }
        // Empty line
        else if (!line.trim()) {
            if (inList) {
                html += listType === 'ul' ? '</ul>' : '</ol>';
                inList = false;
                listType = '';
            }
        }
        // Paragraph
        else {
            if (inList) {
                html += listType === 'ul' ? '</ul>' : '</ol>';
                inList = false;
                listType = '';
            }
            html += `<p data-line="${lineNum}" class="clickable-line">${processInline(line)}</p>`;
        }
    }

    // Close any open lists
    if (inList) {
        html += listType === 'ul' ? '</ul>' : '</ol>';
    }

    // Close any open table
    if (inTable) {
        html += renderTable(tableHeaders, tableRows, tableStartLine);
    }

    viewer.innerHTML = html;

    // Add click listeners to all lines
    attachClickListeners();
}

function renderTable(headers, rows, startLine) {
    let tableHtml = '<table class="md-table" data-line="' + startLine + '">';
    
    // Table header
    if (headers.length > 0) {
        tableHtml += '<thead><tr>';
        headers.forEach(header => {
            tableHtml += `<th>${processInline(header)}</th>`;
        });
        tableHtml += '</tr></thead>';
    }
    
    // Table body
    if (rows.length > 0) {
        tableHtml += '<tbody>';
        rows.forEach(row => {
            tableHtml += '<tr>';
            row.forEach(cell => {
                tableHtml += `<td>${processInline(cell)}</td>`;
            });
            tableHtml += '</tr>';
        });
        tableHtml += '</tbody>';
    }
    
    tableHtml += '</table>';
    return tableHtml;
}

function attachClickListeners() {
    document.querySelectorAll('.clickable-line').forEach(line => {
        line.addEventListener('click', (e) => {
            selectedLineIndex = parseInt(e.currentTarget.dataset.line);

            // Remove previous selection
            document.querySelectorAll('.clickable-line.selected').forEach(l => l.classList.remove('selected'));
            e.currentTarget.classList.add('selected');

            console.log('Selected line:', selectedLineIndex);
        });

        line.addEventListener('dblclick', async (e) => {
            selectedLineIndex = parseInt(e.currentTarget.dataset.line);

            // Remove previous selection
            document.querySelectorAll('.clickable-line.selected').forEach(l => l.classList.remove('selected'));
            e.currentTarget.classList.add('selected');

            // Stop current speech if any
            if (isSpeaking) {
                shouldStopSpeaking = true;
                await Stop();
                await sleep(300);
            }

            // Start speaking from this line
            speakBtn.click();
        });
    });
}

function processInline(text) {
    // Bold
    text = text.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
    text = text.replace(/__(.+?)__/g, '<strong>$1</strong>');
    // Italic
    text = text.replace(/\*(.+?)\*/g, '<em>$1</em>');
    text = text.replace(/_(.+?)_/g, '<em>$1</em>');
    // Inline code
    text = text.replace(/`(.+?)`/g, '<code>$1</code>');
    // Links
    text = text.replace(/\[(.+?)\]\((.+?)\)/g, '<a href="$2" target="_blank">$1</a>');
    return text;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// TTS Functions
speakBtn.addEventListener('click', async () => {
    if (isSpeaking) return;
    
    if (!content) {
        alert('Please open a file first');
        return;
    }

    try {
        isSpeaking = true;
        shouldStopSpeaking = false;
        speakBtn.disabled = true;
        stopBtn.disabled = false;
        loadingIndicator.classList.add('active');
        readingStatusEl.textContent = 'Reading...';

        const voice = voiceSelect.value;
        const rate = parseInt(rateSlider.value);
        
        // Get all lines from content
        currentLines = content.split('\n');
        
        // Start speaking from selected line
        await speakLinesSequentially(selectedLineIndex, voice, rate);
        
    } catch (err) {
        console.error('TTS Error:', err);
        alert('TTS Error: ' + err);
    } finally {
        stopSpeaking();
    }
});

stopBtn.addEventListener('click', async () => {
    shouldStopSpeaking = true;
    try {
        await Stop();
    } catch (err) {
        console.error('Stop Error:', err);
    }
    stopSpeaking();
});

async function speakLinesSequentially(startLine, voice, rate) {
    for (let i = startLine; i < currentLines.length; i++) {
        // Check if we should stop
        if (shouldStopSpeaking) {
            console.log('Stopping speech at line', i);
            break;
        }
        
        let line = currentLines[i];
        
        // Skip empty lines
        if (!line.trim()) {
            continue;
        }
        
        // Clean markdown syntax before speaking
        const originalLine = line;
        line = cleanMarkdownForSpeech(line);
        
        // Debug: Log what we're processing
        if (originalLine.startsWith('-') || originalLine.startsWith('*') || originalLine.startsWith('+')) {
            console.log('Bullet point detected:');
            console.log('  Original:', originalLine);
            console.log('  Cleaned:', line);
        }
        
        // Skip if nothing left after cleaning
        if (!line.trim()) {
            console.log('  Skipping (empty after cleaning)');
            continue;
        }
        
        // Highlight current line
        highlightLine(i);
        
        // Update status
        readingStatusEl.textContent = `Reading line ${i + 1} of ${currentLines.length}...`;
        
        try {
            // Speak this line and wait for it to complete
            await SpeakLine(line, voice || '', rate);
        } catch (err) {
            console.error('Error speaking line', i, ':', err);
            // Check if it was intentionally stopped
            if (shouldStopSpeaking) {
                break;
            }
            // Otherwise continue to next line
        }
        
        // Check again after speaking (in case stop was pressed during speech)
        if (shouldStopSpeaking) {
            break;
        }
        
        // Small pause between lines (backend already has 500ms delay after audio)
        await sleep(100);
    }
}

function cleanMarkdownForSpeech(text) {
    // Remove markdown formatting for TTS
    let cleaned = text;
    
    // Remove headers (# ## ### etc)
    cleaned = cleaned.replace(/^#{1,6}\s+/, '');
    
    // Remove bullet points (- * +) including indented ones
    cleaned = cleaned.replace(/^\s*[\-\*\+]\s+/, '');
    
    // Remove numbered lists (1. 2. etc) including indented ones
    cleaned = cleaned.replace(/^\s*\d+\.\s+/, '');
    
    // Remove blockquote markers (>)
    cleaned = cleaned.replace(/^>\s+/, '');
    
    // Remove bold/italic markers but keep the text
    cleaned = cleaned.replace(/\*\*(.+?)\*\*/g, '$1');
    cleaned = cleaned.replace(/__(.+?)__/g, '$1');
    cleaned = cleaned.replace(/\*(.+?)\*/g, '$1');
    cleaned = cleaned.replace(/_(.+?)_/g, '$1');
    
    // Remove inline code markers
    cleaned = cleaned.replace(/`(.+?)`/g, '$1');
    
    // Remove links but keep the text
    cleaned = cleaned.replace(/\[(.+?)\]\((.+?)\)/g, '$1');
    
    // Remove table separators (|---|---|)
    if (cleaned.match(/^\|[\s\-:|]+\|$/)) {
        return '';
    }
    
    // Clean up table cell separators for better reading
    if (cleaned.includes('|')) {
        cleaned = cleaned.replace(/\|/g, ', ').replace(/,\s*,/g, ',');
        cleaned = cleaned.replace(/^,\s*/, '').replace(/,\s*$/, '');
    }
    
    return cleaned.trim();
}

function stopSpeaking() {
    isSpeaking = false;
    shouldStopSpeaking = true;
    speakBtn.disabled = false;
    stopBtn.disabled = true;
    loadingIndicator.classList.remove('active');
    readingStatusEl.textContent = '';
    clearHighlight();
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

function highlightLine(lineNum) {
    clearHighlight();
    const elements = document.querySelectorAll(`[data-line="${lineNum}"]`);
    elements.forEach(el => {
        el.classList.add('line-reading');
        // Scroll the highlighted line into view
        el.scrollIntoView({ behavior: 'smooth', block: 'center' });
    });
}

function clearHighlight() {
    document.querySelectorAll('.line-reading').forEach(el => {
        el.classList.remove('line-reading');
    });
}

function getSelectedText() {
    // Check editor first if focused
    if (document.activeElement === editor) {
        const start = editor.selectionStart;
        const end = editor.selectionEnd;
        if (start !== end) {
            return editor.value.substring(start, end);
        }
    }
    // Check window selection (viewer or elsewhere)
    const sel = window.getSelection();
    if (sel && sel.toString().trim()) {
        return sel.toString();
    }
    return '';
}

function hasEditorSelection() {
    return document.activeElement === editor && editor.selectionStart !== editor.selectionEnd;
}

// Context menu handling
function showContextMenu(x, y, target) {
    const selection = getSelectedText();
    const editorSelection = hasEditorSelection();
    const isEditorTarget = target === editor || editor.contains(target);

    const items = contextMenu.querySelectorAll('.context-menu-item');
    items.forEach(item => {
        const action = item.dataset.action;
        item.classList.remove('disabled');
        switch (action) {
            case 'speak':
                if (!selection) item.classList.add('disabled');
                break;
            case 'copy':
                if (!selection) item.classList.add('disabled');
                break;
            case 'cut':
            case 'delete':
                if (!editorSelection) item.classList.add('disabled');
                break;
            case 'paste':
                if (!isEditorTarget) item.classList.add('disabled');
                break;
        }
    });

    contextMenu.style.display = 'block';
    contextMenu.style.left = x + 'px';
    contextMenu.style.top = y + 'px';

    const rect = contextMenu.getBoundingClientRect();
    if (rect.right > window.innerWidth) {
        contextMenu.style.left = (x - rect.width) + 'px';
    }
    if (rect.bottom > window.innerHeight) {
        contextMenu.style.top = (y - rect.height) + 'px';
    }
}

function hideContextMenu() {
    contextMenu.style.display = 'none';
}

function isWordChar(ch) {
    if (!ch) return false;
    try {
        return /^[\p{L}\p{N}_]$/u.test(ch);
    } catch {
        return /[A-Za-z0-9_]/.test(ch);
    }
}

function findAllMatches(text, needle, matchCase, wholeWord) {
    const results = [];
    if (!needle) return results;

    const hay = matchCase ? text : text.toLowerCase();
    const n = matchCase ? needle : needle.toLowerCase();
    const len = n.length;
    let pos = 0;

    while (pos <= hay.length - len) {
        const idx = hay.indexOf(n, pos);
        if (idx === -1) break;

        const afterIdx = idx + len;
        if (wholeWord) {
            const beforeCh = idx > 0 ? text[idx - 1] : '';
            const afterCh = afterIdx < text.length ? text[afterIdx] : '';
            if (isWordChar(beforeCh) || isWordChar(afterCh)) {
                pos = idx + 1;
                continue;
            }
        }

        results.push({ start: idx, end: afterIdx });
        pos = afterIdx;
    }
    return results;
}

function lineNumberAtIndex(text, index) {
    return text.slice(0, index).split('\n').length;
}

function buildFindSnippet(text, start, end, maxLen) {
    const lineStart = text.lastIndexOf('\n', start - 1) + 1;
    const lineEndIdx = text.indexOf('\n', end);
    const lineEnd = lineEndIdx === -1 ? text.length : lineEndIdx;
    let segment = text.slice(lineStart, lineEnd);
    if (segment.length <= maxLen) return segment;

    const rel = start - lineStart;
    const half = Math.floor(maxLen / 2);
    let sliceStart = Math.max(0, rel - half);
    let sliceEnd = Math.min(segment.length, sliceStart + maxLen);
    if (sliceEnd - sliceStart < maxLen) {
        sliceStart = Math.max(0, sliceEnd - maxLen);
    }
    segment = segment.slice(sliceStart, sliceEnd);
    const prefix = sliceStart > 0 ? '…' : '';
    const suffix = sliceEnd < lineEnd - lineStart ? '…' : '';
    return prefix + segment + suffix;
}

function scrollEditorToIndex(index) {
    const style = getComputedStyle(editor);
    const lh = parseFloat(style.lineHeight);
    const lineHeight = Number.isFinite(lh) && lh > 0 ? lh : parseFloat(style.fontSize) * 1.6;
    const padTop = parseFloat(style.paddingTop) || 0;
    const textBefore = editor.value.substring(0, index);
    const lineIndex = textBefore.split('\n').length - 1;

    const lineTop = lineIndex * lineHeight + padTop;
    let targetScroll;
    if (lineIndex === 0) {
        targetScroll = 0;
    } else {
        const viewportMid = editor.clientHeight * 0.5;
        targetScroll = lineTop - viewportMid + lineHeight * 0.5;
    }

    const maxScroll = Math.max(0, editor.scrollHeight - editor.clientHeight);
    editor.scrollTop = Math.max(0, Math.min(maxScroll, targetScroll));
    lineNumbers.scrollTop = editor.scrollTop;
}

function ensureEditorTab() {
    const editorTab = document.querySelector('.tab[data-tab="editor"]');
    if (editorTab && !editorTab.classList.contains('active')) {
        editorTab.click();
    }
}

function openFindDialog() {
    hideContextMenu();
    let sel = '';
    if (document.activeElement === editor) {
        const a = editor.selectionStart;
        const b = editor.selectionEnd;
        if (a !== b) sel = editor.value.substring(a, b);
    }
    if (!sel) {
        const winSel = window.getSelection();
        if (winSel && winSel.toString()) sel = winSel.toString();
    }
    if (sel && !sel.includes('\n') && sel.length < 200) {
        findInput.value = sel.trim();
    }
    findDialogOverlay.classList.add('open');
    findDialogOverlay.setAttribute('aria-hidden', 'false');
    setTimeout(() => {
        findInput.focus();
        findInput.select();
    }, 0);
}

function closeFindDialog() {
    findDialogOverlay.classList.remove('open');
    findDialogOverlay.setAttribute('aria-hidden', 'true');
}

function closeFindPanel() {
    findResultsPanel.classList.remove('open');
    findResultsPanel.setAttribute('aria-hidden', 'true');
    findMatches = [];
    findResultsList.innerHTML = '';
}

function renderFindResults(matches, query) {
    findResultsList.innerHTML = '';
    const trimmed = query.trim();
    findResultsTitle.textContent = matches.length
        ? `${matches.length} result${matches.length === 1 ? '' : 's'}`
        : 'Find results';

    if (!trimmed) {
        const empty = document.createElement('div');
        empty.className = 'find-results-empty';
        empty.textContent = 'Enter text to search.';
        findResultsList.appendChild(empty);
        return;
    }

    if (matches.length === 0) {
        const empty = document.createElement('div');
        empty.className = 'find-results-empty';
        empty.textContent = 'No matches.';
        findResultsList.appendChild(empty);
        return;
    }

    const text = editor.value;
    matches.forEach((m, i) => {
        const line = lineNumberAtIndex(text, m.start);
        const el = document.createElement('div');
        el.className = 'find-result-item';
        el.dataset.index = String(i);
        const lineEl = document.createElement('div');
        lineEl.className = 'find-result-line';
        lineEl.textContent = `Line ${line}`;
        const sn = document.createElement('div');
        sn.className = 'find-result-snippet';
        sn.textContent = buildFindSnippet(text, m.start, m.end, 120);
        el.appendChild(lineEl);
        el.appendChild(sn);
        el.addEventListener('click', () => focusFindResult(i));
        findResultsList.appendChild(el);
    });
}

function focusFindResult(index) {
    const m = findMatches[index];
    if (!m) return;

    ensureEditorTab();

    findResultsList.querySelectorAll('.find-result-item').forEach((el, i) => {
        el.classList.toggle('active', i === index);
    });

    editor.focus();
    scrollEditorToIndex(m.start);

    const applySelection = () => {
        editor.setSelectionRange(m.start, m.end, 'forward');
    };
    applySelection();
    requestAnimationFrame(() => {
        applySelection();
        requestAnimationFrame(() => {
            applySelection();
            editor.classList.add('editor-find-flash');
            window.setTimeout(() => editor.classList.remove('editor-find-flash'), 900);
        });
    });
}

function runFind() {
    const query = findInput.value;
    if (!query.trim()) {
        findInput.focus();
        return;
    }

    const matchCase = findMatchCase.checked;
    const wholeWord = findWholeWord.checked;

    findMatches = findAllMatches(editor.value, query, matchCase, wholeWord);
    closeFindDialog();

    findResultsPanel.classList.add('open');
    findResultsPanel.setAttribute('aria-hidden', 'false');
    renderFindResults(findMatches, query);
}

findToolbarBtn.addEventListener('click', () => openFindDialog());
findCancelBtn.addEventListener('click', () => closeFindDialog());
findRunBtn.addEventListener('click', () => runFind());

findDialogOverlay.addEventListener('click', (e) => {
    if (e.target === findDialogOverlay) closeFindDialog();
});

findResultsClose.addEventListener('click', () => closeFindPanel());

findInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
        e.preventDefault();
        runFind();
    } else if (e.key === 'Escape') {
        e.preventDefault();
        closeFindDialog();
    }
});

document.addEventListener('keydown', (e) => {
    const isFindShortcut = (e.ctrlKey || e.metaKey) && (e.key === 'f' || e.key === 'F');
    if (isFindShortcut) {
        e.preventDefault();
        openFindDialog();
        return;
    }
    if (e.key === 'Escape') {
        if (findDialogOverlay.classList.contains('open')) {
            e.preventDefault();
            closeFindDialog();
            return;
        }
        if (findResultsPanel.classList.contains('open')) {
            e.preventDefault();
            closeFindPanel();
        }
    }
}, true);

// Prevent native context menu and show custom one
document.addEventListener('contextmenu', (e) => {
    e.preventDefault();
    showContextMenu(e.pageX, e.pageY, e.target);
});

document.addEventListener('click', (e) => {
    if (!contextMenu.contains(e.target)) {
        hideContextMenu();
    }
    if (recentMenuWrap && !recentMenuWrap.contains(e.target)) {
        closeRecentMenu();
    }
});

contextMenu.addEventListener('click', async (e) => {
    const item = e.target.closest('.context-menu-item');
    if (!item || item.classList.contains('disabled')) return;

    const action = item.dataset.action;
    hideContextMenu();

    const selection = getSelectedText();
    const start = editor.selectionStart;
    const end = editor.selectionEnd;
    const selectedText = editor.value.substring(start, end);

    switch (action) {
        case 'copy': {
            const textToCopy = selectedText || selection;
            if (textToCopy) {
                try {
                    await navigator.clipboard.writeText(textToCopy);
                } catch (err) {
                    console.error('Copy failed:', err);
                }
            }
            break;
        }
        case 'cut': {
            if (selectedText) {
                try {
                    await navigator.clipboard.writeText(selectedText);
                    editor.setRangeText('', start, end);
                    editor.dispatchEvent(new Event('input'));
                } catch (err) {
                    console.error('Cut failed:', err);
                }
            }
            break;
        }
        case 'paste': {
            if (document.activeElement !== editor) {
                editor.focus();
            }
            try {
                const text = await navigator.clipboard.readText();
                editor.setRangeText(text, editor.selectionStart, editor.selectionEnd, 'end');
                editor.dispatchEvent(new Event('input'));
            } catch (err) {
                console.error('Paste failed:', err);
            }
            break;
        }
        case 'delete': {
            if (selectedText) {
                editor.setRangeText('', start, end);
                editor.dispatchEvent(new Event('input'));
            }
            break;
        }
        case 'speak': {
            const textToSpeak = selectedText || selection;
            if (!textToSpeak) return;

            // Stop any ongoing line-by-line reading
            if (isSpeaking) {
                shouldStopSpeaking = true;
                try { await Stop(); } catch (err) { /* ignore */ }
                await sleep(200);
            }

            const voice = voiceSelect.value;
            const rate = parseInt(rateSlider.value);

            readingStatusEl.textContent = 'Speaking selection...';
            loadingIndicator.classList.add('active');

            try {
                await SpeakLine(textToSpeak, voice || '', rate);
            } catch (err) {
                console.error('Speak error:', err);
            } finally {
                readingStatusEl.textContent = '';
                loadingIndicator.classList.remove('active');
            }
            break;
        }
    }
});

// Voice selection
async function loadVoices() {
    try {
        const voices = await GetAvailableVoices();
        voiceSelect.innerHTML = '<option value="">Default Voice</option>';
        voices.forEach(voice => {
            const option = document.createElement('option');
            // Backend may return "Name|Description" format
            const sepIndex = voice.indexOf('|');
            if (sepIndex > 0) {
                const name = voice.substring(0, sepIndex);
                const desc = voice.substring(sepIndex + 1);
                option.value = name;
                option.textContent = name + ' — ' + desc;
            } else {
                option.value = voice;
                option.textContent = voice;
            }
            voiceSelect.appendChild(option);
        });
    } catch (err) {
        console.error('Error loading voices:', err);
    }
}

// Speech rate slider
if (rateSlider && rateValue) {
    rateSlider.addEventListener('input', (e) => {
        rateValue.textContent = e.target.value;
    });
}

// Initialize
updateLineNumbers();
loadVoices();

renderRecentMenu();

EventsOn('document-disk-changed', (payload) => {
    let data;
    try {
        data = typeof payload === 'string' ? JSON.parse(payload) : payload;
    } catch {
        return;
    }
    const diskPath = data && data.path;
    const newContent = data && data.content;
    if (!diskPath || typeof newContent !== 'string') return;

    docTabs.forEach((tab) => {
        if (tab.path && pathsEqual(tab.path, diskPath)) {
            tab.value = newContent;
        }
    });

    const active = getActiveDocTab();
    if (active && active.path && pathsEqual(active.path, diskPath)) {
        closeFindPanel();
        editor.value = newContent;
        content = newContent;
        updateLineNumbers();
        wordCountEl.textContent = `${countWords(newContent)} words`;
        speakBtn.disabled = !newContent.trim();
        if (viewerContent.classList.contains('active')) {
            renderMarkdown(newContent);
        }
        SetActiveDocument(active.path, newContent);
    }
});

(async function initFromStartupOrWelcome() {
    try {
        const path = await GetCurrentFile();
        const fileContent = await GetContent();
        if (path) {
            docTabs = [];
            docTabIdSeq = 0;
            activeDocTabId = null;
            openOrFocusDocument(path, fileContent);
            return;
        }
    } catch (err) {
        console.error('Startup file load:', err);
    }
    if (docTabs.length === 0) {
        ensureDefaultUntitledTab();
    }
    viewer.innerHTML = `
    <div class="no-file">
        <div class="no-file-icon">📄</div>
        <div class="no-file-text">Welcome to MDViewer</div>
        <div class="no-file-hint">Click "Open" to load a markdown file</div>
    </div>
`;
})();
