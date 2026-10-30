import { useEffect, useRef, type RefObject } from 'react';
import {
  DataTexture,
  Mesh,
  OrthographicCamera,
  PlaneGeometry,
  Scene,
  ShaderMaterial,
  SRGBColorSpace,
  Vector2,
  Vector3,
  WebGLRenderer,
} from 'three';
import { blackHoleFragmentShader } from './blackHoleShader';

const BASE_SIZE = 480;

const vertexShader = /* glsl */ `
  varying vec2 vUv;
  void main() {
    vUv = uv;
    gl_Position = vec4(position.xy, 0.0, 1.0);
  }
`;

export interface BlackHoleProps {
  hostRef: RefObject<HTMLElement | null>;
  worldRef: RefObject<SVGGElement | null>;
  centerX: number;
  centerY: number;
}

/**
 * A WebGL black hole rendered over the SVG star map. Its CSS transform is
 * synced to the map camera every frame, so pan/zoom carry it with the world
 * without making plugin nodes depend on WebGL.
 */
export function BlackHole({ hostRef, worldRef, centerX, centerY }: BlackHoleProps) {
  const frameRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const overlay = frameRef.current;
    const host = hostRef.current;
    if (!overlay || !host) return;

    let renderer: WebGLRenderer;
    try {
      renderer = new WebGLRenderer({ alpha: true, antialias: true, powerPreference: 'low-power' });
    } catch {
      overlay.classList.add('is-webgl-unavailable');
      return;
    }

    const transparentTexture = new DataTexture(new Uint8Array([0, 0, 0, 0]), 1, 1);
    transparentTexture.needsUpdate = true;
    const scene = new Scene();
    const camera = new OrthographicCamera(-1, 1, 1, -1, 0, 1);
    const geometry = new PlaneGeometry(2, 2);
    const material = new ShaderMaterial({
      transparent: true,
      depthWrite: false,
      uniforms: {
        uTime: { value: 0 },
        uResolution: { value: new Vector2(BASE_SIZE, BASE_SIZE) },
        uAccretionDisk: { value: 1.0 },
        uCanvasTexture: { value: transparentTexture },
        uCameraTranslate: { value: new Vector3(0, 0, 0) },
        uPov: { value: 75.0 },
        uMaxIterations: { value: 220 },
        uStepSize: { value: 2.5 / 220 },
      },
      vertexShader,
      fragmentShader: blackHoleFragmentShader,
    });
    const mesh = new Mesh(geometry, material);
    scene.add(mesh);
    renderer.outputColorSpace = SRGBColorSpace;
    renderer.setClearColor(0x000000, 0);
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
    renderer.setSize(BASE_SIZE, BASE_SIZE, false);
    overlay.prepend(renderer.domElement);
    renderer.domElement.className = 'galaxy-black-hole-canvas';

    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)');
    let frame = 0;
    const started = performance.now();
    let visible = true;

    const syncPosition = () => {
      const world = worldRef.current;
      if (!world || !host) return;
      const matrix = world.getScreenCTM();
      if (!matrix) return;
      const point = new DOMPoint(centerX, centerY).matrixTransform(matrix);
      const rect = host.getBoundingClientRect();
      const scale = Math.max(0.35, Math.min(2.6, matrix.a));
      overlay.style.transform = `translate3d(${(point.x - rect.left).toFixed(2)}px, ${(point.y - rect.top).toFixed(2)}px, 0) translate(-50%, -50%) scale(${scale.toFixed(4)})`;
    };

    const render = (now: number) => {
      if (!visible) {
        frame = requestAnimationFrame(render);
        return;
      }
      const elapsed = reduced.matches ? 8.0 : (now - started) / 1000;
      material.uniforms.uTime.value = elapsed;
      material.uniforms.uCameraTranslate.value.set(Math.sin(elapsed * 0.045) * 0.35, 0, 0);
      syncPosition();
      renderer.render(scene, camera);
      frame = requestAnimationFrame(render);
    };

    const onVisibility = () => {
      visible = document.visibilityState === 'visible';
    };
    document.addEventListener('visibilitychange', onVisibility);
    render(performance.now());

    return () => {
      cancelAnimationFrame(frame);
      document.removeEventListener('visibilitychange', onVisibility);
      transparentTexture.dispose();
      geometry.dispose();
      material.dispose();
      renderer.dispose();
      renderer.domElement.remove();
    };
  }, [centerX, centerY, hostRef, worldRef]);

  return (
    <div ref={frameRef} className="galaxy-black-hole" aria-hidden="true">
      <span className="galaxy-black-hole-caption">GONAVI</span>
    </div>
  );
}
