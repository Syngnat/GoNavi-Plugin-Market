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
    skipToMain: "跳到主内容",
    navLabel: "主分区",
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
    includedFiles: "包含文件",
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
    reloadApplied: "已重载,error_log 未见 emerg/alert",
    reloadUnknown: "已发送重载信号,但无法确认是否生效;请检查 error_log",
    reloadBlocked: "nginx -t 未通过,已拒绝重载",
    logsTitle: "日志",
    logsSub: "tail 最近的日志;支持正则过滤与搜索",
    errorLog: "error.log",
    accessLog: "access.log",
    refresh: "刷新",
    lines: "行数",
    lineColumn: "行",
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
    parseError: "解析失败",
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
    skipped: "跳过",
    backup: "backup",
    disabled: "已禁用",
    connTitle: "连接管理",
    connectionsTitle: "连接管理",
    connectionsSub: "SSH 连接配置保存在本机插件数据目录,密码不回传到界面",
    connSub: "SSH 连接配置保存在本机插件数据目录,密码不回传到界面",
    name: "名称",
    host: "主机",
    port: "端口",
    username: "用户名",
    authType: "认证",
    password: "密码",
    keyPath: "私钥路径",
    keyPassphrase: "私钥口令",
    chooseFile: "选择…",
    keyFilePickTitle: "选择本机 SSH 私钥文件",
    keyFilePickUnavailable: "当前宿主不支持文件选择，请手动输入私钥路径。",
    keyFilePickFailed: "打开文件选择器失败，请手动输入私钥路径。",
    save: "保存",
    cancel: "取消",
    edit: "编辑",
    delete: "删除",
    test: "测试",
    testConnection: "测试当前连接",
    confirmDelete: "删除这条连接?",
    deleteTitle: "删除连接",
    deleteConfirmBody: "连接配置、保存的凭据与 TOFU 指纹都会被移除,且不可撤销。",
    saved: "已保存",
    deleted: "已删除",
    testOk: "连接成功",
    connHintPassword: "首次连接会记录服务器指纹(TOFU);指纹变化时连接会被拒绝。仅在测试或保存时才会建立 SSH 连接。",
    passwordUnchangedHint: "留空表示不修改已保存的密码。",
    passphraseUnchangedHint: "留空表示不修改已保存的私钥口令。",
    actions: "操作",
    window: "时间窗",
    timeRange: "时间范围",
    scanned: "解析行",
    unparsed: "未解析行",
    analyzedBytes: "实际分析",
    requestedBytes: "请求窗口",
    searchWindow: "搜索窗口",
    source: "来源",
    windowStart: "窗口起点",
    fileSize: "文件大小",
    fingerprint: "主机指纹",
    systemd: "systemd",
    docker: "容器",
    container: "容器环境",
    unit: "systemd 单元",
    totalLines: "总行数",
    level: "日志级别",
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
    discoveredAt: "发现时间",
    refreshInfo: "刷新主机信息",
    refreshed: "已刷新",
    infoTtlHint: "主机信息在插件进程内缓存;安装或升级 nginx、改过路径后点「刷新主机信息」,无需重开窗口。",
    yes: "是",
    no: "否",
    count: "次数",
    statusCode: "状态码",
    classLabel: "分类",
    methodLabel: "方法",
    ipLabel: "IP",
    pathLabel: "路径",
    paramsLabel: "参数",
    serverLabel: "server",
    sslColumn: "SSL",
    fileLine: "file:line",
    note: "说明",
    reason: "原因",
    truncated: "已截断",
    truncatedMark: "(已截断)",
    configTruncated: "配置树已截断:文件数、节点预算或 include 深度超过上限,下面的结果不完整。",
    fileTruncated: "文件超过读取上限,仅显示前缀内容。",
    includeUnresolved: "include 未匹配到任何文件,配置可能遗漏",
    includeUnreadable: "include 文件不可读",
    tailTruncated: "tail 输出超过远端命令输出上限,显示的是前缀而不是完整尾部。",
    searchTruncated: "搜索命中数达到上限,仅显示前若干条。",
    statsTruncated: "日志窗口被输出上限截断:统计只覆盖窗口内更新的那部分字节,不代表整个时间窗。",
    gzipUnsupported: "日志是 gzip 压缩格式,当前不支持解压解析,统计结果为空。",
    requestTime: "请求耗时",
    upstreamTime: "上游耗时",
    reloadUnknownShort: "重载结果未知",
    skippedNote: "地址无法解析、变量或 unix socket,未探测",
    portInvalid: "端口必须是 1–65535 的整数",
    requiredFields: "名称、主机、用户名不能为空",
    passwordRequired: "密码认证需要填写密码",
    keyPathRequired: "私钥认证需要填写私钥路径",
    formError: "请检查表单字段",
  },
  "en-US": {
    pluginSubtitle: "Read-only nginx diagnostics",
    skipToMain: "Skip to content",
    navLabel: "Sections",
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
    includedFiles: "Included files",
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
    reloadApplied: "Reloaded; error_log shows no emerg/alert",
    reloadUnknown: "Reload signal sent, but the outcome is unconfirmed; check error_log",
    reloadBlocked: "nginx -t failed; reload refused",
    logsTitle: "Logs",
    logsSub: "Tail recent entries; filter by regex and search",
    errorLog: "error.log",
    accessLog: "access.log",
    refresh: "Refresh",
    lines: "Lines",
    lineColumn: "line",
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
    parseError: "Parse failed",
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
    skipped: "Skipped",
    backup: "backup",
    disabled: "disabled",
    connTitle: "Connections",
    connectionsTitle: "Connections",
    connectionsSub: "Profiles are stored in the local plugin data directory; passwords never come back to the UI",
    connSub: "Profiles are stored in the local plugin data directory; passwords never come back to the UI",
    name: "Name",
    host: "Host",
    port: "Port",
    username: "Username",
    authType: "Auth",
    password: "Password",
    keyPath: "Private key path",
    keyPassphrase: "Key passphrase",
    chooseFile: "Choose…",
    keyFilePickTitle: "Select a local SSH private key",
    keyFilePickUnavailable: "This host cannot open a file picker; enter the key path manually.",
    keyFilePickFailed: "Could not open the file picker; enter the key path manually.",
    save: "Save",
    cancel: "Cancel",
    edit: "Edit",
    delete: "Delete",
    test: "Test",
    testConnection: "Test current connection",
    confirmDelete: "Delete this connection?",
    deleteTitle: "Delete connection",
    deleteConfirmBody: "The profile, saved credentials and TOFU fingerprint are removed for good. This cannot be undone.",
    saved: "Saved",
    deleted: "Deleted",
    testOk: "Connection OK",
    connHintPassword: "The first connection records the server fingerprint (TOFU); a changed fingerprint is rejected. SSH is only dialed from Test or Save.",
    passwordUnchangedHint: "Leave empty to keep the saved password.",
    passphraseUnchangedHint: "Leave empty to keep the saved key passphrase.",
    actions: "Actions",
    window: "Window",
    timeRange: "Time range",
    scanned: "Parsed lines",
    unparsed: "Unparsed lines",
    analyzedBytes: "Analyzed",
    requestedBytes: "Requested",
    searchWindow: "Search window",
    source: "Source",
    windowStart: "Window start",
    fileSize: "File size",
    fingerprint: "Host fingerprint",
    systemd: "systemd",
    docker: "Container",
    container: "Container",
    unit: "systemd unit",
    totalLines: "Total lines",
    level: "Log level",
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
    discoveredAt: "Discovered at",
    refreshInfo: "Refresh host info",
    refreshed: "Refreshed",
    infoTtlHint: "Host info is cached in the plugin process; after installing or upgrading nginx, use Refresh host info instead of reopening the window.",
    yes: "yes",
    no: "no",
    count: "count",
    statusCode: "status",
    classLabel: "class",
    methodLabel: "method",
    ipLabel: "ip",
    pathLabel: "path",
    paramsLabel: "params",
    serverLabel: "server",
    sslColumn: "SSL",
    fileLine: "file:line",
    note: "note",
    reason: "reason",
    truncated: "truncated",
    truncatedMark: "(truncated)",
    configTruncated: "The config tree is truncated: the file, node-budget or include-depth limit was reached, so the results below are incomplete.",
    fileTruncated: "The file exceeded the read limit; only the prefix is shown.",
    includeUnresolved: "include matched no files; the configuration may be incomplete",
    includeUnreadable: "included file is unreadable",
    tailTruncated: "The tail output hit the remote command limit; this is the prefix, not the newest lines.",
    searchTruncated: "Match limit reached; only the first matches are shown.",
    statsTruncated: "The log window was cut by the output limit: statistics cover only the newest bytes of the window, not the whole time range.",
    gzipUnsupported: "The access log is gzip-compressed; parsing is not supported, so no statistics are available.",
    requestTime: "Request time",
    upstreamTime: "Upstream time",
    reloadUnknownShort: "Reload outcome unknown",
    skippedNote: "address is variable, a unix socket or unparsable, so it was not probed",
    portInvalid: "Port must be an integer between 1 and 65535",
    requiredFields: "Name, host and username are required",
    passwordRequired: "A password is required for password auth",
    keyPathRequired: "A key path is required for key auth",
    formError: "Check the form fields",
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
      // Accept both {code,message,data} and JSON-RPC {error:{...}} shapes.
      const payload = parsed.error && typeof parsed.error === "object" ? parsed.error : parsed;
      message = payload.message || raw;
      details = payload.data || null;
      if (details && typeof details === "string") {
        try { details = JSON.parse(details); } catch { /* keep string */ }
      }
      code = (details && details.code) || payload.code || "";
    }
  } catch { /* plain message */ }
  if (!code && err && err.data) {
    const d = typeof err.data === "string" ? safeParse(err.data) : err.data;
    if (d && d.code) { code = d.code; details = d.details || d; }
  }
  if (!code && err && err.code) code = String(err.code);
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
    EXEC_PERMISSION: "权限不足:该操作需要 root 或可读取 nginx 配置的用户",
    FILE_UNREADABLE: "远程文件不可读(权限不足或不存在)",
    PATH_NOT_ALLOWED: "路径不在允许范围内(仅限 nginx prefix、日志与证书目录)",
    CONFIG_PARSE_FAILED: "配置解析失败",
    NGINX_TEST_FAILED: "nginx -t 校验失败",
    RELOAD_BLOCKED: "nginx -t 未通过,已拒绝重载",
    RELOAD_CONFIRMATION_REQUIRED: "缺少确认标记",
    CERT_UNREADABLE: "证书文件不可读,无法解析",
    bridge_request_failed: "请求未完成：请检查 SSH 连接与私钥路径（路径不要带引号），再试一次",
    window_revoked: "插件窗口已失效，请关闭后重新打开",
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
    EXEC_PERMISSION: "Permission denied: this needs root or a user that can read the nginx configuration",
    FILE_UNREADABLE: "Remote file is not readable (missing or permission denied)",
    PATH_NOT_ALLOWED: "Path is outside the allowed nginx prefix, log and certificate directories",
    CONFIG_PARSE_FAILED: "Configuration parse failed",
    NGINX_TEST_FAILED: "nginx -t failed",
    RELOAD_BLOCKED: "nginx -t failed; reload refused",
    RELOAD_CONFIRMATION_REQUIRED: "Confirmation flag missing",
    CERT_UNREADABLE: "Certificate file is unreadable and could not be parsed",
    bridge_request_failed: "Request failed: check the SSH connection and key path (no surrounding quotes), then retry",
    window_revoked: "The plugin window is no longer valid; close and reopen it",
  },
};

function errorDetail(err) {
  const d = err && err.details;
  return d && d.details && typeof d.details === "object" ? d.details : d;
}

function errorText(err) {
  const detail = errorDetail(err);
  if (err.code === "EXEC_FAILED" && detail && detail.kind === "permission") {
    return ERROR_TEXT[LOCALE]["EXEC_PERMISSION"] || err.message;
  }
  if (err.code === "RELOAD_BLOCKED" && detail && detail.reason === "permission") {
    return ERROR_TEXT[LOCALE]["EXEC_PERMISSION"] || err.message;
  }
  const localized = err.code && ERROR_TEXT[LOCALE][err.code];
  if (localized) return localized;
  return err.message || "Unknown error";
}

/* ---------------- storage ---------------- */

// A broken or blocked localStorage must never take the whole window down.
const storage = {
  get(key) {
    try { return window.localStorage.getItem(key) || ""; } catch { return ""; }
  },
  set(key, value) {
    try { window.localStorage.setItem(key, value); } catch { /* volatile session */ }
  },
  remove(key) {
    try { window.localStorage.removeItem(key); } catch { /* ignore */ }
  },
};

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
  return v.toFixed(v >= 100 || i === 0 ? 0 : 1) + " " + units[i];
}
function fmtTime(iso) {
  if (!iso) return "-";
  const d = new Date(iso);
  return isNaN(d) ? iso : d.toLocaleString();
}
function numOrNull(value) {
  return typeof value === "number" && isFinite(value) ? value : null;
}
function firstNumber(source, keys) {
  if (!source || typeof source !== "object") return null;
  for (const key of keys) {
    const value = numOrNull(source[key]);
    if (value != null) return value;
  }
  return null;
}
function fmtMs(value) {
  const n = numOrNull(value);
  return n == null ? "-" : n.toFixed(0) + " ms";
}
function toast(text, bad = false) {
  const node = $("#toast");
  node.textContent = text;
  node.classList.toggle("bad", bad);
  node.classList.add("show");
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => node.classList.remove("show"), 3200);
}

/* ---------------- modal (Esc + focus trap) ---------------- */

const modalState = { previous: null };

function modal(content) {
  const backdrop = $("#modal-backdrop");
  const box = $("#modal");
  if (!backdrop.classList.contains("show")) {
    const active = document.activeElement;
    modalState.previous = active && active !== document.body ? active : null;
  }
  box.replaceChildren(...[].concat(content));
  const heading = box.querySelector("h3");
  if (heading) {
    if (!heading.id) heading.id = "modal-title";
    box.setAttribute("aria-labelledby", heading.id);
  } else {
    box.removeAttribute("aria-labelledby");
  }
  backdrop.classList.add("show");
  backdrop.setAttribute("aria-hidden", "false");
  const target = box.querySelector("[data-autofocus]") ||
    box.querySelector("input, select, button.primary, button");
  if (target) target.focus();
  else box.focus();
}

function closeModal() {
  const backdrop = $("#modal-backdrop");
  if (!backdrop.classList.contains("show")) return;
  backdrop.classList.remove("show");
  backdrop.setAttribute("aria-hidden", "true");
  $("#modal").replaceChildren();
  const previous = modalState.previous;
  modalState.previous = null;
  if (previous && document.contains(previous) && typeof previous.focus === "function") {
    previous.focus();
  }
}

$("#modal-backdrop").addEventListener("click", function (e) {
  if (e.target.id === "modal-backdrop") closeModal();
});

document.addEventListener("keydown", function (event) {
  const backdrop = $("#modal-backdrop");
  if (!backdrop.classList.contains("show")) return;
  if (event.key === "Escape") {
    event.preventDefault();
    closeModal();
    return;
  }
  if (event.key !== "Tab") return;
  const box = $("#modal");
  const focusable = Array.from(box.querySelectorAll(
    "button, input, select, textarea, a[href], [tabindex]:not([tabindex='-1'])"
  )).filter(function (node) {
    return !node.disabled && node.getClientRects().length > 0;
  });
  if (!focusable.length) {
    event.preventDefault();
    box.focus();
    return;
  }
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  const active = document.activeElement;
  if (!box.contains(active)) {
    event.preventDefault();
    first.focus();
    return;
  }
  if (event.shiftKey && active === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && active === last) {
    event.preventDefault();
    first.focus();
  }
});

/* ---------------- state & shell ---------------- */

const state = {
  connections: [],
  connectionId: storage.get(STORE_KEY),
  section: "overview",
  infoCache: new Map(),
  testCache: new Map(),
  generation: 0,
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

function current(generation) {
  return generation === state.generation;
}

function renderShell() {
  document.documentElement.lang = LOCALE;
  $("#brand-sub").textContent = t("pluginSubtitle");
  const skip = $("#skip");
  if (skip) skip.textContent = t("skipToMain");
  const nav = $("#nav");
  nav.setAttribute("aria-label", t("navLabel"));
  nav.replaceChildren(...SECTIONS.map(function (entry) {
    const id = entry[0];
    const key = entry[1];
    return el("button", {
      class: "nav-item",
      type: "button",
      "data-section": id,
      text: t(key),
      onclick: function () {
        if (state.section === id) return;
        state.section = id;
        renderSection();
      },
    });
  }));
  $("#foot").textContent = PLUGIN_ID;
  renderConnSelect();
  updateRail();
}

function updateRail() {
  const buttons = $("#nav").querySelectorAll(".nav-item");
  for (const button of buttons) {
    const active = button.dataset.section === state.section;
    button.classList.toggle("active", active);
    if (active) button.setAttribute("aria-current", "page");
    else button.removeAttribute("aria-current");
  }
  refreshConnMeta();
}

function renderConnSelect() {
  const select = $("#conn-select");
  select.setAttribute("aria-label", t("conn"));
  const options = state.connections.map(function (c) {
    return el("option", { value: c.id }, c.name);
  });
  select.replaceChildren(
    el("option", { value: "" }, "— " + t("addConn") + " —"),
    ...options,
    el("option", { value: "__new__" }, "+ " + t("addConn"))
  );
  select.value = state.connectionId;
  select.onchange = function () {
    const previous = state.connectionId;
    if (select.value === "" || select.value === "__new__") {
      select.value = previous;
      connForm(null);
      return;
    }
    if (select.value === previous) return;
    state.connectionId = select.value;
    storage.set(STORE_KEY, state.connectionId);
    state.infoCache.clear();
    renderConnSelect();
    renderSection();
  };
}

function currentConn() {
  return state.connections.find(function (c) { return c.id === state.connectionId; }) || null;
}

// No network here: the connection is only tested from the explicit buttons.
function refreshConnMeta() {
  const conn = currentConn();
  const dot = $("#conn-dot");
  const meta = $("#conn-meta");
  const testButton = $("#conn-test");
  if (testButton) {
    testButton.disabled = !conn;
    testButton.textContent = t("test");
    testButton.setAttribute("aria-label", t("testConnection"));
    testButton.onclick = function () { testCurrentConnection(); };
  }
  if (!conn) {
    dot.className = "conn-dot";
    meta.textContent = "";
    return;
  }
  meta.textContent = conn.username + "@" + conn.host + ":" + conn.port;
  if (conn.fingerprint) meta.textContent += "\n" + t("fingerprint") + ": " + conn.fingerprint;
  const cached = state.testCache.get(conn.id);
  dot.className = cached ? "conn-dot " + (cached.ok ? "ok" : "bad") : "conn-dot";
  if (!cached) return;
  if (cached.ok) {
    if (cached.fingerprint) meta.textContent += "\n" + t("fingerprint") + ": " + cached.fingerprint;
    if (cached.latencyMs != null) meta.textContent += " · " + cached.latencyMs + " ms";
  } else {
    meta.textContent += "\n" + cached.message;
  }
}

async function testCurrentConnection() {
  const conn = currentConn();
  if (!conn) return;
  const dot = $("#conn-dot");
  const button = $("#conn-test");
  if (button) button.disabled = true;
  dot.className = "conn-dot busy";
  try {
    const res = await call("connections.test", { id: conn.id });
    state.testCache.set(conn.id, { ok: true, fingerprint: res.fingerprint || "", latencyMs: res.latencyMs });
    toast(t("testOk") + (res.latencyMs != null ? " · " + res.latencyMs + " ms" : ""));
  } catch (err) {
    state.testCache.set(conn.id, { ok: false, message: errorText(err) });
    toast(errorText(err), true);
  }
  refreshConnMeta();
}

function requireConn() {
  if (state.connectionId || state.section === "connections") return true;
  $("#main").setAttribute("aria-busy", "false");
  $("#main").replaceChildren(
    el("h1", { text: t(state.section + "Title") }),
    el("p", { class: "sub", text: t("emptyNoConn") })
  );
  return false;
}

function sectionHeader(titleKey, subKey) {
  return [el("h1", { text: t(titleKey) }), el("p", { class: "sub", text: t(subKey) })];
}

function factsList(pairs) {
  const dl = el("div", { class: "facts" });
  for (const pair of pairs) {
    const label = pair[0];
    const value = pair[1];
    const mono = pair[2];
    const dd = el("dd", { class: mono ? "mono" : "" });
    if (value instanceof Node) dd.append(value);
    else dd.textContent = value == null || value === "" ? "-" : String(value);
    dl.append(el("div", {}, [el("dt", { text: label }), dd]));
  }
  return dl;
}

function table(headers, rows) {
  const thead = el("thead", {}, el("tr", {}, headers.map(function (h, i) {
    return el("th", { scope: "col", class: i > 0 && /ms|count|#|days/i.test(h) ? "num" : "", text: h });
  })));
  const tbody = el("tbody");
  for (const row of rows) {
    tbody.append(el("tr", {}, row.map(function (cell) {
      if (cell instanceof Node) return el("td", {}, cell);
      return el("td", { class: typeof cell === "number" ? "num" : "", text: String(cell) });
    })));
  }
  return el("table", {}, [thead, tbody]);
}

function pill(text, cls) { return el("span", { class: "pill " + cls, text: text }); }

/* ---------------- sections ---------------- */

async function renderSection() {
  const generation = ++state.generation;
  updateRail();
  const main = $("#main");
  if (!requireConn()) return;
  main.setAttribute("aria-busy", "true");
  main.replaceChildren(el("div", { class: "skel", text: t("loading") }));
  const render = RENDERERS[state.section] || renderOverview;
  try {
    await render(main, generation);
  } catch (err) {
    if (!current(generation)) return;
    main.replaceChildren(
      ...sectionHeader(state.section + "Title", state.section + "Sub"),
      el("div", { class: "alert bad", text: errorText(err) })
    );
  } finally {
    if (current(generation)) main.setAttribute("aria-busy", "false");
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

async function renderOverview(main, generation) {
  const info = await getInfo();
  if (!current(generation)) return;
  const parts = sectionHeader("overviewTitle", "overviewSub");
  if (!info.found) {
    parts.push(el("div", { class: "alert bad", text: t("notFound") }));
  }
  const facts = [
    [t("hostname"), info.hostname],
    [t("kernel"), info.kernel],
    [t("version"), info.version, true],
    [t("binary"), info.binary, true],
    [t("prefix"), info.prefix, true],
    [t("confPath"), info.confPath, true],
    [t("errorLogPath"), info.errorLogPath, true],
    [t("accessLogPath"), info.httpLogPath, true],
    [t("discoveredAt"), fmtTime(info.discoveredAt)],
    [t("systemd"), info.systemd ? t("yes") : t("no")],
    [t("docker"), info.docker ? t("yes") : t("no")],
  ];
  if (info.container) facts.push([t("container"), t("yes")]);
  parts.push(factsList(facts));
  const refreshButton = el("button", { text: t("refreshInfo") });
  refreshButton.onclick = async function () {
    refreshButton.disabled = true;
    try {
      await getInfo(true);
      if (!current(generation)) return;
      toast(t("refreshed"));
      renderSection();
    } catch (err) {
      if (current(generation)) toast(errorText(err), true);
    } finally {
      refreshButton.disabled = false;
    }
  };
  parts.push(el("div", { class: "row" }, [refreshButton]));
  parts.push(el("p", { class: "sub", text: t("infoTtlHint") }));
  main.replaceChildren(...parts);
}

async function getInfo(force = false) {
  const id = state.connectionId;
  if (!force && state.infoCache.has(id)) return state.infoCache.get(id);
  const args = { connectionId: id };
  if (force) args.refresh = true;
  const info = await call("host.info", args);
  state.infoCache.set(id, info);
  return info;
}

/* ----- config ----- */

function nodeLine(node) {
  const line = el("div", { class: "node-line" });
  line.append(el("b", { text: node.name }));
  if (node.args && node.args.length) {
    line.append(el("span", { class: "args", text: " " + node.args.join(" ") }));
  }
  line.append(el("span", { class: "ln", text: "  :" + node.line }));
  const name = String(node.name || "");
  if (name.startsWith("#")) {
    line.classList.add("dir-err");
    if (name === "#unresolved-include") line.append(el("span", { class: "ln", text: "  " + t("includeUnresolved") }));
    else if (name === "#unreadable-include") line.append(el("span", { class: "ln", text: "  " + t("includeUnreadable") }));
  }
  return line;
}

function renderNodes(nodes) {
  const box = el("div", { class: "node" });
  for (const node of nodes || []) {
    const entry = el("div");
    entry.append(nodeLine(node));
    if (node.children && node.children.length) entry.append(renderNodes(node.children));
    box.append(entry);
  }
  return box;
}

async function renderConfig(main, generation) {
  const tree = await call("config.tree", { connectionId: state.connectionId });
  if (!current(generation)) return;
  const files = Array.isArray(tree.files) ? tree.files : [];
  const parts = sectionHeader("configTitle", "configSub");
  parts.push(factsList([
    [t("confPath"), tree.mainPath, true],
    [t("prefix"), tree.prefix, true],
    [t("version"), tree.version, true],
    [t("files"), String(files.length)],
    [t("includedFiles"), String(tree.includedCount != null ? tree.includedCount : Math.max(0, files.length - 1))],
  ]));
  if (tree.truncated) {
    parts.push(el("div", { class: "alert bad", text: t("configTruncated") }));
  }

  const serverRows = (tree.servers || []).map(function (s) {
    return [
      s.listen ? s.listen.join(", ") : "?",
      (s.names || []).join(", ") || "-",
      s.ssl ? pill("ssl", "ok") : pill("http", "dim"),
      (s.file || tree.mainPath) + ":" + s.line,
      String((s.locations || []).length),
    ];
  });
  parts.push(el("h2", { text: t("servers") + " (" + (tree.servers || []).length + ")" }));
  parts.push(serverRows.length
    ? table(["listen", "server_name", t("sslColumn"), t("fileLine"), t("locations")], serverRows)
    : el("div", { class: "empty", text: "-" }));

  const upstreamRows = (tree.upstreams || []).map(function (u) {
    return [
      u.name,
      (u.servers || []).map(function (m) { return m.address; }).join("\n") || "-",
      (u.file || tree.mainPath) + ":" + u.line,
    ];
  });
  parts.push(el("h2", { text: t("upstreams") + " (" + (tree.upstreams || []).length + ")" }));
  parts.push(upstreamRows.length
    ? table([t("upstreams"), t("serverLabel"), t("fileLine")], upstreamRows)
    : el("div", { class: "empty", text: "-" }));

  parts.push(el("h2", { text: t("files") + " (" + files.length + ")" }));
  if (!files.length) {
    parts.push(el("div", { class: "empty", text: "-" }));
  } else {
    const mainFile = el("details", { class: "file", open: "" });
    mainFile.append(el("summary", {
      text: tree.mainPath + (files[0].truncated ? " " + t("truncatedMark") : ""),
    }));
    if (files[0].truncated) mainFile.append(el("div", { class: "alert", text: t("fileTruncated") }));
    mainFile.append(renderNodes(files[0].directives));
    parts.push(mainFile);
    for (let i = 1; i < files.length; i++) {
      const f = files[i];
      const details = el("details", { class: "file" });
      const summary = el("summary", {
        text: f.path + " (" + fmtBytes(f.sizeBytes) + ")" + (f.truncated ? " " + t("truncatedMark") : ""),
      });
      summary.addEventListener("click", function () {
        // View content on expand.
        if (details.dataset.loaded) return;
        details.dataset.loaded = "1";
        call("config.get", { connectionId: state.connectionId, path: f.path })
          .then(function (res) {
            if (!current(generation)) return;
            const body = el("div");
            if (res.truncated) body.append(el("div", { class: "alert", text: t("fileTruncated") }));
            const pre = el("pre", { class: "log" });
            pre.textContent = res.content;
            body.append(pre);
            const lineCount = res.lineCount != null ? res.lineCount : "?";
            body.append(el("div", {
              class: "sub",
              text: lineCount + " " + t("lines") + " · " + fmtBytes(res.sizeBytes),
            }));
            details.append(body);
          })
          .catch(function (err) {
            details.dataset.loaded = "";
            if (current(generation)) details.append(el("div", { class: "alert bad", text: errorText(err) }));
          });
      });
      details.append(summary);
      parts.push(details);
    }
  }
  main.replaceChildren(...parts);
}

/* ----- validate ----- */

async function renderValidate(main, generation) {
  const parts = sectionHeader("validateTitle", "validateSub");
  const button = el("button", { class: "primary", text: t("runValidate") });
  const out = el("div");
  button.onclick = async function () {
    const gen = state.generation;
    button.disabled = true;
    out.replaceChildren(el("div", { class: "skel", text: t("loading") }));
    try {
      const result = await call("config.validate", { connectionId: state.connectionId });
      if (!current(gen)) return;
      out.replaceChildren(renderValidation(result));
    } catch (err) {
      if (current(gen)) out.replaceChildren(el("div", { class: "alert bad", text: errorText(err) }));
    } finally {
      button.disabled = false;
    }
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
    const text = msg.level + ": " + msg.message + (msg.file ? " (" + msg.file + ":" + msg.line + ")" : "");
    const bad = msg.level === "emerg" || msg.level === "alert" || msg.level === "crit";
    parts.push(el("div", { class: bad ? "alert bad" : "alert", text: text }));
  }
  return el("div", {}, parts);
}

/* ----- service ----- */

async function renderService(main, generation) {
  const status = await call("service.status", { connectionId: state.connectionId });
  if (!current(generation)) return;
  const parts = sectionHeader("serviceTitle", "serviceSub");
  const statePill = status.state === "active" ? pill(status.state, "ok")
    : status.state === "inactive" ? pill(status.state, "dim")
    : pill(status.state || "unknown", "warn");
  const facts = [
    [t("version"), status.version, true],
    [t("state"), statePill],
    [t("pid"), status.pid ? String(status.pid) : "-"],
    [t("uptime"), status.uptime || "-"],
    [t("systemd"), status.systemd ? t("yes") : t("no")],
    [t("docker"), status.docker ? t("yes") : t("no")],
  ];
  if (status.unit) facts.push([t("unit"), status.unit, true]);
  parts.push(factsList(facts));
  const button = el("button", { class: "primary", text: t("reload") });
  button.onclick = function () { confirmReload(button); };
  parts.push(el("div", { class: "row" }, [button]));
  main.replaceChildren(...parts);
}

function confirmReload(button) {
  const connectionId = state.connectionId;
  const body = el("div", { class: "fingerprint", text: t("reloadConfirmBody") });
  const okButton = el("button", {
    class: "primary",
    text: t("reload"),
    onclick: async function () {
      okButton.disabled = true;
      try {
        const result = await call("service.reload", { connectionId: connectionId, confirm: true });
        closeModal();
        const applied = result.status === "applied";
        const suffix = result.method ? " (" + result.method + ")" : "";
        toast((applied || result.status == null ? t("reloadApplied") : t("reloadUnknown")) + suffix, !applied && result.status != null);
        if (state.connectionId === connectionId && state.section === "service") renderSection();
      } catch (err) {
        closeModal();
        toast(errorText(err), true);
      } finally {
        okButton.disabled = false;
        if (button) button.disabled = false;
      }
    },
  });
  modal([
    el("h3", { text: t("reloadConfirmTitle") }),
    body,
    el("div", { class: "actions" }, [
      el("button", { text: t("cancel"), onclick: closeModal, "data-autofocus": "" }),
      okButton,
    ]),
  ]);
}

/* ----- logs ----- */

async function renderLogs(main, generation) {
  const parts = sectionHeader("logsTitle", "logsSub");
  const kindSelect = el("select", {}, [
    el("option", { value: "error" }, t("errorLog")),
    el("option", { value: "access" }, t("accessLog")),
  ]);
  kindSelect.setAttribute("aria-label", t("logsTitle"));
  const linesInput = el("input", { type: "number", value: "200", min: "1", max: "1000", inputmode: "numeric" });
  linesInput.setAttribute("aria-label", t("lines"));
  const filterInput = el("input", { type: "text", class: "wide", placeholder: t("filter") });
  const searchInput = el("input", { type: "text", class: "wide", placeholder: t("searchPattern") });
  const refreshButton = el("button", { class: "primary", text: t("refresh") });
  const searchButton = el("button", { text: t("search") });
  const meta = el("div", { class: "sub" });
  const tailNotice = el("div");
  const out = el("pre", { class: "log", text: "" });
  const searchOut = el("div");

  async function doTail() {
    const gen = state.generation;
    meta.textContent = "";
    tailNotice.replaceChildren();
    out.textContent = t("loading");
    try {
      const res = await call("logs.tail", {
        connectionId: state.connectionId,
        kind: kindSelect.value,
        lines: clampLines(linesInput.value),
        filter: filterInput.value,
      });
      if (!current(gen)) return;
      const metaParts = res.path ? [res.path] : [];
      if (res.source) metaParts.push(t("source") + ": " + res.source);
      if (res.totalLines != null && res.totalLines >= 0) metaParts.push(t("totalLines") + ": " + res.totalLines);
      if (res.windowStartLine > 0) metaParts.push(t("windowStart") + ": " + res.windowStartLine);
      metaParts.push(res.matched + "/" + res.scanned, fmtBytes(res.sizeBytes));
      if (res.level) metaParts.push(t("level") + ": " + res.level);
      meta.textContent = metaParts.join(" · ");
      if (res.truncated) tailNotice.append(el("div", { class: "alert bad", text: t("tailTruncated") }));
      if (res.note) tailNotice.append(el("div", { class: "alert", text: res.note }));
      out.textContent = (res.lines || []).join("\n") || "-";
    } catch (err) {
      if (!current(gen)) return;
      meta.textContent = "";
      out.textContent = errorText(err);
    }
  }

  async function doSearch() {
    if (!searchInput.value.trim()) return;
    const gen = state.generation;
    searchOut.replaceChildren(el("div", { class: "skel", text: t("loading") }));
    try {
      const res = await call("logs.search", {
        connectionId: state.connectionId,
        kind: kindSelect.value,
        pattern: searchInput.value,
        maxMatches: 100,
      });
      if (!current(gen)) return;
      const rows = (res.matches || []).map(function (m) {
        return [m.windowLine != null ? m.windowLine : m.line, m.text];
      });
      const countLabel = t("search") + " (" + (res.matches ? res.matches.length : 0) + (res.more ? "+" : "") + ")";
      const windowBytes = res.windowBytes != null ? res.windowBytes : res.scannedBytes;
      const metaParts = [res.path || ""];
      metaParts.push(t("searchWindow") + ": " + fmtBytes(windowBytes));
      if (res.windowStartOffset != null) metaParts.push(t("windowStart") + ": " + fmtBytes(res.windowStartOffset));
      if (res.fileSizeBytes != null) metaParts.push(t("fileSize") + ": " + fmtBytes(res.fileSizeBytes));
      if (res.source) metaParts.push(t("source") + ": " + res.source);
      const nodes = [
        el("h2", { text: countLabel }),
        el("p", { class: "sub", text: metaParts.filter(Boolean).join(" · ") }),
      ];
      if (res.note) nodes.push(el("p", { class: "sub", text: t("note") + ": " + res.note }));
      if (res.more) nodes.push(el("div", { class: "alert", text: t("searchTruncated") }));
      nodes.push(rows.length ? table(["#", t("lineColumn")], rows) : el("div", { class: "empty", text: "-" }));
      searchOut.replaceChildren(...nodes);
    } catch (err) {
      if (!current(gen)) return;
      searchOut.replaceChildren(el("div", { class: "alert bad", text: errorText(err) }));
    }
  }

  refreshButton.onclick = doTail;
  searchButton.onclick = doSearch;
  searchInput.addEventListener("keydown", function (e) { if (e.key === "Enter") doSearch(); });
  filterInput.addEventListener("keydown", function (e) { if (e.key === "Enter") doTail(); });

  parts.push(
    el("div", { class: "row" }, [
      kindSelect,
      el("label", { text: t("lines") }),
      linesInput,
      filterInput,
      refreshButton,
    ]),
    tailNotice,
    meta,
    out,
    el("div", { class: "row" }, [searchInput, searchButton]),
    searchOut
  );
  main.replaceChildren(...parts);
  doTail();
}

function clampLines(raw) {
  const n = Number.parseInt(String(raw), 10);
  if (!Number.isFinite(n)) return 200;
  return Math.min(1000, Math.max(1, n));
}

/* ----- stats ----- */

function countTable(entries, label) {
  const rows = (entries || []).map(function (e) {
    return [e.value, e.count, e.avgMs != null ? Number(e.avgMs.toFixed(1)) : ""];
  });
  return table([label, t("count"), t("avgMs")], rows);
}

// Newer backends report the bytes actually analyzed; older ones only echo the
// requested window in windowBytes. Read both shapes.
function analyzedBytes(stats) {
  const explicit = firstNumber(stats, ["analyzedBytes", "scannedBytes"]);
  if (explicit != null) return explicit;
  return numOrNull(stats.windowBytes);
}

function timingRow(stats, kind) {
  const nestedKey = kind === "upstream" ? "upstreamTime" : "requestTime";
  const avgKeys = kind === "upstream" ? ["avgUpstreamMs"] : ["avgRequestMs"];
  const maxKeys = kind === "upstream" ? ["maxUpstreamMs"] : ["maxRequestMs"];
  const nested = stats[nestedKey];
  let avg = null;
  let max = null;
  let samples = null;
  if (nested && typeof nested === "object") {
    avg = firstNumber(nested, ["avgMs", "avg"]);
    max = firstNumber(nested, ["maxMs", "max"]);
    samples = firstNumber(nested, ["samples", "count"]);
  }
  if (avg == null) avg = firstNumber(stats, avgKeys);
  if (max == null) max = firstNumber(stats, maxKeys);
  if (avg == null && max == null) return null;
  let value = fmtMs(avg) + " / " + fmtMs(max);
  if (samples != null) value += " (n=" + samples + ")";
  return [t(nestedKey), value];
}

async function renderStats(main, generation) {
  const parts = sectionHeader("statsTitle", "statsSub");
  const bytesSelect = el("select", {}, [
    ["20", "20 MB"], ["50", "50 MB"], ["100", "100 MB"],
  ].map(function (entry) {
    return el("option", { value: String(Number(entry[0]) * 1048576) }, entry[1]);
  }));
  bytesSelect.setAttribute("aria-label", t("statsTitle"));
  const button = el("button", { class: "primary", text: t("refresh") });
  const out = el("div");

  async function run() {
    const gen = state.generation;
    out.replaceChildren(el("div", { class: "skel", text: t("loading") }));
    try {
      const requestedArg = Number(bytesSelect.value);
      const s = await call("access.stats", { connectionId: state.connectionId, bytes: requestedArg });
      if (!current(gen)) return;
      const analyzed = analyzedBytes(s);
      const requested = numOrNull(s.requestedBytes) != null ? s.requestedBytes : requestedArg;
      const truncated = s.truncated === true;
      const samples = s.requestTime && numOrNull(s.requestTime.samples);
      const facts = [
        [t("pathLabel"), s.path, true],
        [t("requestedBytes"), fmtBytes(requested)],
        [t("analyzedBytes"), fmtBytes(analyzed)],
        [t("scanned"), s.parsed + "/" + s.total],
        [t("unparsed"), String(s.unparsed != null ? s.unparsed : Math.max(0, s.total - s.parsed))],
        ["5xx", String(countOf(s.classes, "5xx"))],
        [t("timeRange"), fmtTime(s.timeFrom) + " → " + fmtTime(s.timeTo)],
      ];
      const upstreamTiming = timingRow(s, "upstream");
      if (upstreamTiming) facts.push(upstreamTiming);
      const requestTiming = timingRow(s, "request");
      if (requestTiming) facts.push(requestTiming);

      const grid = el("div");
      grid.append(el("h2", { text: t("statsTitle") }), factsList(facts));
      if (s.gzipUnsupported) grid.append(el("div", { class: "alert bad", text: t("gzipUnsupported") }));
      if (truncated) grid.append(el("div", { class: "alert bad", text: t("statsTruncated") }));
      grid.append(
        el("h2", { text: t("statusCodes") }), countTable(s.statuses, t("statusCode")),
        el("h2", { text: t("statusClasses") }), countTable(s.classes, t("classLabel")),
        el("h2", { text: t("methods") }), countTable(s.methods, t("methodLabel")),
        el("h2", { text: t("topIPs") }), countTable(s.topIPs, t("ipLabel")),
        el("h2", { text: t("topPaths") }), countTable(s.topPaths, t("pathLabel"))
      );
      if (samples != null) {
        grid.append(el("p", { class: "sub", text: t("requestTime") + ": n=" + samples }));
      }
      out.replaceChildren(grid);
    } catch (err) {
      if (!current(gen)) return;
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

async function renderCerts(main, generation) {
  const report = await call("certs.list", { connectionId: state.connectionId });
  if (!current(generation)) return;
  const parts = sectionHeader("certsTitle", "certsSub");
  const certificates = Array.isArray(report.certificates) ? report.certificates : [];
  const rows = certificates.map(function (c) {
    let status;
    if (c.skipped) status = pill(t("skipped"), "dim");
    else if (c.parseError) status = pill(t("parseError"), "bad");
    else if (!c.readable) status = pill(t("unreadable"), "warn");
    else if (c.expired || (c.daysLeft != null && c.daysLeft <= 0)) status = pill(t("expired"), "bad");
    else if (c.expiringSoon) status = pill(t("expiringSoon") + " " + c.daysLeft + t("days"), "warn");
    else status = pill((c.daysLeft != null ? c.daysLeft : "-") + t("days"), "ok");
    const note = c.reason || c.parseErrorText || c.skipReason || c.error || (c.skipped ? t("skippedNote") : "");
    return [
      c.file && c.file !== c.path ? c.path + " → " + c.file : c.path,
      (c.domains || []).join(", ") || "-",
      c.issuer || "-",
      fmtTime(c.notAfter),
      status,
      note,
    ];
  });
  const summary = [];
  if (report.expiringSoon) summary.push(t("expiringSoon") + ": " + report.expiringSoon);
  if (report.expired) summary.push(t("expired") + ": " + report.expired);
  if (report.unreadable) summary.push(t("unreadable") + ": " + report.unreadable);
  if (report.parseErrors) summary.push(t("parseError") + ": " + report.parseErrors);
  if (report.skipped) summary.push(t("skipped") + ": " + report.skipped);
  if (summary.length) parts.push(el("div", { class: "alert", text: summary.join(" · ") }));
  if (report.truncated) parts.push(el("div", { class: "alert bad", text: t("truncated") }));
  parts.push(rows.length
    ? table([t("files"), t("domains"), t("issuer"), t("notAfter"), "", t("note")], rows)
    : el("div", { class: "empty", text: "-" }));
  main.replaceChildren(...parts);
}

/* ----- upstreams ----- */

async function renderUpstreams(main, generation) {
  const parts = sectionHeader("upstreamsTitle", "upstreamsSub");
  const button = el("button", { class: "primary", text: t("check") });
  const out = el("div");
  button.onclick = async function () {
    const gen = state.generation;
    button.disabled = true;
    out.replaceChildren(el("div", { class: "skel", text: t("loading") }));
    try {
      const check = await call("upstreams.check", { connectionId: state.connectionId });
      if (!current(gen)) return;
      const rows = [];
      let skippedMembers = 0;
      for (const u of check.upstreams || []) {
        for (const m of u.servers || []) {
          const skipped = m.alive == null || m.skipped === true;
          if (skipped) skippedMembers++;
          const pills = [];
          if (skipped) pills.push(pill(t("skipped"), "dim"));
          else if (m.alive) pills.push(pill(t("alive") + (m.latencyMs != null ? " " + m.latencyMs + "ms" : ""), "ok"));
          else pills.push(pill(t("down"), "bad"));
          if (m.disabled) pills.push(pill(t("disabled"), "dim"));
          if (m.backup) pills.push(pill(t("backup"), "dim"));
          const note = m.skipReason || m.reason || m.skippedReason || m.error || (skipped ? t("skippedNote") : "");
          rows.push([u.name, m.address, (m.params || []).join(" "), el("span", { class: "pills" }, pills), note]);
        }
      }
      const skippedTotal = check.skipped != null ? check.skipped : skippedMembers;
      const summaryParts = [
        t("alive") + ": " + check.alive,
        t("down") + ": " + check.down,
        t("skipped") + ": " + skippedTotal,
      ];
      if (check.backup != null) summaryParts.push(t("backup") + ": " + check.backup);
      out.replaceChildren(
        rows.length
          ? table([t("upstreams"), t("address"), t("paramsLabel"), "", t("note")], rows)
          : el("div", { class: "empty", text: "-" }),
        el("p", { class: "sub", text: summaryParts.join(" · ") })
      );
    } catch (err) {
      if (!current(gen)) return;
      out.replaceChildren(el("div", { class: "alert bad", text: errorText(err) }));
    } finally {
      button.disabled = false;
    }
  };
  parts.push(el("div", { class: "row" }, [button]), out);
  main.replaceChildren(...parts);
}

/* ----- connections ----- */

async function renderConnections(main) {
  const parts = sectionHeader("connectionsTitle", "connectionsSub");
  parts.push(el("div", { class: "alert", text: t("connHintPassword") }));
  const list = el("div");
  for (const c of state.connections) {
    const row = el("div", { class: "row" }, [
      el("b", { text: c.name }),
      el("span", { class: "mono", text: c.username + "@" + c.host + ":" + c.port }),
      pill(t(c.authType === "key" ? "keyPath" : "password"), "dim"),
      el("button", {
        text: t("test"),
        onclick: async function (e) {
          const target = e.currentTarget;
          target.disabled = true;
          try {
            const res = await call("connections.test", { id: c.id });
            state.testCache.set(c.id, { ok: true, fingerprint: res.fingerprint || "", latencyMs: res.latencyMs });
            toast(t("testOk") + (res.latencyMs != null ? " · " + res.latencyMs + " ms" : ""));
          } catch (err) {
            state.testCache.set(c.id, { ok: false, message: errorText(err) });
            toast(errorText(err), true);
          }
          target.disabled = false;
          refreshConnMeta();
        },
      }),
      el("button", { text: t("edit"), onclick: function () { connForm(c); } }),
      el("button", {
        class: "danger",
        text: t("delete"),
        onclick: function () { confirmDelete(c); },
      }),
    ]);
    list.append(row);
  }
  if (!state.connections.length) parts.push(el("div", { class: "empty", text: t("emptyNoConn") }));
  parts.push(list, el("div", { class: "row" }, [
    el("button", { class: "primary", text: t("addConn"), onclick: function () { connForm(null); } }),
  ]));
  main.replaceChildren(...parts);
}

function confirmDelete(conn) {
  const cancelButton = el("button", { text: t("cancel"), onclick: closeModal, "data-autofocus": "" });
  const deleteButton = el("button", {
    class: "danger",
    text: t("delete"),
    onclick: async function () {
      deleteButton.disabled = true;
      try {
        await call("connections.delete", { id: conn.id });
        state.testCache.delete(conn.id);
        state.infoCache.delete(conn.id);
        if (state.connectionId === conn.id) {
          state.connectionId = "";
          storage.remove(STORE_KEY);
        }
        await loadConnections();
        closeModal();
        toast(t("deleted"));
        renderConnSelect();
        renderSection();
      } catch (err) {
        deleteButton.disabled = false;
        closeModal();
        toast(errorText(err), true);
      }
    },
  });
  modal([
    el("h3", { text: t("deleteTitle") + ": " + conn.name }),
    el("p", { text: conn.username + "@" + conn.host + ":" + conn.port }),
    el("p", { class: "sub", text: t("deleteConfirmBody") }),
    el("div", { class: "actions" }, [cancelButton, deleteButton]),
  ]);
}

function cleanKeyPath(value) {
  let path = String(value || "").trim();
  if (path.length >= 2) {
    const first = path[0];
    const last = path[path.length - 1];
    if ((first === '"' && last === '"') || (first === "'" && last === "'")) {
      path = path.slice(1, -1).trim();
    }
  }
  return path;
}

function parsePort(raw) {
  const text = String(raw == null ? "" : raw).trim();
  if (!/^[0-9]+$/.test(text)) return null;
  const port = Number(text);
  return Number.isInteger(port) && port >= 1 && port <= 65535 ? port : null;
}

function connForm(existing) {
  const authSelect = el("select", {}, [
    el("option", { value: "password" }, t("password")),
    el("option", { value: "key" }, t("keyPath")),
  ]);
  if (existing) authSelect.value = existing.authType === "key" ? "key" : "password";

  const nameInput = el("input", { type: "text", value: existing ? existing.name : "", "data-autofocus": "", maxlength: "100", spellcheck: "false" });
  const hostInput = el("input", { type: "text", value: existing ? existing.host : "", maxlength: "255", spellcheck: "false", autocapitalize: "off" });
  const portInput = el("input", { type: "number", value: existing ? String(existing.port) : "22", min: "1", max: "65535", inputmode: "numeric" });
  const userInput = el("input", { type: "text", value: existing ? existing.username : "", maxlength: "100", spellcheck: "false", autocapitalize: "off" });
  const passwordInput = el("input", {
    type: "password",
    autocomplete: "new-password",
    placeholder: existing && existing.hasPassword ? "••••••" : "",
  });
  const keyPathInput = el("input", {
    type: "text",
    value: existing && existing.keyPath ? existing.keyPath : "",
    maxlength: "1024",
    spellcheck: "false",
  });
  const keyPathPicker = el("button", {
    type: "button",
    text: t("chooseFile"),
    title: t("keyFilePickTitle"),
    "aria-label": t("keyFilePickTitle"),
  });
  const canPickKeyFile = window.gonaviPlugin && typeof window.gonaviPlugin.pickFile === "function";
  if (!canPickKeyFile) {
    keyPathPicker.disabled = true;
    keyPathPicker.title = t("keyFilePickUnavailable");
  }
  keyPathPicker.addEventListener("click", async function () {
    if (!canPickKeyFile) {
      formError.textContent = t("keyFilePickUnavailable");
      return;
    }
    keyPathPicker.disabled = true;
    try {
      const path = await window.gonaviPlugin.pickFile({ title: t("keyFilePickTitle") });
      if (path) {
        const cleaned = cleanKeyPath(path);
        keyPathInput.value = cleaned;
        keyPathInput.focus();
        keyPathInput.setSelectionRange(cleaned.length, cleaned.length);
      }
    } catch {
      formError.textContent = t("keyFilePickFailed");
    } finally {
      keyPathPicker.disabled = false;
    }
  });
  const keyPathField = el("div", { class: "path-field" }, [keyPathInput, keyPathPicker]);
  const keyPassInput = el("input", {
    type: "password",
    autocomplete: "new-password",
    placeholder: existing && existing.hasKeyPassphrase ? "••••••" : "",
  });

  const passwordLabel = el("label", { text: t("password") });
  const keyPathLabel = el("label", { text: t("keyPath") });
  const keyPassLabel = el("label", { text: t("keyPassphrase") });
  const passwordHint = existing && existing.hasPassword
    ? el("div", { class: "field-note wide-row", text: t("passwordUnchangedHint") }) : null;
  const passphraseHint = existing && existing.hasKeyPassphrase
    ? el("div", { class: "field-note wide-row", text: t("passphraseUnchangedHint") }) : null;

  const syncAuth = function () {
    const passwordMode = authSelect.value === "password";
    const passwordNodes = [passwordLabel, passwordInput, passwordHint];
    const keyNodes = [keyPathLabel, keyPathField, keyPassLabel, keyPassInput, passphraseHint];
    for (const node of passwordNodes) {
      if (node) node.classList.toggle("hidden", !passwordMode);
    }
    for (const node of keyNodes) {
      if (node) node.classList.toggle("hidden", passwordMode);
    }
  };
  authSelect.addEventListener("change", syncAuth);

  const formError = el("div", { class: "field-note bad wide-row", role: "alert" });
  const form = el("form", { class: "conn" }, [
    formError,
    el("label", { text: t("name") }), nameInput,
    el("label", { text: t("host") }), hostInput,
    el("label", { text: t("port") }), portInput,
    el("label", { text: t("username") }), userInput,
    el("label", { text: t("authType") }), authSelect,
    passwordLabel, passwordInput,
    passwordHint,
    keyPathLabel, keyPathField,
    keyPassLabel, keyPassInput,
    passphraseHint,
  ]);
  syncAuth();

  const saveButton = el("button", {
    class: "primary",
    text: t("save"),
    onclick: async function (e) {
      e.preventDefault();
      formError.textContent = "";
      portInput.removeAttribute("aria-invalid");
      const name = nameInput.value.trim();
      const host = hostInput.value.trim();
      const username = userInput.value.trim();
      if (!name || !host || !username) {
        formError.textContent = t("formError") + ": " + t("requiredFields");
        return;
      }
      const port = parsePort(portInput.value);
      if (port == null) {
        portInput.setAttribute("aria-invalid", "true");
        formError.textContent = t("portInvalid");
        portInput.focus();
        return;
      }
      if (authSelect.value === "password" && !passwordInput.value && !(existing && existing.hasPassword)) {
        formError.textContent = t("passwordRequired");
        passwordInput.focus();
        return;
      }
      if (authSelect.value === "key" && !keyPathInput.value.trim()) {
        formError.textContent = t("keyPathRequired");
        keyPathInput.focus();
        return;
      }
      const payload = {
        id: existing ? existing.id : "",
        name: name,
        host: host,
        port: port,
        username: username,
        authType: authSelect.value,
      };
      if (authSelect.value === "password") {
        if (passwordInput.value) payload.password = passwordInput.value;
      } else {
        payload.keyPath = cleanKeyPath(keyPathInput.value);
        keyPathInput.value = payload.keyPath;
        if (keyPassInput.value) payload.keyPassphrase = keyPassInput.value;
      }
      saveButton.disabled = true;
      try {
        const res = await call("connections.save", payload);
        state.connectionId = res.connection.id;
        state.testCache.delete(state.connectionId);
        state.infoCache.delete(state.connectionId);
        storage.set(STORE_KEY, state.connectionId);
        await loadConnections();
        closeModal();
        toast(t("saved"));
        renderConnSelect();
        renderSection();
      } catch (err) {
        formError.textContent = errorText(err);
      }
      saveButton.disabled = false;
    },
  });
  modal([
    el("h3", { text: existing ? t("edit") + " — " + existing.name : t("addConn") }),
    form,
    el("div", { class: "actions" }, [
      el("button", { text: t("cancel"), onclick: closeModal }),
      saveButton,
    ]),
  ]);
  form.addEventListener("submit", function (e) { e.preventDefault(); });
  form.addEventListener("keydown", function (e) {
    if (e.key === "Enter" && e.target && e.target.tagName !== "TEXTAREA") {
      e.preventDefault();
      saveButton.click();
    }
  });
  nameInput.focus();
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
    toast(errorText(err), true);
  }
  if (state.connectionId && !state.connections.some(function (c) { return c.id === state.connectionId; })) {
    state.connectionId = "";
    storage.remove(STORE_KEY);
  }
  state.section = state.connections.length ? "overview" : "connections";
  renderShell();
  renderSection();
})();
