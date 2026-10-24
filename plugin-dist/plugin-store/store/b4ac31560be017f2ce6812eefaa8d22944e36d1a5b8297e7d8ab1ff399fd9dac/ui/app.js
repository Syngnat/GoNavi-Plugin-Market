/* Nginx Inspector window UI. Talks to the backend only through
 * window.gonaviPlugin.invoke; no network, no storage beyond the selected
 * connection id. */
"use strict";

const PLUGIN_ID = "com.gonavi.nginx-inspector";
const STORE_KEY = "nginxInspector.connectionId";

/* ---------------- i18n ---------------- */

const STRINGS = {
  "zh-CN": {
    pluginSubtitle: "只读 Nginx 诊断",
    conn: "连接",
    noConn: "未选择连接",
    addConn: "新建连接…",
    manage: "连接管理",
    overview: "概览",
    config: "配置",
    validate: "校验",
    service: "服务",
    logs: "日志",
    stats: "访问统计",
    certs: "证书",
    upstreams: "上游",
    loading: "加载中…",
    emptyNoConn: "先在左侧选择或新建一个 SSH 连接。",
    overviewTitle: "远程主机概览",
    overviewSub: "nginx 安装与运行环境(nginx -V 解析)",
    notFound: "远程机器上未找到 nginx 可执行文件",
    configTitle: "配置结构",
    configSub: "主配置与 include 展开;点击文件名查看内容",
    servers: "server 块",
    upstreams: "upstream 块",
    files: "文件",
    locations: "locations",
    certificates: "证书",
    viewFile: "查看",
    validateTitle: "配置校验",
    validateSub: "在远程执行 nginx -t,只读不 reload",
    runValidate: "运行 nginx -t",
    validateOk: "校验通过",
    validateFail: "校验失败",
    serviceTitle: "服务状态",
    serviceSub: "systemd / pid 文件状态;reload 前强制先校验",
    reload: "重载 nginx",
    reloadConfirmTitle: "确认重载 nginx?",
    reloadConfirmBody: "将先执行 nginx -t,通过后才会重载。生产环境请确认当前流量窗口。",
    reloadOk: "重载完成",
    reloadBlocked: "nginx -t 未通过,已拒绝重载",
    logsTitle: "日志",
    logsSub: "tail 最近的日志;支持正则过滤与搜索",
    errorLog: "error.log",
    accessLog: "access.log",
    refresh: "刷新",
    lines: "行数",
    filter: "过滤(正则)",
    search: "搜索",
    searchPattern: "搜索(POSIX ERE)",
    statsTitle: "访问统计",
    statsSub: "对日志尾部窗口做聚合分析",
    statusCodes: "状态码",
    statusClasses: "状态分类",
    methods: "请求方法",
    topIPs: "来源 IP",
    topPaths: "路径",
    avgMs: "平均(ms)",
    certsTitle: "TLS 证书",
    certsSub: "解析配置中引用的 ssl_certificate 文件",
    expiringSoon: "即将过期",
    expired: "已过期",
    days: "天",
    unreadable: "不可读",
    domains: "域名",
    issuer: "签发者",
    notAfter: "到期时间",
    upstreamsTitle: "上游健康",
    upstreamsSub: "从远程主机对 upstream server 发起 TCP 探测",
    check: "探测",
    address: "地址",
    latency: "延迟",
    alive: "存活",
    down: "不可达",
    connTitle: "连接管理",
    connSub: "SSH 连接配置保存在本机插件数据目录,密码不回传到界面",
    name: "名称",
    host: "主机",
    port: "端口",
    username: "用户名",
    authType: "认证",
    password: "密码",
    keyPath: "私钥路径",
    keyPassphrase: "私钥口令",
    save: "保存",
    cancel: "取消",
    edit: "编辑",
    delete: "删除",
    test: "测试",
    confirmDelete: "删除这条连接?",
    saved: "已保存",
    deleted: "已删除",
    testOk: "连接成功",
    connHintPassword: "首次连接会记录服务器指纹(TOFU);指纹变化时连接会被拒绝。",
    actions: "操作",
    window: "时间窗",
    scanned: "解析行",
    fingerprint: "主机指纹",
    systemd: "systemd",
    docker: "容器",
    state: "状态",
    pid: "PID",
    uptime: "运行时长",
    binary: "可执行文件",
    prefix: "prefix",
    confPath: "主配置",
    errorLogPath: "error 日志",
    accessLogPath: "access 日志",
    version: "版本",
    hostname: "主机名",
    kernel: "内核",
    errTitle: "操作失败",
  },
  "en-US": {
    pluginSubtitle: "Read-only nginx diagnostics",
    conn: "Connection",
    noConn: "No connection",
    addConn: "New connection…",
    manage: "Connections",
    overview: "Overview",
    config: "Config",
    validate: "Validate",
    service: "Service",
    logs: "Logs",
    stats: "Access stats",
    certs: "Certificates",
    upstreams: "Upstreams",
    loading: "Loading…",
    emptyNoConn: "Pick or create an SSH connection on the left first.",
    overviewTitle: "Remote host overview",
    overviewSub: "nginx installation and environment (parsed from nginx -V)",
    notFound: "nginx binary was not found on the remote machine",
    configTitle: "Configuration structure",
    configSub: "Main config with includes expanded; click a file to view it",
    servers: "server blocks",
    upstreams: "upstream blocks",
    files: "Files",
    locations: "locations",
    certificates: "Certificates",
    viewFile: "View",
    validateTitle: "Configuration validation",
    validateSub: "Runs nginx -t remotely; strictly read-only",
    runValidate: "Run nginx -t",
    validateOk: "Validation passed",
    validateFail: "Validation failed",
    serviceTitle: "Service status",
    serviceSub: "systemd / pid file status; reload always validates first",
    reload: "Reload nginx",
    reloadConfirmTitle: "Reload nginx?",
    reloadConfirmBody: "nginx -t runs first; the reload is refused when it fails. Confirm the traffic window for production.",
    reloadOk: "Reload finished",
    reloadBlocked: "nginx -t failed; reload refused",
    logsTitle: "Logs",
    logsSub: "Tail recent entries; filter by regex and search",
    errorLog: "error.log",
    accessLog: "access.log",
    refresh: "Refresh",
    lines: "Lines",
    filter: "Filter (regex)",
    search: "Search",
    searchPattern: "Search (POSIX ERE)",
    statsTitle: "Access statistics",
    statsSub: "Aggregate analysis over the tail window of the access log",
    statusCodes: "Status codes",
    statusClasses: "Status classes",
    methods: "Methods",
    topIPs: "Client IPs",
    topPaths: "Paths",
    avgMs: "avg (ms)",
    certsTitle: "TLS certificates",
    certsSub: "Parses ssl_certificate files referenced by the config",
    expiringSoon: "Expiring soon",
    expired: "Expired",
    days: "days",
    unreadable: "Unreadable",
    domains: "Domains",
    issuer: "Issuer",
    notAfter: "Not after",
    upstreamsTitle: "Upstream health",
    upstreamsSub: "TCP probes from the remote host to each upstream server",
    check: "Probe",
    address: "Address",
    latency: "Latency",
    alive: "Alive",
    down: "Down",
    connTitle: "Connections",
    connSub: "Profiles are stored in the local plugin data directory; passwords never come back to the UI",
    name: "Name",
    host: "Host",
    port: "Port",
    username: "Username",
    authType: "Auth",
    password: "Password",
    keyPath: "Private key path",
    keyPassphrase: "Key passphrase",
    save: "Save",
    cancel: "Cancel",
    edit: "Edit",
    delete: "Delete",
    test: "Test",
    confirmDelete: "Delete this connection?",
    saved: "Saved",
    deleted: "Deleted",
    testOk: "Connection OK",
    connHintPassword: "The first connection records the server fingerprint (TOFU); a changed fingerprint is rejected.",
    actions: "Actions",
    window: "Window",
    scanned: "Parsed lines",
    fingerprint: "Host fingerprint",
    systemd: "systemd",
    docker: "Container",
    state: "State",
    pid: "PID",
    uptime: "Uptime",
    binary: "Binary",
    prefix: "prefix",
    confPath: "Main config",
    errorLogPath: "error log",
    accessLogPath: "access log",
    version: "Version",
    hostname: "Hostname",
    kernel: "Kernel",
    errTitle: "Operation failed",
  },
};

const LOCALE = (navigator.language || "en-US").toLowerCase().startsWith("zh") ? "zh-CN" : "en-US";
const t = (key) => (STRINGS[LOCALE] && STRINGS[LOCALE][key]) || STRINGS["en-US"][key] || key;

/* ---------------- bridge ---------------- */

async function call(command, args = {}) {
  try {
    return await window.gonaviPlugin.invoke(command, args);
  } catch (err) {
    throw normalizeError(err);
  }
}

function normalizeError(err) {
  // The bridge surfaces JSON-RPC errors; keep a stable shape for mapping.
  const raw = err && err.message ? err.message : String(err);
  let code = "";
  let message = raw;
  let details = null;
  try {
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === "object") {
      message = parsed.message || raw;
      details = parsed.data || null;
      if (details && typeof details === "string") {
        try { details = JSON.parse(details); } catch { /* keep string */ }
      }
      code = (details && details.code) || "";
    }
  } catch { /* plain message */ }
  if (!code && err && err.data) {
    const d = typeof err.data === "string" ? safeParse(err.data) : err.data;
    if (d && d.code) { code = d.code; details = d.details || d; }
  }
  return { code, message, details };
}

function safeParse(text) { try { return JSON.parse(text); } catch { return null; } }

const ERROR_TEXT = {
  "zh-CN": {
    INVALID_ARGS: "参数不合法",
    PROFILE_NOT_FOUND: "连接不存在,请刷新连接列表",
    STORE_UNAVAILABLE: "本机插件数据目录不可用",
    SSH_CONNECT_FAILED: "SSH 连接失败,请检查主机、端口与网络",
    SSH_AUTH_FAILED: "SSH 认证失败,请检查用户名、密码或私钥",
    SSH_TIMEOUT: "SSH 操作超时",
    HOSTKEY_CHANGED: "服务器指纹与记录不一致!如确认服务器未被劫持,请在连接管理中删除并重建该连接",
    NGINX_NOT_FOUND: "远程机器上未找到 nginx",
    EXEC_FAILED: "远程命令执行失败",
    FILE_UNREADABLE: "远程文件不可读(权限不足或不存在)",
    CONFIG_PARSE_FAILED: "配置解析失败",
    NGINX_TEST_FAILED: "nginx -t 校验失败",
    RELOAD_BLOCKED: "nginx -t 未通过,已拒绝重载",
    RELOAD_CONFIRMATION_REQUIRED: "缺少确认标记",
  },
  "en-US": {
    INVALID_ARGS: "Invalid arguments",
    PROFILE_NOT_FOUND: "Connection not found; refresh the list",
    STORE_UNAVAILABLE: "Local plugin data directory is unavailable",
    SSH_CONNECT_FAILED: "SSH connection failed; check host, port and network",
    SSH_AUTH_FAILED: "SSH authentication failed; check credentials",
    SSH_TIMEOUT: "SSH operation timed out",
    HOSTKEY_CHANGED: "Server fingerprint changed! If you verified the server, delete and recreate the connection",
    NGINX_NOT_FOUND: "nginx was not found on the remote machine",
    EXEC_FAILED: "Remote command failed",
    FILE_UNREADABLE: "Remote file is not readable (missing or permission denied)",
    CONFIG_PARSE_FAILED: "Configuration parse failed",
    NGINX_TEST_FAILED: "nginx -t failed",
    RELOAD_BLOCKED: "nginx -t failed; reload refused",
    RELOAD_CONFIRMATION_REQUIRED: "Confirmation flag missing",
  },
};

function errorText(err) {
  const localized = err.code && ERROR_TEXT[LOCALE][err.code];
  if (localized) return localized;
  return err.message || "Unknown error";
}

/* ---------------- tiny dom helpers ---------------- */

const $ = (sel) => document.querySelector(sel);
function el(tag, attrs = {}, children = []) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(attrs)) {
    if (key === "class") node.className = value;
    else if (key === "text") node.textContent = value;
    else if (key.startsWith("on")) node.addEventListener(key.slice(2), value);
    else node.setAttribute(key, value);
  }
  for (const child of [].concat(children)) {
    if (child == null) continue;
    node.append(child);
  }
  return node;
}
function fmtBytes(n) {
  if (n == null || n < 0) return "-";
  const units = ["B", "KB", "MB", "GB"];
  let v = n, i = 0;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`;
}
function fmtTime(iso) {
  if (!iso) return "-";
  const d = new Date(iso);
  return isNaN(d) ? iso : d.toLocaleString();
}
function toast(text, bad = false) {
  const node = $("#toast");
  node.textContent = text;
  node.classList.toggle("bad", bad);
  node.classList.add("show");
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => node.classList.remove("show"), 3200);
}
function modal(content) {
  const backdrop = $("#modal-backdrop");
  const box = $("#modal");
  box.replaceChildren(...[].concat(content));
  backdrop.classList.add("show");
}
function closeModal() { $("#modal-backdrop").classList.remove("show"); }
$("#modal-backdrop").addEventListener("click", (e) => { if (e.target.id === "modal-backdrop") closeModal(); });

/* ---------------- state & shell ---------------- */

const state = {
  connections: [],
  connectionId: localStorage.getItem(STORE_KEY) || "",
  section: "overview",
  infoCache: new Map(),
};

const SECTIONS = [
  ["overview", "overview"],
  ["config", "config"],
  ["validate", "validate"],
  ["service", "service"],
  ["logs", "logs"],
  ["stats", "stats"],
  ["certs", "certs"],
  ["upstreams", "upstreams"],
  ["connections", "connections"],
];

function renderShell() {
  $("#brand-sub").textContent = t("pluginSubtitle");
  const nav = $("#nav");
  nav.replaceChildren(...SECTIONS.map(([id, key]) =>
    el("button", {
      class: "nav-item" + (state.section === id ? " active" : ""),
      onclick: () => { state.section = id; renderShell(); renderSection(); },
      text: t(key),
    })
  ));
  $("#foot").textContent = `${PLUGIN_ID}`;
  renderConnBlock();
}

function renderConnBlock() {
  const select = $("#conn-select");
  const options = state.connections.map((c) => el("option", { value: c.id }, `${c.name} (${c.host})`));
  select.replaceChildren(
    el("option", { value: "" }, `— ${t("addConn")} —`),
    ...options
  );
  select.value = state.connectionId;
  select.onchange = () => {
    state.connectionId = select.value;
    localStorage.setItem(STORE_KEY, state.connectionId);
    state.infoCache.clear();
    refreshConnMeta();
    renderSection();
  };
  refreshConnMeta();
}

function currentConn() {
  return state.connections.find((c) => c.id === state.connectionId) || null;
}

function refreshConnMeta() {
  const conn = currentConn();
  const dot = $("#conn-dot");
  const meta = $("#conn-meta");
  if (!conn) {
    $("#conn-label").textContent = t("noConn");
    dot.className = "conn-dot";
    meta.textContent = "";
    return;
  }
  $("#conn-label").textContent = conn.name;
  dot.className = "conn-dot busy";
  meta.textContent = `${conn.username}@${conn.host}:${conn.port}`;
  call("connections.test", { id: conn.id })
    .then((res) => {
      dot.className = "conn-dot ok";
      meta.textContent = `${conn.username}@${conn.host}:${conn.port}` +
        (res.fingerprint ? `\n${t("fingerprint")}: ${res.fingerprint}` : "");
    })
    .catch((err) => {
      dot.className = "conn-dot bad";
      meta.textContent = errorText(err);
    });
}

function requireConn() {
  if (!state.connectionId) {
    $("#main").replaceChildren(
      el("h1", { text: t(state.section + "Title") }),
      el("p", { class: "sub", text: t("emptyNoConn") })
    );
    return false;
  }
  return true;
}

function sectionHeader(titleKey, subKey) {
  return [el("h1", { text: t(titleKey) }), el("p", { class: "sub", text: t(subKey) })];
}

function factsList(pairs) {
  const dl = el("div", { class: "facts" });
  for (const [label, value, mono] of pairs) {
    dl.append(
      el("div", {}, [
        el("dt", { text: label }),
        el("dd", { class: mono ? "mono" : "", text: value == null || value === "" ? "-" : String(value) }),
      ])
    );
  }
  return dl;
}

function table(headers, rows) {
  const thead = el("thead", {}, el("tr", {}, headers.map((h, i) =>
    el("th", { class: i > 0 && /ms|count|#|days/i.test(h) ? "num" : "", text: h }))));
  const tbody = el("tbody");
  for (const row of rows) {
    tbody.append(el("tr", {}, row.map((cell) => {
      if (cell instanceof Node) return el("td", {}, cell);
      return el("td", { class: typeof cell === "number" ? "num" : "", text: String(cell) });
    })));
  }
  return el("table", {}, [thead, tbody]);
}

function pill(text, cls) { return el("span", { class: `pill ${cls}`, text }); }

/* ---------------- sections ---------------- */

async function renderSection() {
  renderShell();
  const main = $("#main");
  if (!requireConn()) return;
  main.replaceChildren(el("div", { class: "skel", text: t("loading") }));
  const render = RENDERERS[state.section];
  try {
    await render(main);
  } catch (err) {
    main.replaceChildren(
      ...sectionHeader(state.section + "Title", state.section + "Sub"),
      el("div", { class: "alert bad", text: errorText(err) })
    );
  }
}

const RENDERERS = {
  overview: renderOverview,
  config: renderConfig,
  validate: renderValidate,
  service: renderService,
  logs: renderLogs,
  stats: renderStats,
  certs: renderCerts,
  upstreams: renderUpstreams,
  connections: renderConnections,
};

/* ----- overview ----- */

async function renderOverview(main) {
  const info = await getInfo();
  const parts = sectionHeader("overviewTitle", "overviewSub");
  if (!info.found) {
    parts.push(el("div", { class: "alert bad", text: t("notFound") }));
  }
  parts.push(factsList([
    [t("hostname"), info.hostname],
    [t("kernel"), info.kernel],
    [t("version"), info.version, true],
    [t("binary"), info.binary, true],
    [t("prefix"), info.prefix, true],
    [t("confPath"), info.confPath, true],
    [t("errorLogPath"), info.errorLogPath, true],
    [t("accessLogPath"), info.httpLogPath, true],
    ["systemd", info.systemd ? "yes" : "no"],
    [t("docker"), info.docker ? "yes" : "no"],
  ]));
  main.replaceChildren(...parts);
}

async function getInfo() {
  if (!state.infoCache.has(state.connectionId)) {
    const info = await call("host.info", { connectionId: state.connectionId });
    state.infoCache.set(state.connectionId, info);
  }
  return state.infoCache.get(state.connectionId);
}

/* ----- config ----- */

function nodeLine(node, onView) {
  const line = el("div", { class: "node-line" });
  line.append(el("b", { text: node.name }));
  if (node.args && node.args.length) {
    line.append(el("span", { class: "args", text: " " + node.args.join(" ") }));
  }
  line.append(el("span", { class: "ln", text: `  :${node.line}` }));
  if (node.name === "#unreadable-include") line.classList.add("dir-err");
  return line;
}

function renderNodes(nodes, onView) {
  const box = el("div", { class: "node" });
  for (const node of nodes) {
    const entry = el("div");
    if (node.children && node.children.length) {
      entry.append(nodeLine(node, onView));
      entry.append(renderNodes(node.children, onView));
    } else {
      entry.append(nodeLine(node, onView));
    }
    box.append(entry);
  }
  return box;
}

async function renderConfig(main) {
  const tree = await call("config.tree", { connectionId: state.connectionId });
  const parts = sectionHeader("configTitle", "configSub");
  parts.push(factsList([
    [t("confPath"), tree.mainPath, true],
    [t("prefix"), tree.prefix, true],
    [t("version"), tree.version, true],
    [t("files"), String(tree.files.length + 1)],
  ]));
  if (tree.truncated) {
    parts.push(el("div", { class: "alert", text: "…" }));
  }

  const serverRows = tree.servers.map((s) => [
    `${s.listen ? s.listen.join(", ") : "?"}`,
    (s.names || []).join(", ") || "-",
    s.ssl ? pill("ssl", "ok") : pill("http", "dim"),
    `${s.file || tree.mainPath}:${s.line}`,
    String((s.locations || []).length),
  ]);
  parts.push(el("h2", { text: `${t("servers")} (${tree.servers.length})` }));
  parts.push(serverRows.length
    ? table(["listen", "server_name", "", "file:line", t("locations")], serverRows)
    : el("div", { class: "empty", text: "-" }));

  const upstreamRows = tree.upstreams.map((u) => [
    u.name,
    (u.servers || []).map((m) => m.address).join("\n") || "-",
    `${u.file || tree.mainPath}:${u.line}`,
  ]);
  parts.push(el("h2", { text: `${t("upstreams")} (${tree.upstreams.length})` }));
  parts.push(upstreamRows.length
    ? table(["upstream", "server", "file:line"], upstreamRows)
    : el("div", { class: "empty", text: "-" }));

  parts.push(el("h2", { text: `${t("files")} (${tree.files.length + 1})` }));
  const mainFile = el("details", { class: "file", open: "" });
  mainFile.append(el("summary", { text: tree.mainPath }));
  mainFile.append(renderNodes(tree.files.length ? tree.files[0].directives : [], () => {}));
  parts.push(mainFile);
  for (let i = 1; i < tree.files.length; i++) {
    const f = tree.files[i];
    const details = el("details", { class: "file" });
    const summary = el("summary", { text: `${f.path} (${fmtBytes(f.sizeBytes)})` });
    summary.addEventListener("click", () => {
      // View content on expand.
      if (!details.dataset.loaded) {
        details.dataset.loaded = "1";
        call("config.get", { connectionId: state.connectionId, path: f.path })
          .then((res) => {
            const pre = el("pre", { class: "log" });
            pre.textContent = res.content;
            details.append(pre);
          })
          .catch((err) => details.append(el("div", { class: "alert bad", text: errorText(err) })));
      }
    });
    details.append(summary);
    parts.push(details);
  }
  main.replaceChildren(...parts);
}

/* ----- validate ----- */

async function renderValidate(main) {
  const parts = sectionHeader("validateTitle", "validateSub");
  const button = el("button", { class: "primary", text: t("runValidate") });
  const out = el("div");
  button.onclick = async () => {
    button.disabled = true;
    out.replaceChildren(el("div", { class: "skel", text: t("loading") }));
    try {
      const result = await call("config.validate", { connectionId: state.connectionId });
      out.replaceChildren(renderValidation(result));
    } catch (err) {
      out.replaceChildren(el("div", { class: "alert bad", text: errorText(err) }));
    }
    button.disabled = false;
  };
  parts.push(el("div", { class: "row" }, [button]));
  parts.push(out);
  main.replaceChildren(...parts);
}

function renderValidation(result) {
  const parts = [result.ok
    ? el("div", { class: "alert ok", text: t("validateOk") })
    : el("div", { class: "alert bad", text: t("validateFail") })];
  for (const msg of result.messages || []) {
    const text = `${msg.level}: ${msg.message}` + (msg.file ? ` (${msg.file}:${msg.line})` : "");
    parts.push(el("div", { class: msg.level === "emerg" || msg.level === "alert" || msg.level === "crit" ? "alert bad" : "alert", text }));
  }
  return el("div", {}, parts);
}

/* ----- service ----- */

async function renderService(main) {
  const parts = sectionHeader("serviceTitle", "serviceSub");
  const status = await call("service.status", { connectionId: state.connectionId });
  const statePill = status.state === "active" ? pill(status.state, "ok")
    : status.state === "inactive" ? pill(status.state, "dim")
    : pill(status.state || "unknown", "warn");
  parts.push(factsList([
    [t("version"), status.version, true],
    [t("state"), statePill],
    [t("pid"), status.pid ? String(status.pid) : "-"],
    [t("uptime"), status.uptime || "-"],
    ["systemd", status.systemd ? "yes" : "no"],
    [t("docker"), status.docker ? "yes" : "no"],
  ]));
  const button = el("button", { class: "primary", text: t("reload") });
  button.onclick = () => confirmReload(button);
  parts.push(el("div", { class: "row" }, [button, el("span", { class: "skel", text: t("serviceSub") })]));
  main.replaceChildren(...parts);
}

function confirmReload(button) {
  const out = el("div", { class: "fingerprint", text: t("reloadConfirmBody") });
  const okButton = el("button", {
    class: "primary",
    text: t("reload"),
    onclick: async () => {
      okButton.disabled = true;
      try {
        const result = await call("service.reload", { connectionId: state.connectionId, confirm: true });
        closeModal();
        toast(t("reloadOk") + (result.method ? ` (${result.method})` : ""));
        renderSection();
      } catch (err) {
        closeModal();
        if (err.code === "RELOAD_BLOCKED") toast(t("reloadBlocked"), true);
        else toast(errorText(err), true);
      }
      okButton.disabled = false;
    },
  });
  modal([
    el("h3", { text: t("reloadConfirmTitle") }),
    out,
    el("div", { class: "actions" }, [
      el("button", { text: t("cancel"), onclick: closeModal }),
      okButton,
    ]),
  ]);
}

/* ----- logs ----- */

async function renderLogs(main) {
  const parts = sectionHeader("logsTitle", "logsSub");
  const kindSelect = el("select", {},
    [el("option", { value: "error" }, t("errorLog")), el("option", { value: "access" }, t("accessLog"))]);
  const linesInput = el("input", { type: "number", value: "200", min: "1", max: "1000" });
  const filterInput = el("input", { type: "text", class: "wide", placeholder: t("filter") });
  const searchInput = el("input", { type: "text", class: "wide", placeholder: t("searchPattern") });
  const refreshButton = el("button", { class: "primary", text: t("refresh") });
  const searchButton = el("button", { text: t("search") });
  const out = el("pre", { class: "log", text: "" });
  const searchOut = el("div");

  async function doTail() {
    out.textContent = t("loading");
    try {
      const res = await call("logs.tail", {
        connectionId: state.connectionId,
        kind: kindSelect.value,
        lines: Number(linesInput.value) || 200,
        filter: filterInput.value,
      });
      const meta = `${res.path} · ${res.matched}/${res.scanned} · ${fmtBytes(res.sizeBytes)}`;
      out.textContent = meta + "\n\n" + ((res.lines || []).join("\n") || "-");
    } catch (err) {
      out.textContent = errorText(err);
    }
  }
  async function doSearch() {
    if (!searchInput.value.trim()) return;
    searchOut.replaceChildren(el("div", { class: "skel", text: t("loading") }));
    try {
      const res = await call("logs.search", {
        connectionId: state.connectionId,
        kind: kindSelect.value,
        pattern: searchInput.value,
        maxMatches: 100,
      });
      const rows = (res.matches || []).map((m) => [m.line, m.text]);
      searchOut.replaceChildren(
        el("h2", { text: `${t("search")} (${res.matches ? res.matches.length : 0}${res.more ? "+" : ""})` }),
        rows.length ? table(["#", "line"], rows) : el("div", { class: "empty", text: "-" })
      );
    } catch (err) {
      searchOut.replaceChildren(el("div", { class: "alert bad", text: errorText(err) }));
    }
  }
  refreshButton.onclick = doTail;
  searchButton.onclick = doSearch;
  searchInput.addEventListener("keydown", (e) => { if (e.key === "Enter") doSearch(); });
  filterInput.addEventListener("keydown", (e) => { if (e.key === "Enter") doTail(); });

  parts.push(
    el("div", { class: "row" }, [kindSelect, el("label", { text: t("lines") }), linesInput, filterInput, refreshButton]),
    out,
    el("div", { class: "row" }, [searchInput, searchButton]),
    searchOut
  );
  main.replaceChildren(...parts);
  doTail();
}

/* ----- stats ----- */

function countTable(entries, label) {
  const rows = (entries || []).map((e) => [e.value, e.count,
    e.avgMs != null ? Number(e.avgMs.toFixed(1)) : ""]);
  return table([label, "count", t("avgMs")], rows);
}

async function renderStats(main) {
  const parts = sectionHeader("statsTitle", "statsSub");
  const bytesSelect = el("select", {},
    [["20", "20 MB"], ["50", "50 MB"], ["100", "100 MB"]].map(([v, label]) => el("option", { value: String(Number(v) * 1048576) }, label)));
  const button = el("button", { class: "primary", text: t("refresh") });
  const out = el("div");
  async function run() {
    out.replaceChildren(el("div", { class: "skel", text: t("loading") }));
    try {
      const s = await call("access.stats", { connectionId: state.connectionId, bytes: Number(bytesSelect.value) });
      const grid = el("div");
      grid.append(
        el("h2", { text: t("window") }),
        factsList([
          ["path", s.path, true],
          [t("window"), fmtBytes(s.windowBytes)],
          [t("scanned"), `${s.parsed}/${s.total}`],
          ["5xx", String(countOf(s.classes, "5xx"))],
          [t("window"), `${fmtTime(s.timeFrom)} → ${fmtTime(s.timeTo)}`],
          ["upstream", s.avgUpstreamMs != null ? `avg ${s.avgUpstreamMs.toFixed(0)}ms / max ${s.maxUpstreamMs.toFixed(0)}ms` : "-"],
        ]),
        el("h2", { text: t("statusCodes") }), countTable(s.statuses, "status"),
        el("h2", { text: t("statusClasses") }), countTable(s.classes, "class"),
        el("h2", { text: t("methods") }), countTable(s.methods, "method"),
        el("h2", { text: t("topIPs") }), countTable(s.topIPs, "ip"),
        el("h2", { text: t("topPaths") }), countTable(s.topPaths, "path")
      );
      out.replaceChildren(grid);
    } catch (err) {
      out.replaceChildren(el("div", { class: "alert bad", text: errorText(err) }));
    }
  }
  button.onclick = run;
  parts.push(el("div", { class: "row" }, [bytesSelect, button]), out);
  main.replaceChildren(...parts);
  run();
}

function countOf(entries, value) {
  for (const e of entries || []) if (e.value === value) return e.count;
  return 0;
}

/* ----- certs ----- */

async function renderCerts(main) {
  const parts = sectionHeader("certsTitle", "certsSub");
  const report = await call("certs.list", { connectionId: state.connectionId });
  const rows = report.certificates.map((c) => {
    let status;
    if (!c.readable) status = pill(t("unreadable"), "warn");
    else if (c.expired) status = pill(t("expired"), "bad");
    else if (c.expiringSoon) status = pill(`${t("expiringSoon")} ${c.daysLeft}${t("days")}`, "warn");
    else status = pill(`${c.daysLeft}${t("days")}`, "ok");
    return [
      c.path,
      (c.domains || []).join(", ") || "-",
      c.issuer || "-",
      fmtTime(c.notAfter),
      status,
    ];
  });
  if (report.expiringSoon || report.expired) {
    parts.push(el("div", { class: "alert", text: `${t("expiringSoon")}: ${report.expiringSoon} · ${t("expired")}: ${report.expired}` }));
  }
  parts.push(rows.length
    ? table(["file", t("domains"), t("issuer"), t("notAfter"), ""], rows)
    : el("div", { class: "empty", text: "-" }));
  main.replaceChildren(...parts);
}

/* ----- upstreams ----- */

async function renderUpstreams(main) {
  const parts = sectionHeader("upstreamsTitle", "upstreamsSub");
  const button = el("button", { class: "primary", text: t("check") });
  const out = el("div");
  button.onclick = async () => {
    button.disabled = true;
    out.replaceChildren(el("div", { class: "skel", text: t("loading") }));
    try {
      const check = await call("upstreams.check", { connectionId: state.connectionId });
      const rows = [];
      for (const u of check.upstreams || []) {
        for (const m of u.servers || []) {
          const status = m.alive == null ? pill("-", "dim")
            : m.alive ? pill(`${t("alive")}${m.latencyMs != null ? ` ${m.latencyMs}ms` : ""}`, "ok")
            : pill(t("down"), "bad");
          rows.push([u.name, m.address, (m.params || []).join(" "), status]);
        }
      }
      out.replaceChildren(table(["upstream", t("address"), "params", ""], rows),
        el("p", { class: "sub", text: `${t("alive")}: ${check.alive} · ${t("down")}: ${check.down}` }));
    } catch (err) {
      out.replaceChildren(el("div", { class: "alert bad", text: errorText(err) }));
    }
    button.disabled = false;
  };
  parts.push(el("div", { class: "row" }, [button]), out);
  main.replaceChildren(...parts);
}

/* ----- connections ----- */

async function renderConnections(main) {
  const parts = sectionHeader("connTitle", "connSub");
  parts.push(el("div", { class: "alert", text: t("connHintPassword") }));
  const list = el("div");
  for (const c of state.connections) {
    list.append(el("div", { class: "row" }, [
      el("b", { text: c.name }),
      el("span", { class: "mono", text: `${c.username}@${c.host}:${c.port}` }),
      pill(c.authType === "key" ? "key" : "password", "dim"),
      el("button", { text: t("test"), onclick: async (e) => {
        e.target.disabled = true;
        try { await call("connections.test", { id: c.id }); toast(t("testOk")); }
        catch (err) { toast(errorText(err), true); }
        e.target.disabled = false;
      } }),
      el("button", { text: t("edit"), onclick: () => connForm(c) }),
      el("button", { class: "danger", text: t("delete"), onclick: async () => {
        if (!window.confirm(t("confirmDelete"))) return;
        try {
          await call("connections.delete", { id: c.id });
          if (state.connectionId === c.id) { state.connectionId = ""; localStorage.removeItem(STORE_KEY); }
          await loadConnections();
          renderSection();
          toast(t("deleted"));
        } catch (err) { toast(errorText(err), true); }
      } }),
    ]));
  }
  parts.push(list, el("div", { class: "row" }, [el("button", { class: "primary", text: t("addConn"), onclick: () => connForm(null) })]));
  main.replaceChildren(...parts);
}

function connForm(existing) {
  const authSelect = el("select", {}, [
    el("option", { value: "password" }, t("password")),
    el("option", { value: "key" }, t("keyPath")),
  ]);
  const passwordInput = el("input", { type: "password", placeholder: existing && existing.hasPassword ? "••••••(unchanged)" : "" });
  const keyPathInput = el("input", { type: "text", value: existing && existing.keyPath ? existing.keyPath : "" });
  const keyPassInput = el("input", { type: "password", placeholder: existing && existing.hasKeyPassphrase ? "••••••(unchanged)" : "" });
  const syncAuth = () => {
    passwordInput.closest("label").classList.toggle("hidden", authSelect.value !== "password");
    keyPathInput.closest("label").classList.toggle("hidden", authSelect.value !== "key");
    keyPassInput.closest("label").classList.toggle("hidden", authSelect.value !== "key");
  };
  authSelect.addEventListener("change", syncAuth);

  const nameInput = el("input", { type: "text", value: existing ? existing.name : "" });
  const hostInput = el("input", { type: "text", value: existing ? existing.host : "" });
  const portInput = el("input", { type: "number", value: existing ? existing.port : 22 });
  const userInput = el("input", { type: "text", value: existing ? existing.username : "" });

  const form = el("form", { class: "conn" }, [
    el("label", { text: t("name") }), nameInput,
    el("label", { text: t("host") }), hostInput,
    el("label", { text: t("port") }), portInput,
    el("label", { text: t("username") }), userInput,
    el("label", { text: t("authType") }), authSelect,
    el("label", {}, [passwordInput]), el("div"),
    el("label", {}, [keyPathInput]), el("div"),
    el("label", {}, [keyPassInput]), el("div"),
  ]);
  syncAuth();

  const saveButton = el("button", {
    class: "primary",
    text: t("save"),
    onclick: async (e) => {
      e.preventDefault();
      const payload = {
        id: existing ? existing.id : "",
        name: nameInput.value,
        host: hostInput.value,
        port: Number(portInput.value) || 22,
        username: userInput.value,
        authType: authSelect.value,
      };
      if (authSelect.value === "password") {
        if (passwordInput.value) payload.password = passwordInput.value;
      } else {
        payload.keyPath = keyPathInput.value;
        if (keyPassInput.value) payload.keyPassphrase = keyPassInput.value;
      }
      saveButton.disabled = true;
      try {
        const res = await call("connections.save", payload);
        state.connectionId = res.connection.id;
        localStorage.setItem(STORE_KEY, state.connectionId);
        await loadConnections();
        closeModal();
        toast(t("saved"));
        renderSection();
      } catch (err) {
        toast(errorText(err), true);
      }
      saveButton.disabled = false;
    },
  });
  modal([
    el("h3", { text: existing ? `${t("edit")} — ${existing.name}` : t("addConn") }),
    form,
    el("div", { class: "actions" }, [
      el("button", { text: t("cancel"), onclick: closeModal }),
      saveButton,
    ]),
  ]);
  form.addEventListener("submit", (e) => e.preventDefault());
}

/* ---------------- boot ---------------- */

async function loadConnections() {
  const res = await call("connections.list");
  state.connections = res.connections || [];
}

(async function boot() {
  try {
    await loadConnections();
  } catch (err) {
    // Storage problems should not blank the window.
    state.connections = [];
  }
  if (state.connectionId && !state.connections.some((c) => c.id === state.connectionId)) {
    state.connectionId = "";
  }
  state.section = state.connections.length ? "overview" : "connections";
  renderShell();
  renderSection();
})();
