import { useEffect, useRef, type RefObject } from 'react';

export interface BlackHoleProps {
  hostRef: RefObject<HTMLElement | null>;
  worldRef: RefObject<SVGGElement | null>;
  centerX: number;
  centerY: number;
}

/**
 * The black hole is a pre-rendered transparent frame derived from the MIT
 * Gargantua raytracer (github.com/Sun-sh902/Gargantua), composited offline. Keeping it as an image
 * avoids a permanent WebGL context and shader compilation on the star map,
 * while the SVG camera still moves it with the world.
 */
export function BlackHole({ hostRef, worldRef, centerX, centerY }: BlackHoleProps) {
  const frameRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const overlay = frameRef.current;
    const host = hostRef.current;
    if (!overlay || !host) return;

    let lastPlacement = '';
    const syncPosition = (force = false) => {
      const world = worldRef.current;
      if (!world) return;
      const matrix = world.getScreenCTM();
      if (!matrix) return;
      const point = new DOMPoint(centerX, centerY).matrixTransform(matrix);
      const rect = host.getBoundingClientRect();
      const scale = Math.max(0.35, Math.min(2.6, matrix.a));
      const placement = `${(point.x - rect.left).toFixed(1)}:${(point.y - rect.top).toFixed(1)}:${scale.toFixed(3)}`;
      if (!force && placement === lastPlacement) return;
      lastPlacement = placement;
      overlay.style.transform = `translate3d(${(point.x - rect.left).toFixed(2)}px, ${(point.y - rect.top).toFixed(2)}px, 0) translate(-50%, -50%) scale(${scale.toFixed(4)})`;
    };

    const cameraObserver = new MutationObserver(() => syncPosition());
    if (worldRef.current) {
      cameraObserver.observe(worldRef.current, { attributes: true, attributeFilter: ['transform'] });
    }
    syncPosition(true);

    return () => cameraObserver.disconnect();
  }, [centerX, centerY, hostRef, worldRef]);

  return (
    <div ref={frameRef} className="galaxy-black-hole" aria-hidden="true">
      <img
        className="galaxy-black-hole-canvas"
        src="/galaxy/black-hole.webp"
        width={640}
        height={360}
        alt=""
        decoding="async"
        draggable={false}
      />
      <span className="galaxy-black-hole-caption">GONAVI</span>
    </div>
  );
}
