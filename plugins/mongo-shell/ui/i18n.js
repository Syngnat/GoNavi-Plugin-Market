/* Host-driven localization for the MongoShell window.
 *
 * The host injects window.gonaviHostPrefs = { locale, theme } before the page
 * loads (same payload the plugin pages get), so the shell never needs a bridge
 * round-trip to pick a language. Missing or unsupported locales fall back to
 * the English dictionary, which is also the source of every key below. */

const SUPPORTED = ['zh-CN', 'zh-TW', 'en-US', 'ja-JP', 'de-DE', 'ru-RU'];

const MESSAGES = {
  'en-US': {
    'brand.loading': 'Loading connection…',
    'chip.readonly': 'Read-only',
    'status.starting': 'Starting…',
    'status.unavailable': 'Unavailable',
    'status.connecting': 'Connecting…',
    'status.connected': 'Connected',
    'status.connected_readonly': 'Connected · read-only',
    'status.connected_latency': 'Connected · {{ms}} ms',
    'status.connection_failed': 'Connection failed',
    'status.disconnected': 'Disconnected',
    'status.ping_failed': 'Ping failed',
    'status.running': 'Running…',
    'status.run_failed': 'Run failed',
    'action.ping': 'Ping',
    'action.connect': 'Connect',
    'action.disconnect': 'Disconnect',
    'action.dismiss': 'Dismiss',
    'action.clear': 'Clear',
    'action.run': 'Run',
    'field.database': 'Database',
    'field.collection': 'Collection',
    'field.collection_placeholder': 'optional collection',
    'field.max_rows': 'Max rows',
    'pane.script': 'Script',
    'pane.result': 'Result',
    'pane.output': 'Output',
    'result.empty': 'Run a script to see documents here.',
    'result.none': 'No query yet',
    'footer.database': 'Database: {{name}}',
    'footer.writes_enabled': 'Writes enabled',
    'footer.readonly': 'Read-only',
    'error.empty_script': 'Write a script before running it.',
    'error.invalid_context': 'This window was not opened from a MongoDB connection. Open it from the connection menu instead.',
    'error.bridge_unavailable': 'The GoNavi plugin bridge is not available in this window.',
    'result.previous': "The previous result stays visible while the next script runs.",
    'result.empty_title': "No result",
    'error.missing_credentials': "The host did not pass the connection credentials. Configure them and reopen this window.",
    'result.truncated': "Row limit reached: showing the first {{rows}} rows (max {{max}}).",
    'result.matched_none': "The query matched no documents.",
    'result.no_columns': "{{count}} value(s) returned, but these documents have no fields.",
    'guidance.title': "Open MongoShell from a connection",
    'guidance.body': "In the GoNavi sidebar, right-click a MongoDB connection and choose MongoShell, or MongoShell (Workbench) to open it in a tab.",
  },
  'zh-CN': {
    'brand.loading': '正在读取连接信息…',
    'chip.readonly': '只读',
    'status.starting': '正在启动…',
    'status.unavailable': '不可用',
    'status.connecting': '正在连接…',
    'status.connected': '已连接',
    'status.connected_readonly': '已连接 · 只读',
    'status.connected_latency': '已连接 · {{ms}} 毫秒',
    'status.connection_failed': '连接失败',
    'status.disconnected': '已断开',
    'status.ping_failed': 'Ping 失败',
    'status.running': '执行中…',
    'status.run_failed': '执行失败',
    'action.ping': 'Ping',
    'action.connect': '连接',
    'action.disconnect': '断开连接',
    'action.dismiss': '关闭',
    'action.clear': '清空',
    'action.run': '运行',
    'field.database': '数据库',
    'field.collection': '集合',
    'field.collection_placeholder': '可选集合',
    'field.max_rows': '最大行数',
    'pane.script': '脚本',
    'pane.result': '结果',
    'pane.output': '输出',
    'result.empty': '运行脚本后，命中结果会显示在这里。',
    'result.none': '尚未查询',
    'footer.database': '数据库：{{name}}',
    'footer.writes_enabled': '允许写入',
    'footer.readonly': '只读',
    'error.empty_script': '请先写一段脚本再运行。',
    'error.invalid_context': '这个窗口不是从 MongoDB 连接打开的，请改用连接右键菜单进入。',
    'error.bridge_unavailable': '当前窗口里没有可用的 GoNavi 插件通道。',
    'result.previous': "上一次结果会在新脚本执行期间保留。",
    'result.empty_title': "暂无结果",
    'error.missing_credentials': "宿主没有把连接凭据传进来，请确认该连接已配置凭据后重新打开窗口。",
    'result.truncated': "已达到行数上限，仅显示前 {{rows}} 行 / 上限 {{max}}。",
    'result.matched_none': "该查询没有命中任何文档。",
    'result.no_columns': "返回 {{count}} 条值，但这些文档没有字段列。",
    'guidance.title': "请从 MongoDB 连接打开 MongoShell",
    'guidance.body': "在 GoNavi 侧栏右键一个 MongoDB 连接，选择 MongoShell（独立窗口）或 MongoShell (Workbench)（工作台标签）。",
  },
  'zh-TW': {
    'brand.loading': '正在讀取連線資訊…',
    'chip.readonly': '唯讀',
    'status.starting': '正在啟動…',
    'status.unavailable': '無法使用',
    'status.connecting': '正在連線…',
    'status.connected': '已連線',
    'status.connected_readonly': '已連線 · 唯讀',
    'status.connected_latency': '已連線 · {{ms}} 毫秒',
    'status.connection_failed': '連線失敗',
    'status.disconnected': '已中斷',
    'status.ping_failed': 'Ping 失敗',
    'status.running': '執行中…',
    'status.run_failed': '執行失敗',
    'action.ping': 'Ping',
    'action.connect': '連線',
    'action.disconnect': '中斷連線',
    'action.dismiss': '關閉',
    'action.clear': '清除',
    'action.run': '執行',
    'field.database': '資料庫',
    'field.collection': '集合',
    'field.collection_placeholder': '選填集合',
    'field.max_rows': '最大列數',
    'pane.script': '腳本',
    'pane.result': '結果',
    'pane.output': '輸出',
    'result.empty': '執行腳本後，結果會顯示在這裡。',
    'result.none': '尚未查詢',
    'footer.database': '資料庫：{{name}}',
    'footer.writes_enabled': '允許寫入',
    'footer.readonly': '唯讀',
    'error.empty_script': '請先撰寫腳本再執行。',
    'error.invalid_context': '這個視窗不是從 MongoDB 連線開啟的，請改用連線右鍵選單進入。',
    'error.bridge_unavailable': '目前視窗沒有可用的 GoNavi 外掛通道。',
    'result.previous': "上一次結果會在新腳本執行期間保留。",
    'result.empty_title': "尚無結果",
    'error.missing_credentials': "宿主沒有傳入連線憑證，請確認該連線已設定憑證後重新開啟視窗。",
    'result.truncated': "已達列數上限，僅顯示前 {{rows}} 列 / 上限 {{max}}。",
    'result.matched_none': "此查詢沒有命中任何文件。",
    'result.no_columns': "回傳 {{count}} 筆值，但這些文件沒有欄位。",
    'guidance.title': "請從 MongoDB 連線開啟 MongoShell",
    'guidance.body': "在 GoNavi 側欄右鍵一個 MongoDB 連線，選擇 MongoShell（獨立視窗）或 MongoShell (Workbench)（工作台分頁）。",
  },
  'ja-JP': {
    'brand.loading': '接続情報を読み込み中…',
    'chip.readonly': '読み取り専用',
    'status.starting': '起動中…',
    'status.unavailable': '利用できません',
    'status.connecting': '接続中…',
    'status.connected': '接続済み',
    'status.connected_readonly': '接続済み · 読み取り専用',
    'status.connected_latency': '接続済み · {{ms}} ミリ秒',
    'status.connection_failed': '接続に失敗しました',
    'status.disconnected': '切断済み',
    'status.ping_failed': 'Ping に失敗しました',
    'status.running': '実行中…',
    'status.run_failed': '実行に失敗しました',
    'action.ping': 'Ping',
    'action.connect': '接続',
    'action.disconnect': '切断',
    'action.dismiss': '閉じる',
    'action.clear': 'クリア',
    'action.run': '実行',
    'field.database': 'データベース',
    'field.collection': 'コレクション',
    'field.collection_placeholder': '任意のコレクション',
    'field.max_rows': '最大行数',
    'pane.script': 'スクリプト',
    'pane.result': '結果',
    'pane.output': '出力',
    'result.empty': 'スクリプトを実行すると結果がここに表示されます。',
    'result.none': 'まだクエリはありません',
    'footer.database': 'データベース: {{name}}',
    'footer.writes_enabled': '書き込み可',
    'footer.readonly': '読み取り専用',
    'error.empty_script': 'スクリプトを書いてから実行してください。',
    'error.invalid_context': 'このウィンドウは MongoDB 接続から開かれていません。接続の右クリックメニューから開いてください。',
    'error.bridge_unavailable': 'このウィンドウでは GoNavi プラグインブリッジを利用できません。',
    'result.previous': "次の実行中も前回の結果を表示します。",
    'result.empty_title': "結果なし",
    'error.missing_credentials': "ホストが接続資格情報を渡していません。設定後にこのウィンドウを開き直してください。",
    'result.truncated': "行数上限に達しました。先頭 {{rows}} 行のみ表示します（上限 {{max}}）。",
    'result.matched_none': "このクエリに一致するドキュメントはありません。",
    'result.no_columns': "{{count}} 件の値が返りましたが、これらのドキュメントにフィールドはありません。",
    'guidance.title': "MongoDB 接続から MongoShell を開いてください",
    'guidance.body': "GoNavi のサイドバーで MongoDB 接続を右クリックし、MongoShell（ウィンドウ）または MongoShell (Workbench)（タブ）を選びます。",
  },
  'de-DE': {
    'brand.loading': 'Verbindung wird geladen…',
    'chip.readonly': 'Schreibgeschützt',
    'status.starting': 'Wird gestartet…',
    'status.unavailable': 'Nicht verfügbar',
    'status.connecting': 'Verbindung wird hergestellt…',
    'status.connected': 'Verbunden',
    'status.connected_readonly': 'Verbunden · schreibgeschützt',
    'status.connected_latency': 'Verbunden · {{ms}} ms',
    'status.connection_failed': 'Verbindung fehlgeschlagen',
    'status.disconnected': 'Getrennt',
    'status.ping_failed': 'Ping fehlgeschlagen',
    'status.running': 'Wird ausgeführt…',
    'status.run_failed': 'Ausführung fehlgeschlagen',
    'action.ping': 'Ping',
    'action.connect': 'Verbinden',
    'action.disconnect': 'Trennen',
    'action.dismiss': 'Schließen',
    'action.clear': 'Leeren',
    'action.run': 'Ausführen',
    'field.database': 'Datenbank',
    'field.collection': 'Collection',
    'field.collection_placeholder': 'optionale Collection',
    'field.max_rows': 'Max. Zeilen',
    'pane.script': 'Skript',
    'pane.result': 'Ergebnis',
    'pane.output': 'Ausgabe',
    'result.empty': 'Führen Sie ein Skript aus, um Dokumente hier zu sehen.',
    'result.none': 'Noch keine Abfrage',
    'footer.database': 'Datenbank: {{name}}',
    'footer.writes_enabled': 'Schreiben erlaubt',
    'footer.readonly': 'Schreibgeschützt',
    'error.empty_script': 'Schreiben Sie zuerst ein Skript.',
    'error.invalid_context': 'Dieses Fenster wurde nicht über eine MongoDB-Verbindung geöffnet. Öffnen Sie es über das Kontextmenü der Verbindung.',
    'error.bridge_unavailable': 'Die GoNavi-Plugin-Bridge ist in diesem Fenster nicht verfügbar.',
    'result.previous': "Das vorherige Ergebnis bleibt während der Ausführung sichtbar.",
    'result.empty_title': "Kein Ergebnis",
    'error.missing_credentials': "Der Host hat keine Verbindungsdaten übergeben. Konfigurieren Sie sie und öffnen Sie das Fenster erneut.",
    'result.truncated': "Zeilenlimit erreicht: nur die ersten {{rows}} Zeilen (max {{max}}).",
    'result.matched_none': "Die Abfrage hat keine Dokumente gefunden.",
    'result.no_columns': "{{count}} Wert(e) zurückgegeben, aber diese Dokumente haben keine Felder.",
    'guidance.title': "MongoShell über eine Verbindung öffnen",
    'guidance.body': "In der GoNavi-Seitenleiste mit der rechten Maustaste auf eine MongoDB-Verbindung klicken und MongoShell oder MongoShell (Workbench) wählen.",
  },
  'ru-RU': {
    'brand.loading': 'Загрузка подключения…',
    'chip.readonly': 'Только чтение',
    'status.starting': 'Запуск…',
    'status.unavailable': 'Недоступно',
    'status.connecting': 'Подключение…',
    'status.connected': 'Подключено',
    'status.connected_readonly': 'Подключено · только чтение',
    'status.connected_latency': 'Подключено · {{ms}} мс',
    'status.connection_failed': 'Не удалось подключиться',
    'status.disconnected': 'Отключено',
    'status.ping_failed': 'Ошибка Ping',
    'status.running': 'Выполняется…',
    'status.run_failed': 'Ошибка выполнения',
    'action.ping': 'Ping',
    'action.connect': 'Подключить',
    'action.disconnect': 'Отключить',
    'action.dismiss': 'Закрыть',
    'action.clear': 'Очистить',
    'action.run': 'Выполнить',
    'field.database': 'База данных',
    'field.collection': 'Коллекция',
    'field.collection_placeholder': 'необязательная коллекция',
    'field.max_rows': 'Максимум строк',
    'pane.script': 'Скрипт',
    'pane.result': 'Результат',
    'pane.output': 'Вывод',
    'result.empty': 'Выполните скрипт, чтобы увидеть документы.',
    'result.none': 'Запросов пока нет',
    'footer.database': 'База данных: {{name}}',
    'footer.writes_enabled': 'Запись разрешена',
    'footer.readonly': 'Только чтение',
    'error.empty_script': 'Сначала напишите скрипт.',
    'error.invalid_context': 'Это окно открыто не из подключения MongoDB. Откройте его из контекстного меню подключения.',
    'error.bridge_unavailable': 'Мост плагинов GoNavi недоступен в этом окне.',
    'result.previous': "Предыдущий результат остаётся видимым во время выполнения.",
    'result.empty_title': "Нет результата",
    'error.missing_credentials': "Хост не передал учётные данные подключения. Настройте их и откройте окно заново.",
    'result.truncated': "Достигнут лимит строк: показаны первые {{rows}} (максимум {{max}}).",
    'result.matched_none': "Запрос не нашёл ни одного документа.",
    'result.no_columns': "Возвращено значений: {{count}}, но у этих документов нет полей.",
    'guidance.title': "Откройте MongoShell из подключения",
    'guidance.body': "В боковой панели GoNavi щёлкните правой кнопкой по подключению MongoDB и выберите MongoShell или MongoShell (Workbench).",
  },
};

/** Host-injected locale first, then the browser, then English. */
export function hostLocale() {
  const prefs = globalThis.gonaviHostPrefs;
  const raw = prefs && typeof prefs.locale === 'string' && prefs.locale
    ? prefs.locale
    : (typeof navigator !== 'undefined' && typeof navigator.language === 'string' ? navigator.language : '');
  if (SUPPORTED.includes(raw)) return raw;
  const lower = raw.toLowerCase();
  if (lower.startsWith('zh')) return lower.includes('tw') || lower.includes('hk') ? 'zh-TW' : 'zh-CN';
  const base = lower.split('-')[0];
  if (!base) return 'en-US';
  return SUPPORTED.find((item) => item.toLowerCase().startsWith(`${base}-`)) || 'en-US';
}

let active = hostLocale();

/** Re-resolve the locale; the host may push preferences after load. */
export function useHostLocale() {
  active = hostLocale();
  return active;
}

export function t(key, params) {
  const table = MESSAGES[active] || MESSAGES['en-US'];
  const template = table[key] || MESSAGES['en-US'][key] || key;
  if (!params) return template;
  return template.replace(/\{\{(\w+)\}\}/g, (match, name) =>
    Object.prototype.hasOwnProperty.call(params, name)
      ? String(params[name])
      : match);
}

/** Fill every [data-i18n] element; placeholders and aria labels use their own attributes. */
export function applyStaticText(root = typeof document === 'undefined' ? undefined : document) {
  if (!root || typeof root.querySelectorAll !== 'function') return;
  for (const node of root.querySelectorAll('[data-i18n]')) {
    node.textContent = t(node.dataset.i18n);
  }
  for (const node of root.querySelectorAll('[data-i18n-placeholder]')) {
    node.setAttribute('placeholder', t(node.dataset.i18nPlaceholder));
  }
  for (const node of root.querySelectorAll('[data-i18n-label]')) {
    node.setAttribute('aria-label', t(node.dataset.i18nLabel));
  }
  if (typeof document !== 'undefined' && document.documentElement) {
    document.documentElement.lang = active;
  }
}

/** Mirror the host appearance on <html data-theme> so the tokens switch with the app. */
export function applyHostTheme() {
  const prefs = globalThis.gonaviHostPrefs;
  const theme = prefs && (prefs.theme === 'dark' || prefs.theme === 'light') ? prefs.theme : '';
  const root = typeof document === 'undefined' ? null : document.documentElement;
  if (!root) return;
  if (theme) root.dataset.theme = theme;
  else delete root.dataset.theme;
}

/** Localized text for the error codes the shell raises itself. */
export function errorText(error) {
  const code = error && typeof error.code === 'string' ? error.code : '';
  const key = `error.${code}`;
  const table = MESSAGES[active] || MESSAGES['en-US'];
  if (code && table[key]) return t(key);
  return error && error.message ? error.message : String(error);
}

export const supportedLocales = SUPPORTED;
