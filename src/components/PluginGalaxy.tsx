import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties, type PointerEvent as ReactPointerEvent } from 'react';
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
  '/galaxy/copper-giant.png',
  '/galaxy/teal-ocean.png',
  '/galaxy/ice-world.png',
  '/galaxy/violet-world.png',
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
  orbit: 'inner' | 'outer';
  planet: string;
  color: string;
  sigil: string;
  labelSide: 'left' | 'right';
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

function layoutNodes(items: PluginItem[]): GalaxyNode[] {
  const sorted = [...items].sort((a, b) => {
    if (a.official !== b.official) return a.official ? -1 : 1;
    return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime();
  });
  const official = sorted.filter((item) => item.official);
  const community = sorted.filter((item) => !item.official);

  const inner = official.length > 0 ? official : sorted.slice(0, Math.min(4, sorted.length));
  const outer = official.length > 0 ? community : sorted.slice(inner.length);

  return sorted.map((item) => {
    const isInner = inner.includes(item);
    const ring = isInner ? inner : outer;
    const index = Math.max(0, ring.indexOf(item));
    const count = Math.max(1, ring.length);
    const baseAngle = isInner ? -Math.PI * 0.7 : Math.PI * 0.12;
    const angle = baseAngle + (Math.PI * 2 * index) / count + (stableFraction(item.id, 3) - 0.5) * 0.12;
    const orbit = isInner ? INNER_ORBIT : OUTER_ORBIT;
    const radiusJitter = 0.92 + stableFraction(item.id, 4) * 0.16;
    const x = WORLD.cx + Math.cos(angle) * orbit.rx * radiusJitter;
    const y = WORLD.cy + Math.sin(angle) * orbit.ry * radiusJitter;
    const category = categoryOf(item);
    return {
      item,
      category,
      x,
      y,
      orbit: isInner ? 'inner' : 'outer',
      planet: planetOf(item, index),
      color: item.official ? '#cf7542' : CATEGORY_COLORS[category],
      sigil: sigilOf(item),
      labelSide: x >= WORLD.cx ? 'right' : 'left',
    };
  });
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
  const zoomAtRef = useRef<(clientX: number, clientY: number, factor: number) => void>(() => {});
  const searchRef = useRef<HTMLInputElement>(null);
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
  const [camera, setCamera] = useState<Camera>(DEFAULT_CAMERA);
  const [dragging, setDragging] = useState(false);
  const [cursor, setCursor] = useState({ x: WORLD.cx, y: WORLD.cy });
  const [mapRectVersion, setMapRectVersion] = useState(0);
  const cameraRef = useRef(camera);
  const mapRectRef = useRef<DOMRect | null>(null);
  const dragRef = useRef({
    pointerId: -1,
    startX: 0,
    startY: 0,
    originX: 0,
    originY: 0,
    moved: false,
  });

  useEffect(() => {
    cameraRef.current = camera;
  }, [camera]);

  useEffect(() => {
    const element = observatoryRef.current;
    if (!element) return;
    mapRectRef.current = element.getBoundingClientRect();
    const update = () => {
      mapRectRef.current = element.getBoundingClientRect();
      setMapRectVersion((value) => value + 1);
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(element);
    window.addEventListener('resize', update);
    return () => {
      observer.disconnect();
      window.removeEventListener('resize', update);
    };
  }, []);

  const selectNode = useCallback((id: string) => {
    setSelectedId(id);
  }, []);

  const focusNode = useCallback((node: GalaxyNode, targetScale = 1.25) => {
    setCamera({
      scale: targetScale,
      x: 730 - node.x * targetScale,
      y: 430 - node.y * targetScale,
    });
  }, []);

  const projectedNodeStyle = useCallback((node: GalaxyNode): CSSProperties => {
    void mapRectVersion;
    const rect = mapRectRef.current;
    if (!rect) return { left: '50%', top: '50%', transform: 'translate(-50%, -50%)' };
    const canvasWidth = rect.width;
    const canvasHeight = rect.height;
    const viewScale = Math.max(WORLD.width / canvasWidth, WORLD.height / canvasHeight);
    const offsetX = (canvasWidth * viewScale - WORLD.width) / 2;
    const offsetY = (canvasHeight * viewScale - WORLD.height) / 2;
    const tx = camera.x + WORLD.cx * (1 - camera.scale);
    const ty = camera.y + WORLD.cy * (1 - camera.scale);
    const x = (tx + node.x * camera.scale + offsetX) / viewScale;
    const y = (ty + node.y * camera.scale + offsetY) / viewScale;
    return {
      left: `${x}px`,
      top: `${y}px`,
      '--node-color': node.color,
      '--node-planet': `url(${node.planet})`,
      '--node-scale': camera.scale,
    } as CSSProperties;
  }, [camera, mapRectVersion]);

  useEffect(() => {
    if (selected && view === 'map' && query.trim()) {
      focusNode(selected, 1.24);
    }
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
        setCamera(DEFAULT_CAMERA);
        return;
      }
      if (event.key === '0' && !typing) {
        setCamera(DEFAULT_CAMERA);
        return;
      }
      if (event.key === '+' || event.key === '=') {
        if (typing) return;
        setCamera((current) => ({ ...current, scale: Math.min(MAX_SCALE, current.scale * 1.18) }));
        return;
      }
      if (event.key === '-' && !typing) {
        setCamera((current) => ({ ...current, scale: Math.max(MIN_SCALE, current.scale / 1.18) }));
      }
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [onCategoryChange, onQueryChange]);

  const zoomAt = useCallback((clientX: number, clientY: number, factor: number) => {
    const rect = svgRef.current?.getBoundingClientRect();
    if (!rect) return;
    const point = clientPointToView(clientX, clientY, rect);
    setCamera((current) => {
      const scale = Math.max(MIN_SCALE, Math.min(MAX_SCALE, current.scale * factor));
      const applied = scale / current.scale;
      return {
        scale,
        x: point.x - WORLD.cx - (point.x - WORLD.cx - current.x) * applied,
        y: point.y - WORLD.cy - (point.y - WORLD.cy - current.y) * applied,
      };
    });
  }, []);

  useEffect(() => {
    zoomAtRef.current = zoomAt;
  }, [zoomAt]);

  useEffect(() => {
    const svg = svgRef.current;
    if (!svg) return;
    const handleWheel = (event: WheelEvent) => {
      event.preventDefault();
      const factor = Math.exp(-event.deltaY * 0.0017 * 2.6);
      zoomAtRef.current(event.clientX, event.clientY, factor);
    };
    svg.addEventListener('wheel', handleWheel, { passive: false });
    return () => svg.removeEventListener('wheel', handleWheel);
  }, []);

  const onPointerDown = (event: ReactPointerEvent<SVGSVGElement>) => {
    if (event.button !== 0) return;
    const target = event.target as Element;
    if (target.closest('[data-galaxy-node]')) return;
    dragRef.current = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      originX: cameraRef.current.x,
      originY: cameraRef.current.y,
      moved: false,
    };
    try {
      event.currentTarget.setPointerCapture(event.pointerId);
    } catch {
      // Synthetic pointers and some embedded WebViews do not support capture.
    }
    setDragging(true);
  };

  const onPointerMove = (event: ReactPointerEvent<SVGSVGElement>) => {
    const rect = event.currentTarget.getBoundingClientRect();
    const point = clientPointToView(event.clientX, event.clientY, rect);
    setCursor({ x: point.x, y: point.y });
    const drag = dragRef.current;
    if (drag.pointerId !== event.pointerId || !dragging) return;
    const viewScale = Math.max(WORLD.width / rect.width, WORLD.height / rect.height);
    const dx = (event.clientX - drag.startX) * viewScale;
    const dy = (event.clientY - drag.startY) * viewScale;
    if (Math.hypot(dx, dy) > 5) drag.moved = true;
    setCamera((current) => ({
      ...current,
      x: drag.originX + dx,
      y: drag.originY + dy,
    }));
  };

  const endPointer = (event: ReactPointerEvent<SVGSVGElement>) => {
    if (dragRef.current.pointerId === event.pointerId) {
      dragRef.current.pointerId = -1;
      setDragging(false);
      if (event.currentTarget.hasPointerCapture(event.pointerId)) {
        event.currentTarget.releasePointerCapture(event.pointerId);
      }
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
    >
      <div className="galaxy-backdrop" aria-hidden="true">
        <div className="galaxy-grid" />
        <div className="galaxy-stars" />
        <div className="galaxy-grain" />
      </div>

      <svg
        ref={svgRef}
        className={`galaxy-map ${dragging ? 'is-dragging' : ''}`}
        viewBox={`0 0 ${WORLD.width} ${WORLD.height}`}
        preserveAspectRatio="xMidYMid slice"
        role="application"
        aria-label="可缩放、可拖拽的插件星图"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endPointer}
        onPointerCancel={endPointer}
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
          <filter id="galaxyPlanetShadow">
            <feDropShadow dx="3" dy="5" stdDeviation="4" floodColor="#000" floodOpacity="0.78" />
          </filter>
        </defs>

        <g
          className="galaxy-world"
          data-zoom-level={zoomLevel(camera.scale)}
          transform={cameraTransform(camera)}
        >
          <circle cx={WORLD.cx} cy={WORLD.cy} r="525" fill="url(#galaxyHalo)" />
          <g className="galaxy-axis" aria-hidden="true">
            <line x1="170" y1={WORLD.cy} x2="1280" y2={WORLD.cy} />
            <line x1={WORLD.cx} y1="70" x2={WORLD.cx} y2="800" />
            <ellipse cx={WORLD.cx} cy={WORLD.cy} rx={INNER_ORBIT.rx} ry={INNER_ORBIT.ry} transform={`rotate(-11 ${WORLD.cx} ${WORLD.cy})`} />
            <ellipse className="is-dashed" cx={WORLD.cx} cy={WORLD.cy} rx="294" ry="186" transform={`rotate(-11 ${WORLD.cx} ${WORLD.cy})`} />
            <ellipse cx={WORLD.cx} cy={WORLD.cy} rx={OUTER_ORBIT.rx} ry={OUTER_ORBIT.ry} transform={`rotate(-11 ${WORLD.cx} ${WORLD.cy})`} />
            <text x="728" y="112">NORTH / DATASOURCE</text>
            <text x="1030" y="438">EAST / AI PROVIDER</text>
            <text x="735" y="746">SOUTH / SKILL</text>
            <text x="322" y="430" textAnchor="end">WEST / OPS</text>
          </g>

          <g className="galaxy-core" transform={`translate(${WORLD.cx} ${WORLD.cy})`}>
            <circle className="core-halo" r="84" fill="url(#galaxySweep)" />
            <circle className="core-ring" r="63" />
            <circle className="core-ring reverse" r="47" />
            <circle className="core-body" r="29" />
            <circle className="core-inner" r="24" fill="url(#galaxyCore)" />
            <text y="4">GN</text>
            <text className="core-caption" y="48">GONAVI CORE</text>
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
              type="button"
              className={`galaxy-dom-node ${selected?.item.id === node.item.id ? 'is-selected' : ''} ${visible ? '' : 'is-filtered'} ${node.labelSide === 'left' ? 'label-left' : ''}`}
              style={projectedNodeStyle(node)}
              aria-label={`${node.item.name}，${node.item.official ? '官方' : '社区'}${node.category}插件`}
              onClick={() => {
                if (dragRef.current.moved) {
                  dragRef.current.moved = false;
                  return;
                }
                selectNode(node.item.id);
              }}
            >
              <span className="dom-node-orbit" aria-hidden="true" />
              <span className="dom-node-aura" aria-hidden="true" />
              <span className="dom-node-ring" aria-hidden="true" />
              <img className="dom-node-planet" src={node.planet} alt="" draggable={false} />
              <span className="dom-node-moon" aria-hidden="true" />
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
                  if (first) focusNode(first, category === '全部' ? 1.08 : 1.28);
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
        <span>CURSOR {String(Math.round(cursor.x)).padStart(4, '0')} / {String(Math.round(cursor.y)).padStart(4, '0')}</span>
        <span>SCALE {camera.scale.toFixed(2)}×</span>
        <span>MODE ORBITAL</span>
      </div>

      <div className="galaxy-zoom" aria-label="星图缩放控制">
        <button type="button" aria-label="放大星图" onClick={() => {
          const rect = observatoryRef.current?.getBoundingClientRect();
          if (rect) zoomAt(rect.left + rect.width / 2, rect.top + rect.height / 2, 1.24);
        }}><span className="material-symbols-outlined">add</span></button>
        <button type="button" aria-label="缩小星图" onClick={() => {
          const rect = observatoryRef.current?.getBoundingClientRect();
          if (rect) zoomAt(rect.left + rect.width / 2, rect.top + rect.height / 2, 1 / 1.24);
        }}><span className="material-symbols-outlined">remove</span></button>
        <button type="button" aria-label="重置星图视角" onClick={() => setCamera(DEFAULT_CAMERA)}>
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
          <span>拖拽平移 · 滚轮缩放 · / 搜索</span>
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
