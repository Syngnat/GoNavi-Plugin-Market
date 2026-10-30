import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type PointerEvent as ReactPointerEvent } from 'react';
import { Link } from 'react-router-dom';
import type { PluginItem } from '../api/types';
import { InstallButton } from './InstallButton';

export type GalaxyCategory = '全部' | '数据源' | 'AI 供应商' | '导出器' | '技能' | '面板' | '命令' | '运维' | '其他';

export const GALAXY_CATEGORIES: GalaxyCategory[] = [
  '全部',
  '数据源',
  'AI 供应商',
  '导出器',
  '技能',
  '面板',
  '命令',
  '运维',
  '其他',
];

const SERVER_CATEGORY_LABELS: Record<string, GalaxyCategory> = {
  datasource: '数据源',
  'ai-provider': 'AI 供应商',
  exporter: '导出器',
  skill: '技能',
  panel: '面板',
  command: '命令',
  ops: '运维',
  other: '其他',
  数据源: '数据源',
  'AI 供应商': 'AI 供应商',
  导出器: '导出器',
  技能: '技能',
  面板: '面板',
  命令: '命令',
  运维: '运维',
  其他: '其他',
};

const CATEGORY_COLORS: Record<GalaxyCategory, string> = {
  全部: '#77d8c6',
  数据源: '#77d8c6',
  'AI 供应商': '#6fb8e9',
  导出器: '#d5b56d',
  技能: '#ae9be8',
  面板: '#7ec9a8',
  命令: '#d9d0b8',
  运维: '#cf7542',
  其他: '#a6b4b3',
};

const PLANETS = [
  '/galaxy/copper-giant.png?v=4',
  '/galaxy/teal-ocean.png?v=4',
  '/galaxy/ice-world.png?v=4',
  '/galaxy/violet-world.png?v=4',
];

const WORLD = { width: 1440, height: 900, cx: 738, cy: 423 };
const INNER_ORBIT = { rx: 164, ry: 108 };
const OUTER_ORBIT = { rx: 330, ry: 216 };
const MIN_SCALE = 0.58;
const MAX_SCALE = 3.4;
const DEFAULT_CAMERA = { scale: 1, x: 0, y: 0 };

interface Camera {
  scale: number;
  x: number;
  y: number;
}

interface GalaxyNode {
  item: PluginItem;
  category: GalaxyCategory;
  x: number;
  y: number;
  angle: number;
  rx: number;
  ry: number;
  spin: number;
  orbit: 'inner' | 'outer';
  planet: string;
  color: string;
  sigil: string;
}

function categoryOf(item: PluginItem): GalaxyCategory {
  return SERVER_CATEGORY_LABELS[item.category ?? ''] ?? '面板';
}

function hashText(text: string): number {
  let hash = 2166136261;
  for (let index = 0; index < text.length; index += 1) {
    hash ^= text.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return hash >>> 0;
}

function stableFraction(text: string, salt: number): number {
  const hash = hashText(`${text}:${salt}`);
  return (hash % 10000) / 10000;
}

function makeStars(count: number, salt: number) {
  return Array.from({ length: count }, (_, index) => {
    const key = `star-${salt}-${index}`;
    const mag = stableFraction(key, 1);
    return {
      x: stableFraction(key, 2) * WORLD.width,
      y: stableFraction(key, 3) * WORLD.height,
      r: mag > 0.95 ? 1.8 : mag > 0.82 ? 1.05 : 0.42,
      o: 0.16 + mag * 0.78,
      glow: mag > 0.95,
    };
  });
}

const FAR_STARS = makeStars(170, 11);
const NEAR_STARS = makeStars(42, 29);

function sigilOf(item: PluginItem): string {
  const words = item.name
    .replace(/[^A-Za-z0-9\s]/g, ' ')
    .split(/\s+/)
    .filter(Boolean);
  if (words.length >= 2) return `${words[0][0]}${words[1][0]}`.toUpperCase();
  return item.name.replace(/[^A-Za-z0-9]/g, '').slice(0, 3).toUpperCase() || 'GN';
}

function planetOf(item: PluginItem, index: number): string {
  const category = categoryOf(item);
  if (category === '运维') return PLANETS[0];
  if (category === 'AI 供应商') return PLANETS[1];
  if (category === '数据源' || category === '面板') return PLANETS[2];
  return PLANETS[3 + (index % 1)] ?? PLANETS[index % PLANETS.length];
}

function pluginMatches(item: PluginItem, category: GalaxyCategory, query: string): boolean {
  if (category !== '全部' && categoryOf(item) !== category) return false;
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return [item.name, item.id, item.description, item.author, item.category ?? '']
    .some((value) => value.toLowerCase().includes(q));
}

const SECTOR_ANGLE: Record<GalaxyCategory, number> = {
  全部: 0,
  数据源: -Math.PI / 2,
  'AI 供应商': 0,
  导出器: Math.PI / 4,
  技能: Math.PI / 2,
  面板: (Math.PI * 3) / 4,
  命令: Math.PI,
  运维: (-Math.PI * 3) / 4,
  其他: -Math.PI / 4,
};

const LANES = [
  { rx: 168, ry: 110 },
  { rx: 248, ry: 162 },
  { rx: 338, ry: 220 },
];

function layoutNodes(items: PluginItem[]): GalaxyNode[] {
  const groups = new Map<GalaxyCategory, PluginItem[]>();
  for (const item of items) {
    const category = categoryOf(item);
    const list = groups.get(category) ?? [];
    list.push(item);
    groups.set(category, list);
  }

  const nodes: GalaxyNode[] = [];
  for (const [category, list] of groups) {
    const sorted = [...list].sort((a, b) => {
      if (a.official !== b.official) return a.official ? -1 : 1;
      return a.name.localeCompare(b.name, 'zh');
    });
    const span = Math.min(0.62, Math.max(0, sorted.length - 1) * 0.26);
    const center = SECTOR_ANGLE[category];
    sorted.forEach((item, index) => {
      const along = sorted.length === 1 ? 0 : index / (sorted.length - 1) - 0.5;
      const angle = center + along * span;
      const lane = LANES[index % LANES.length];
      const radiusJitter = 0.96 + stableFraction(item.id, 4) * 0.06;
      const rx = lane.rx * radiusJitter;
      const ry = lane.ry * radiusJitter;
      const at = pointOnOrbit(angle, rx, ry);
      nodes.push({
        item,
        category,
        x: at.x,
        y: at.y,
        angle,
        rx,
        ry,
        spin: 0.008,
        orbit: item.official ? 'inner' : 'outer',
        planet: planetOf(item, index),
        color: CATEGORY_COLORS[category],
        sigil: sigilOf(item),
      });
    });
  }
  return nodes;
}

function formatDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(date).replace(/\//g, '.');
}

function platformLabel(item: PluginItem): string {
  const categories = new Set(item.platforms.map((platform) => platform.split('/')[0]));
  if (categories.size >= 3) return `${item.platforms.length} TARGETS`;
  return Array.from(categories).map((entry) => entry.toUpperCase()).join(' / ') || '—';
}

function digestLabel(item: PluginItem): string {
  const value = item.contentDigest ?? item.artifact?.sha256 ?? '';
  return value ? `${value.slice(0, 16)}…${value.slice(-8)}` : '未提供摘要';
}

function clientPointToView(clientX: number, clientY: number, rect: DOMRect): { x: number; y: number } {
  const viewScale = Math.max(WORLD.width / rect.width, WORLD.height / rect.height);
  const offsetX = (rect.width * viewScale - WORLD.width) / 2;
  const offsetY = (rect.height * viewScale - WORLD.height) / 2;
  return {
    x: (clientX - rect.left) * viewScale - offsetX,
    y: (clientY - rect.top) * viewScale - offsetY,
  };
}

function cameraTransform(camera: Camera): string {
  const tx = camera.x + WORLD.cx * (1 - camera.scale);
  const ty = camera.y + WORLD.cy * (1 - camera.scale);
  return `translate(${tx.toFixed(2)} ${ty.toFixed(2)}) scale(${camera.scale.toFixed(4)})`;
}

const ORBIT_ROTATION = (-11 * Math.PI) / 180;
const FOCUS = { x: 640, y: 448 };
const FLIGHT_MS = 920;

function pointOnOrbit(angle: number, rx: number, ry: number): { x: number; y: number } {
  const lx = Math.cos(angle) * rx;
  const ly = Math.sin(angle) * ry;
  const cos = Math.cos(ORBIT_ROTATION);
  const sin = Math.sin(ORBIT_ROTATION);
  return {
    x: WORLD.cx + lx * cos - ly * sin,
    y: WORLD.cy + lx * sin + ly * cos,
  };
}

function orbitPoint(node: GalaxyNode, now: number, reduced: boolean): { x: number; y: number } {
  const seconds = reduced ? 0 : now / 1000;
  return pointOnOrbit(node.angle + node.spin * seconds, node.rx, node.ry);
}

function clampCamera(camera: Camera): Camera {
  const scale = Math.max(MIN_SCALE, Math.min(MAX_SCALE, camera.scale));
  const spanX = 520 * scale;
  const spanY = 340 * scale;
  return {
    scale,
    x: Math.max(-spanX, Math.min(spanX, camera.x)),
    y: Math.max(-spanY, Math.min(spanY, camera.y)),
  };
}

function frameCamera(worldX: number, worldY: number, scale: number): Camera {
  const next = Math.max(MIN_SCALE, Math.min(MAX_SCALE, scale));
  return clampCamera({
    scale: next,
    x: FOCUS.x - WORLD.cx * (1 - next) - worldX * next,
    y: FOCUS.y - WORLD.cy * (1 - next) - worldY * next,
  });
}

function projectNode(worldX: number, worldY: number, camera: Camera, rect: DOMRect): { x: number; y: number } {
  const viewScale = Math.max(WORLD.width / rect.width, WORLD.height / rect.height);
  const offsetX = (rect.width * viewScale - WORLD.width) / 2;
  const offsetY = (rect.height * viewScale - WORLD.height) / 2;
  const tx = camera.x + WORLD.cx * (1 - camera.scale);
  const ty = camera.y + WORLD.cy * (1 - camera.scale);
  return {
    x: (tx + worldX * camera.scale + offsetX) / viewScale,
    y: (ty + worldY * camera.scale + offsetY) / viewScale,
  };
}

function easeOutQuart(t: number): number {
  return 1 - (1 - t) ** 4;
}

interface Flight {
  from: Camera;
  to: Camera;
  start: number;
  duration: number;
}

interface Rig {
  current: Camera;
  desiredScale: number;
  vx: number;
  vy: number;
  mode: 'idle' | 'drag' | 'coast' | 'zoom' | 'flight';
  zoomAnchor: { x: number; y: number } | null;
  flight: Flight | null;
  reduced: boolean;
}

const MODE_LABEL: Record<Rig['mode'], string> = {
  idle: 'ORBITAL',
  drag: 'PAN',
  coast: 'INERTIA',
  zoom: 'ZOOM',
  flight: 'GLIDE',
};

function stepRig(rig: Rig, dt: number, now: number) {
  if (rig.mode === 'coast') {
    const next = {
      ...rig.current,
      x: rig.current.x + rig.vx * dt,
      y: rig.current.y + rig.vy * dt,
    };
    const clamped = clampCamera(next);
    if (clamped.x !== next.x) rig.vx = 0;
    if (clamped.y !== next.y) rig.vy = 0;
    rig.current = clamped;
    const decay = Math.exp(-dt / 0.45);
    rig.vx *= decay;
    rig.vy *= decay;
    if (Math.hypot(rig.vx, rig.vy) < 18) {
      rig.vx = 0;
      rig.vy = 0;
      rig.mode = 'idle';
    }
    return;
  }
  if (rig.mode === 'zoom' && rig.zoomAnchor) {
    const delta = rig.desiredScale - rig.current.scale;
    if (Math.abs(delta) < 0.0012) {
      const applied = rig.current.scale === 0 ? 1 : rig.desiredScale / rig.current.scale;
      const anchor = rig.zoomAnchor;
      rig.current = clampCamera({
        scale: rig.desiredScale,
        x: anchor.x - WORLD.cx - (anchor.x - WORLD.cx - rig.current.x) * applied,
        y: anchor.y - WORLD.cy - (anchor.y - WORLD.cy - rig.current.y) * applied,
      });
      rig.mode = 'idle';
      return;
    }
    const amount = 1 - Math.exp(-dt / 0.12);
    const nextScale = rig.current.scale + delta * amount;
    const applied = rig.current.scale === 0 ? 1 : nextScale / rig.current.scale;
    const anchor = rig.zoomAnchor;
    rig.current = clampCamera({
      scale: nextScale,
      x: anchor.x - WORLD.cx - (anchor.x - WORLD.cx - rig.current.x) * applied,
      y: anchor.y - WORLD.cy - (anchor.y - WORLD.cy - rig.current.y) * applied,
    });
    return;
  }
  if (rig.mode === 'flight' && rig.flight) {
    const t = Math.min(1, (now - rig.flight.start) / rig.flight.duration);
    const eased = easeOutQuart(t);
    const { from, to } = rig.flight;
    rig.current = {
      scale: from.scale + (to.scale - from.scale) * eased,
      x: from.x + (to.x - from.x) * eased,
      y: from.y + (to.y - from.y) * eased,
    };
    if (t >= 1) {
      rig.current = to;
      rig.desiredScale = to.scale;
      rig.mode = 'idle';
      rig.flight = null;
    }
  }
}

function zoomLevel(scale: number): string {
  if (scale >= 1.5) return 'close';
  if (scale < 0.8) return 'far';
  return 'normal';
}

interface PluginGalaxyProps {
  items: PluginItem[];
  loading: boolean;
  error: boolean;
  nextCursor?: string;
  loadingMore: boolean;
  stale?: string;
  activeCategory: GalaxyCategory;
  query: string;
  onCategoryChange: (category: GalaxyCategory) => void;
  onQueryChange: (query: string) => void;
  onLoadMore: () => void;
}

export function PluginGalaxy({
  items,
  loading,
  error,
  nextCursor,
  loadingMore,
  stale,
  activeCategory,
  query,
  onCategoryChange,
  onQueryChange,
  onLoadMore,
}: PluginGalaxyProps) {
  const observatoryRef = useRef<HTMLElement>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const worldRef = useRef<SVGGElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const cursorRef = useRef<HTMLSpanElement>(null);
  const scaleRef = useRef<HTMLSpanElement>(null);
  const modeReadoutRef = useRef<HTMLSpanElement>(null);
  const nodeEls = useRef(new Map<string, HTMLButtonElement>());
  const paintRef = useRef<(now: number) => void>(() => {});
  const nodes = useMemo(() => layoutNodes(items), [items]);
  const visibleNodes = useMemo(
    () => nodes.filter((node) => pluginMatches(node.item, activeCategory, query)),
    [nodes, activeCategory, query],
  );
  const [selectedId, setSelectedId] = useState('');
  const selected = nodes.find((node) => node.item.id === selectedId) ?? visibleNodes[0] ?? null;

  useEffect(() => {
    if (visibleNodes.length > 0 && !visibleNodes.some((node) => node.item.id === selectedId)) {
      setSelectedId(visibleNodes[0].item.id);
    }
  }, [visibleNodes, selectedId]);

  const [view, setView] = useState<'map' | 'list'>('map');
  const nodesRef = useRef(nodes);
  const viewRef = useRef(view);
  const rigRef = useRef<Rig>({
    current: { ...DEFAULT_CAMERA },
    desiredScale: DEFAULT_CAMERA.scale,
    vx: 0,
    vy: 0,
    mode: 'idle',
    zoomAnchor: null,
    flight: null,
    reduced: false,
  });
  const dragRef = useRef({
    pointerId: -1,
    startX: 0,
    startY: 0,
    originX: 0,
    originY: 0,
    lastX: 0,
    lastY: 0,
    lastT: 0,
    moved: false,
  });
  const suppressClickRef = useRef(false);
  nodesRef.current = nodes;
  viewRef.current = view;

  const bindNode = useCallback((id: string) => (element: HTMLButtonElement | null) => {
    if (element) nodeEls.current.set(id, element);
    else nodeEls.current.delete(id);
  }, []);

  const selectNode = useCallback((id: string) => {
    setSelectedId(id);
  }, []);

  const focusNode = useCallback((node: GalaxyNode, targetScale = 1.48) => {
    const rig = rigRef.current;
    const now = performance.now();
    const live = orbitPoint(node, now, rig.reduced);
    const to = frameCamera(live.x, live.y, targetScale);
    if (rig.reduced) {
      rig.current = to;
      rig.desiredScale = to.scale;
      rig.mode = 'idle';
      rig.flight = null;
      paintRef.current(now);
      return;
    }
    rig.flight = { from: { ...rig.current }, to, start: now, duration: FLIGHT_MS };
    rig.mode = 'flight';
    rig.vx = 0;
    rig.vy = 0;
  }, []);

  const focusHome = useCallback(() => {
    const rig = rigRef.current;
    const now = performance.now();
    const to = { ...DEFAULT_CAMERA };
    if (rig.reduced) {
      rig.current = to;
      rig.desiredScale = to.scale;
      rig.mode = 'idle';
      rig.flight = null;
      paintRef.current(now);
      return;
    }
    rig.flight = { from: { ...rig.current }, to, start: now, duration: FLIGHT_MS };
    rig.mode = 'flight';
    rig.vx = 0;
    rig.vy = 0;
  }, []);

  const beginZoom = useCallback((clientX: number, clientY: number, factor: number) => {
    const rect = observatoryRef.current?.getBoundingClientRect();
    if (!rect) return;
    const rig = rigRef.current;
    const base = rig.mode === 'zoom' ? rig.desiredScale : rig.current.scale;
    const scale = Math.max(MIN_SCALE, Math.min(MAX_SCALE, base * factor));
    rig.desiredScale = scale;
    rig.zoomAnchor = clientPointToView(clientX, clientY, rect);
    rig.flight = null;
    rig.vx = 0;
    rig.vy = 0;
    if (rig.reduced || Math.abs(scale - rig.current.scale) < 0.0001) {
      const applied = rig.current.scale === 0 ? 1 : scale / rig.current.scale;
      const anchor = rig.zoomAnchor;
      rig.current = clampCamera({
        scale,
        x: anchor.x - WORLD.cx - (anchor.x - WORLD.cx - rig.current.x) * applied,
        y: anchor.y - WORLD.cy - (anchor.y - WORLD.cy - rig.current.y) * applied,
      });
      rig.desiredScale = rig.current.scale;
      rig.mode = 'idle';
      paintRef.current(performance.now());
      return;
    }
    rig.mode = 'zoom';
  }, []);

  useLayoutEffect(() => {
    const root = observatoryRef.current;
    const media = window.matchMedia('(prefers-reduced-motion: reduce)');
    const syncReduced = () => {
      rigRef.current.reduced = media.matches;
    };
    syncReduced();
    media.addEventListener('change', syncReduced);

    const paint = (now: number) => {
      const rig = rigRef.current;
      const camera = rig.current;
      const world = worldRef.current;
      const host = observatoryRef.current;
      if (world) {
        world.setAttribute('transform', cameraTransform(camera));
        world.dataset.zoomLevel = zoomLevel(camera.scale);
      }
      if (host) {
        host.dataset.zoomLevel = zoomLevel(camera.scale);
        host.style.setProperty('--star-shift-x', `${(camera.x * 0.055).toFixed(2)}px`);
        host.style.setProperty('--star-shift-y', `${(camera.y * 0.055).toFixed(2)}px`);
        host.style.setProperty('--grid-shift-x', `${(camera.x * 0.022).toFixed(2)}px`);
        host.style.setProperty('--grid-shift-y', `${(camera.y * 0.022).toFixed(2)}px`);
      }
      const rect = host?.getBoundingClientRect();
      if (rect && rect.width > 0) {
        const body = Math.max(0.8, Math.min(1.62, camera.scale)).toFixed(3);
        for (const node of nodesRef.current) {
          const element = nodeEls.current.get(node.item.id);
          if (!element) continue;
          const live = orbitPoint(node, now, rig.reduced);
          const projected = projectNode(live.x, live.y, camera, rect);
          element.style.transform = `translate3d(${projected.x.toFixed(2)}px, ${projected.y.toFixed(2)}px, 0) translate(-50%, -50%)`;
          element.style.setProperty('--body-scale', body);
          // Keep the black hole and planet bodies in one visual layer: planets
          // on the near side of the orbit pass over the disk, while planets
          // behind the center are occluded by the shadow.
          element.style.zIndex = live.y >= WORLD.cy ? '4' : '1';
          element.classList.toggle('label-left', live.x < WORLD.cx);
        }
      }
      if (scaleRef.current) scaleRef.current.textContent = `SCALE ${camera.scale.toFixed(2)}×`;
      if (modeReadoutRef.current) modeReadoutRef.current.textContent = `MODE ${MODE_LABEL[rig.mode]}`;
    };
    paintRef.current = paint;

    const handleWheel = (event: WheelEvent) => {
      if (viewRef.current !== 'map') return;
      const target = event.target;
      if (target instanceof Element && target.closest('.galaxy-inspector, .galaxy-list-panel, .galaxy-sectors')) return;
      event.preventDefault();
      const factor = Math.exp(Math.max(-0.42, Math.min(0.42, -event.deltaY * 0.00105)));
      beginZoom(event.clientX, event.clientY, factor);
    };
    root?.addEventListener('wheel', handleWheel, { passive: false });

    let last = performance.now();
    let lastPaint = 0;
    let frame = 0;
    const loop = (now: number) => {
      frame = requestAnimationFrame(loop);
      if (document.visibilityState === 'hidden') return;
      const dt = Math.min(0.034, (now - last) / 1000);
      last = now;
      const rig = rigRef.current;
      if (!rig.reduced && rig.mode !== 'idle' && rig.mode !== 'drag') stepRig(rig, dt, now);
      const interval = rig.reduced ? 100 : 1000 / 24;
      if (now - lastPaint >= interval) {
        paint(now);
        lastPaint = now;
      }
    };
    paint(performance.now());
    frame = requestAnimationFrame(loop);
    return () => {
      cancelAnimationFrame(frame);
      media.removeEventListener('change', syncReduced);
      root?.removeEventListener('wheel', handleWheel);
    };
  }, [beginZoom]);

  useLayoutEffect(() => {
    paintRef.current(performance.now());
  });

  useEffect(() => {
    if (selected && view === 'map' && query.trim()) focusNode(selected, 1.48);
  }, [focusNode, query, selected, view]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target;
      const typing = target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement;
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        searchRef.current?.focus();
        searchRef.current?.select();
        return;
      }
      if (event.key === '/' && !typing) {
        event.preventDefault();
        searchRef.current?.focus();
        searchRef.current?.select();
        return;
      }
      if (event.key === 'Escape' && !typing) {
        onQueryChange('');
        onCategoryChange('全部');
        focusHome();
        return;
      }
      if (event.key === '0' && !typing) {
        focusHome();
        return;
      }
      const rect = observatoryRef.current?.getBoundingClientRect();
      if (!rect || typing || viewRef.current !== 'map') return;
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;
      if (event.key === '+' || event.key === '=') beginZoom(centerX, centerY, 1.16);
      if (event.key === '-') beginZoom(centerX, centerY, 1 / 1.16);
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [beginZoom, focusHome, onCategoryChange, onQueryChange]);

  const onPointerDown = (event: ReactPointerEvent<HTMLElement>) => {
    if (event.button !== 0 || viewRef.current !== 'map') return;
    const target = event.target;
    if (!(target instanceof Element)) return;
    if (target.closest('.galaxy-inspector, .galaxy-sectors, .galaxy-hero, .galaxy-zoom, .galaxy-status, .galaxy-list-panel, .galaxy-metrics, .galaxy-empty, .galaxy-view-toggle')) return;
    const rig = rigRef.current;
    dragRef.current = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      originX: rig.current.x,
      originY: rig.current.y,
      lastX: rig.current.x,
      lastY: rig.current.y,
      lastT: performance.now(),
      moved: false,
    };
  };

  const onPointerMove = (event: ReactPointerEvent<HTMLElement>) => {
    const host = observatoryRef.current;
    const rect = host?.getBoundingClientRect();
    if (rect && cursorRef.current) {
      const point = clientPointToView(event.clientX, event.clientY, rect);
      cursorRef.current.textContent = `CURSOR ${String(Math.round(point.x)).padStart(4, '0')} / ${String(Math.round(point.y)).padStart(4, '0')}`;
    }
    const drag = dragRef.current;
    if (!rect || !host || drag.pointerId !== event.pointerId) return;
    const viewScale = Math.max(WORLD.width / rect.width, WORLD.height / rect.height);
    const dx = (event.clientX - drag.startX) * viewScale;
    const dy = (event.clientY - drag.startY) * viewScale;
    const now = performance.now();
    const wasMoving = drag.moved;
    if (!drag.moved) {
      if (Math.hypot(dx, dy) < 6) return;
      drag.moved = true;
      const rig = rigRef.current;
      rig.mode = 'drag';
      rig.flight = null;
      rig.vx = 0;
      rig.vy = 0;
      host.classList.add('is-panning');
      try {
        host.setPointerCapture(event.pointerId);
      } catch {
        // Synthetic pointers and some embedded WebViews do not support capture.
      }
    }
    const next = clampCamera({
      ...rigRef.current.current,
      x: drag.originX + dx,
      y: drag.originY + dy,
    });
    const rig = rigRef.current;
    if (wasMoving) {
      const dt = Math.max(0.008, (now - drag.lastT) / 1000);
      rig.vx = Math.max(-980, Math.min(980, (next.x - drag.lastX) / dt));
      rig.vy = Math.max(-980, Math.min(980, (next.y - drag.lastY) / dt));
    }
    drag.lastX = next.x;
    drag.lastY = next.y;
    drag.lastT = now;
    rig.current = next;
    rig.desiredScale = next.scale;
    paintRef.current(now);
  };

  const endPointer = (event: ReactPointerEvent<HTMLElement>) => {
    const drag = dragRef.current;
    if (drag.pointerId !== event.pointerId) return;
    const moved = drag.moved;
    drag.pointerId = -1;
    const host = observatoryRef.current;
    host?.classList.remove('is-panning');
    if (host?.hasPointerCapture(event.pointerId)) host.releasePointerCapture(event.pointerId);
    if (!moved) return;
    suppressClickRef.current = true;
    requestAnimationFrame(() => {
      suppressClickRef.current = false;
    });
    const rig = rigRef.current;
    if (!rig.reduced && Math.hypot(rig.vx, rig.vy) > 80) rig.mode = 'coast';
    else {
      rig.mode = 'idle';
      rig.vx = 0;
      rig.vy = 0;
    }
  };

  const categoryCounts = useMemo(() => {
    const result = new Map<GalaxyCategory, number>();
    for (const category of GALAXY_CATEGORIES) {
      result.set(category, category === '全部'
        ? items.length
        : items.filter((item) => categoryOf(item) === category).length);
    }
    return result;
  }, [items]);

  const officialCount = items.filter((item) => item.official).length;
  const visibleCount = visibleNodes.length;
  const hasItems = !error && items.length > 0;

  return (
    <section
      ref={observatoryRef}
      className={`galaxy-observatory ${view === 'list' ? 'is-list-mode' : ''}`}
      aria-label="GoNavi 插件星图"
      data-zoom-level="normal"
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={endPointer}
      onPointerCancel={endPointer}
    >
      <div className="galaxy-backdrop" aria-hidden="true">
        <div className="galaxy-nebula" />
        <div className="galaxy-band" />
        <svg className="galaxy-stars-far" viewBox={`0 0 ${WORLD.width} ${WORLD.height}`} preserveAspectRatio="xMidYMid slice">
          {FAR_STARS.map((star, index) => (
            <circle key={index} cx={star.x} cy={star.y} r={star.r} fill="#e7f3ee" opacity={star.o} />
          ))}
        </svg>
        <svg className="galaxy-stars-near" viewBox={`0 0 ${WORLD.width} ${WORLD.height}`} preserveAspectRatio="xMidYMid slice">
          {NEAR_STARS.map((star, index) => (
            <g key={index}>
              {star.glow && <circle cx={star.x} cy={star.y} r={star.r * 5} fill="#d7fff4" opacity="0.14" />}
              <circle cx={star.x} cy={star.y} r={star.r} fill="#f4fff9" opacity={star.o} />
            </g>
          ))}
        </svg>
        <div className="galaxy-grid" />
        <div className="galaxy-grain" />
      </div>

      <svg
        ref={svgRef}
        className="galaxy-map"
        viewBox={`0 0 ${WORLD.width} ${WORLD.height}`}
        preserveAspectRatio="xMidYMid slice"
        role="application"
        aria-label="可缩放、可拖拽的插件星图"
      >
        <defs>
          <radialGradient id="galaxyHalo">
            <stop offset="0%" stopColor="#6fcbbb" stopOpacity="0.1" />
            <stop offset="45%" stopColor="#3c8884" stopOpacity="0.035" />
            <stop offset="100%" stopColor="#061015" stopOpacity="0" />
          </radialGradient>
          <radialGradient id="galaxyCore">
            <stop offset="0%" stopColor="#f5c098" />
            <stop offset="42%" stopColor="#cf7542" />
            <stop offset="100%" stopColor="#74351f" />
          </radialGradient>
          <linearGradient id="galaxySignal" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0%" stopColor="#cf7542" stopOpacity="0.2" />
            <stop offset="50%" stopColor="#77d8c6" stopOpacity="0.62" />
            <stop offset="100%" stopColor="#77d8c6" stopOpacity="0.08" />
          </linearGradient>
          <radialGradient id="galaxySweep">
            <stop offset="0%" stopColor="#77d8c6" stopOpacity="0" />
            <stop offset="72%" stopColor="#77d8c6" stopOpacity="0.025" />
            <stop offset="100%" stopColor="#77d8c6" stopOpacity="0.28" />
          </radialGradient>
          <radialGradient id="coreCorona">
            <stop offset="0%" stopColor="#ffd7b0" stopOpacity="0.55" />
            <stop offset="18%" stopColor="#cf7542" stopOpacity="0.28" />
            <stop offset="46%" stopColor="#3c8884" stopOpacity="0.08" />
            <stop offset="100%" stopColor="#061015" stopOpacity="0" />
          </radialGradient>
          <radialGradient id="orbitWash">
            <stop offset="62%" stopColor="#12343a" stopOpacity="0" />
            <stop offset="82%" stopColor="#1c4d52" stopOpacity="0.16" />
            <stop offset="100%" stopColor="#77d8c6" stopOpacity="0" />
          </radialGradient>
          <filter id="galaxyPlanetShadow">
            <feDropShadow dx="3" dy="5" stdDeviation="4" floodColor="#000" floodOpacity="0.78" />
          </filter>
        </defs>

        <g
          ref={worldRef}
          className="galaxy-world"
          data-zoom-level="normal"
          transform={cameraTransform(DEFAULT_CAMERA)}
        >
          <circle cx={WORLD.cx} cy={WORLD.cy} r="620" fill="url(#galaxyHalo)" />
          <g className="galaxy-axis" aria-hidden="true" transform={`rotate(-11 ${WORLD.cx} ${WORLD.cy})`}>
            <ellipse className="orbit-wash" cx={WORLD.cx} cy={WORLD.cy} rx={OUTER_ORBIT.rx} ry={OUTER_ORBIT.ry} fill="url(#orbitWash)" />
            <ellipse className="orbit-glow outer" cx={WORLD.cx} cy={WORLD.cy} rx={OUTER_ORBIT.rx} ry={OUTER_ORBIT.ry} />
            <ellipse className="orbit-hair outer" cx={WORLD.cx} cy={WORLD.cy} rx={OUTER_ORBIT.rx} ry={OUTER_ORBIT.ry} />
            <ellipse className="orbit-glow mid" cx={WORLD.cx} cy={WORLD.cy} rx="294" ry="186" />
            <ellipse className="orbit-hair mid is-dashed" cx={WORLD.cx} cy={WORLD.cy} rx="294" ry="186" />
            <ellipse className="orbit-glow inner" cx={WORLD.cx} cy={WORLD.cy} rx={INNER_ORBIT.rx} ry={INNER_ORBIT.ry} />
            <ellipse className="orbit-hair inner" cx={WORLD.cx} cy={WORLD.cy} rx={INNER_ORBIT.rx} ry={INNER_ORBIT.ry} />
          </g>
          <g className="galaxy-axis-labels" aria-hidden="true">
            <text x="728" y="108">NORTH / DATASOURCE</text>
            <text x="1030" y="438">EAST / AI PROVIDER</text>
            <text x="735" y="748">SOUTH / SKILL</text>
            <text x="322" y="430" textAnchor="end">WEST / OPS</text>
          </g>
          <g className="galaxy-core" transform={`translate(${WORLD.cx} ${WORLD.cy})`}>
            <circle className="core-corona" r="230" fill="url(#coreCorona)" />
            <circle className="core-halo" r="96" fill="url(#galaxySweep)" />
            <circle className="core-ring" r="58" />
            <circle className="core-ring reverse" r="40" />
            <circle className="core-body" r="22" />
            <circle className="core-inner" r="16" fill="url(#galaxyCore)" />
            <circle className="core-hot" r="5.5" />
            <text className="core-caption" y="46">GONAVI</text>
          </g>

          <g className="galaxy-coordinates" aria-hidden="true">
            <text x="118" y="118">X 012.4 / Y 086.1</text>
            <text x="118" y="132">SECTOR 07 / LOCAL CLUSTER</text>
            <text x="1038" y="764">R.A. 18H 36M / DEC +38° 47′</text>
          </g>
        </g>
      </svg>

      <div className="galaxy-nodes" aria-label="插件对象">
        {nodes.map((node, index) => {
          const visible = pluginMatches(node.item, activeCategory, query);
          return (
            <button
              key={node.item.id}
              ref={bindNode(node.item.id)}
              type="button"
              className={`galaxy-dom-node ${selected?.item.id === node.item.id ? 'is-selected' : ''} ${visible ? '' : 'is-filtered'}`}
              style={{ '--node-color': node.color, '--node-planet': `url(${node.planet})` } as CSSProperties}
              aria-label={`${node.item.name}，${node.item.official ? '官方' : '社区'}${node.category}插件`}
              onClick={() => {
                if (suppressClickRef.current) return;
                selectNode(node.item.id);
                focusNode(node, 1.7);
              }}
            >
              <span className="dom-node-hit" aria-hidden="true" />
              <span className="dom-node-body" aria-hidden="true">
                <span className="dom-node-orbit" />
                <span className="dom-node-aura" />
                <span className="dom-node-atmo" />
                <span className="dom-node-reticle" />
                <span className="dom-node-sphere">
                  <img className="dom-node-planet" src={node.planet} alt="" draggable={false} />
                </span>
                <span className="dom-node-moon-track"><span className="dom-node-moon" /></span>
              </span>
              <span className="dom-node-label">
                <i />
                <small>{node.item.official ? `GN-${String(index + 1).padStart(3, '0')} / OFFICIAL` : `GH-${String(index + 1).padStart(3, '0')} / COMMUNITY`}</small>
                <strong>{node.item.name}</strong>
                <em>{node.category.toUpperCase()} · V{node.item.version} · {platformLabel(node.item)}</em>
              </span>
            </button>
          );
        })}
      </div>

      <div className="galaxy-metrics" aria-label="目录概况">
        <span><strong>{String(items.length).padStart(2, '0')}</strong> 插件对象</span>
        <span><strong>{String(officialCount).padStart(2, '0')}</strong> 官方核心</span>
        <span><strong>{String(visibleCount).padStart(2, '0')}</strong> 当前可见</span>
      </div>

      <div className="galaxy-hero">
        <div className="galaxy-live">
          <span className="galaxy-live-dot" />
          <span>{stale ? '目录缓存' : '星图在线'}</span>
          <i />
          <span>CATALOG / V1</span>
        </div>
        <h1>插件<span>星图</span></h1>
        <p>
          缩放、搜索并靠近每一个扩展。官方核心轨道与社区星区始终保持可辨，
          <strong>所有插件都在同一张可导航的地图里。</strong>
        </p>
        <label className="galaxy-search">
          <span className="material-symbols-outlined">search</span>
          <input
            ref={searchRef}
            type="search"
            value={query}
            onChange={(event) => onQueryChange(event.target.value)}
            placeholder="搜索插件名称、ID 或能力…"
            aria-label="搜索插件"
          />
          <kbd>/</kbd>
        </label>
      </div>

      <aside className="galaxy-sectors" aria-label="按分类筛选">
        <div className="galaxy-section-label">
          <span>SECTORS / 分类星区</span>
          <b>{stale ? 'CACHE' : 'LIVE'}</b>
        </div>
        <div className="galaxy-sector-list">
          {GALAXY_CATEGORIES.map((category, index) => {
            const count = categoryCounts.get(category) ?? 0;
            return (
              <button
                key={category}
                type="button"
                className={`galaxy-sector ${activeCategory === category ? 'is-active' : ''} ${count > 0 ? 'has-items' : ''}`}
                onClick={() => {
                  onCategoryChange(category);
                  const first = nodes.find((node) => pluginMatches(node.item, category, query));
                  if (first) focusNode(first, category === '全部' ? 1.05 : 1.42);
                }}
              >
                <span>{String(index).padStart(2, '0')}</span>
                <b>{category === '全部' ? '全部插件' : category}</b>
                <i>{String(count).padStart(2, '0')}</i>
              </button>
            );
          })}
        </div>
        <div className="galaxy-legend">
          <span><i className="official" />官方插件</span>
          <span><i className="community" />社区插件</span>
          <span><i className="orbit" />分类轨道</span>
          <span><i className="stardust" />环境星尘</span>
        </div>
      </aside>

      <aside className="galaxy-inspector" aria-live="polite">
        {selected ? (
          <>
            <div className="inspector-kicker">
              <span>SELECTED OBJECT</span>
              <strong>{selected.item.official ? 'INNER ORBIT / OFFICIAL' : 'OUTER ORBIT / COMMUNITY'}</strong>
            </div>
            <div className="inspector-head">
              <div className="inspector-sigil" style={{ '--node-color': selected.color } as CSSProperties}>
                {selected.sigil}
              </div>
              <div className="inspector-copy">
                <div className="inspector-tag" style={{ color: selected.color }}>
                  <i />
                  {selected.item.official ? 'OFFICIAL' : 'COMMUNITY'} · {selected.category}
                </div>
                <h2>{selected.item.name}</h2>
                <p className="inspector-id">{selected.item.id}</p>
              </div>
            </div>
            <p className="inspector-description">{selected.item.description}</p>
            <div className="inspector-telemetry">
              <div><span>VERSION</span><strong>{selected.item.version}</strong></div>
              <div><span>MIN HOST</span><strong>{selected.item.minHostVersion ?? '—'}</strong></div>
              <div><span>PLATFORMS</span><strong>{platformLabel(selected.item)}</strong></div>
              <div><span>UPDATED</span><strong>{formatDate(selected.item.updatedAt)}</strong></div>
            </div>
            <div className="inspector-digest">
              <span className="material-symbols-outlined">verified_user</span>
              <span>SHA-256</span>
              <code>{digestLabel(selected.item)}</code>
            </div>
            <div className="inspector-actions">
              <InstallButton item={selected.item} variant="block" />
              <Link to={`/plugins/${encodeURIComponent(selected.item.id)}`}>详情</Link>
            </div>
          </>
        ) : (
          <div className="inspector-empty">
            <span className="material-symbols-outlined">travel_explore</span>
            <h2>{error ? '目录服务暂不可用' : loading ? '正在接收星图信号' : '当前星区没有匹配对象'}</h2>
            <p>{error ? '请稍后重试。' : '换一个关键词，或回到全部插件与分类星区。'}</p>
            {!loading && (query || activeCategory !== '全部') && (
              <button type="button" onClick={() => { onQueryChange(''); onCategoryChange('全部'); }}>
                清除筛选
              </button>
            )}
          </div>
        )}
      </aside>

      <div className="galaxy-coordinate-line" aria-hidden="true">
        <span ref={cursorRef}>CURSOR 0738 / 0423</span>
        <span ref={scaleRef}>SCALE 1.00×</span>
        <span ref={modeReadoutRef}>MODE ORBITAL</span>
      </div>

      <div className="galaxy-zoom" aria-label="星图缩放控制">
        <button type="button" aria-label="放大星图" onClick={() => {
          const rect = observatoryRef.current?.getBoundingClientRect();
          if (rect) beginZoom(rect.left + rect.width / 2, rect.top + rect.height / 2, 1.2);
        }}><span className="material-symbols-outlined">add</span></button>
        <button type="button" aria-label="缩小星图" onClick={() => {
          const rect = observatoryRef.current?.getBoundingClientRect();
          if (rect) beginZoom(rect.left + rect.width / 2, rect.top + rect.height / 2, 1 / 1.2);
        }}><span className="material-symbols-outlined">remove</span></button>
        <button type="button" aria-label="重置星图视角" onClick={focusHome}>
          <span className="material-symbols-outlined">restart_alt</span>
        </button>
      </div>

      <div className="galaxy-list-panel" aria-label="插件目录列表">
        <div className="galaxy-list-head">
          <div>
            <p>CATALOG INDEX / 目录视图</p>
            <h2>{activeCategory === '全部' ? '全部插件' : activeCategory}</h2>
          </div>
          <span>{visibleCount} 个对象</span>
        </div>
        <div className="galaxy-list">
          {visibleNodes.map((node) => (
            <button
              type="button"
              key={node.item.id}
              className={selected?.item.id === node.item.id ? 'is-selected' : ''}
              style={{ '--node-color': node.color, '--node-planet': `url(${node.planet})` } as CSSProperties}
              onClick={() => selectNode(node.item.id)}
            >
              <span className="list-sigil">{node.sigil}</span>
              <span className="list-copy">
                <b>{node.item.name}{node.item.official && <em>官方</em>}</b>
                <small>{node.item.description}</small>
              </span>
              <span className="list-meta">
                <b>{node.category} · v{node.item.version}</b>
                {node.item.id}
              </span>
              <span className="material-symbols-outlined">north_east</span>
            </button>
          ))}
          {visibleNodes.length === 0 && <p className="galaxy-list-empty">当前星区没有匹配对象。</p>}
        </div>
        {nextCursor && (
          <button type="button" className="galaxy-load-more" onClick={onLoadMore} disabled={loadingMore}>
            {loadingMore ? '正在扩展星图…' : '加载更多插件'}
          </button>
        )}
      </div>

      <footer className="galaxy-status">
        <div>
          <span className="galaxy-status-live">星图在线</span>
          <i />
          <span>显示 <b>{visibleCount}</b> / {items.length} 个对象</span>
          <i />
          <span>星区 / {activeCategory}</span>
        </div>
        <div className="galaxy-status-right">
          <span>拖拽后惯性滑行 · 滚轮对准缩放 · 点击飞近 · / 搜索</span>
          <span className="galaxy-credit">纹理 NASA · Solar System Scope</span>
          <div className="galaxy-view-toggle">
            <button type="button" className={view === 'map' ? 'is-active' : ''} onClick={() => setView('map')}>
              <span className="material-symbols-outlined">radar</span>星图
            </button>
            <button type="button" className={view === 'list' ? 'is-active' : ''} onClick={() => setView('list')}>
              <span className="material-symbols-outlined">format_list_bulleted</span>列表
            </button>
          </div>
        </div>
      </footer>

      {hasItems && visibleCount === 0 && (
        <div className="galaxy-empty" role="status">
          <strong>当前星区没有匹配对象</strong>
          <p>换一个关键词，或回到全部插件与分类星区。</p>
          <button type="button" onClick={() => { onQueryChange(''); onCategoryChange('全部'); }}>
            清除筛选
          </button>
        </div>
      )}
    </section>
  );
}
