import { useEffect, useRef, type RefObject } from 'react';
import {
  Mesh,
  OrthographicCamera,
  PlaneGeometry,
  Scene,
  ShaderMaterial,
  SRGBColorSpace,
  Vector2,
  WebGLRenderer,
} from 'three';

const RENDER_SIZE = 512;

const vertexShader = /* glsl */ `
  varying vec2 vUv;
  void main() {
    vUv = uv;
    gl_Position = vec4(position.xy, 0.0, 1.0);
  }
`;

const fragmentShader = /* glsl */ `
  precision mediump float;
  uniform float uTime;
  uniform vec2 uResolution;
  varying vec2 vUv;

  mat2 rotate2d(float a) {
    float s = sin(a), c = cos(a);
    return mat2(c, -s, s, c);
  }

  float hash21(vec2 p) {
    p = fract(p * vec2(123.34, 456.21));
    p += dot(p, p + 45.32);
    return fract(p.x * p.y);
  }

  float noise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    return mix(
      mix(hash21(i), hash21(i + vec2(1.0, 0.0)), f.x),
      mix(hash21(i + vec2(0.0, 1.0)), hash21(i + vec2(1.0, 1.0)), f.x),
      f.y
    );
  }

  float fbm(vec2 p) {
    float value = 0.0;
    float amplitude = 0.5;
    for (int i = 0; i < 3; i++) {
      value += amplitude * noise(p);
      p = rotate2d(0.63) * p * 2.03 + 17.1;
      amplitude *= 0.52;
    }
    return value;
  }

  void main() {
    vec2 p = vUv * 2.0 - 1.0;
    p.x *= uResolution.x / max(uResolution.y, 1.0);
    float time = uTime;
    float radius = length(p);
    float angle = atan(p.y, p.x);

    vec2 diskP = rotate2d(-0.24) * p;
    diskP.y *= 2.15;
    float diskRadius = length(diskP);
    float diskAngle = atan(diskP.y, diskP.x);
    float inner = 0.32;
    float outer = 1.02;
    float band = smoothstep(inner, inner + 0.045, diskRadius) *
                 (1.0 - smoothstep(outer - 0.19, outer, diskRadius));

    float spiral = diskAngle * 2.6 + time * 0.34 + log(max(diskRadius, 0.001)) * 4.2;
    float turbulence = fbm(vec2(spiral, diskRadius * 9.0 - time * 0.18));
    float diskTexture = 0.42 + 0.82 * turbulence;
    float doppler = 0.60 + 0.62 * pow(max(0.0, cos(diskAngle + 0.72)), 2.8);
    float dust = smoothstep(0.54, 0.78, fbm(vec2(spiral * 0.8 + 4.0, diskRadius * 5.0 - time * 0.06)));
    float disk = band * diskTexture * doppler * (1.0 - dust * 0.42);

    float innerRim = exp(-pow((diskRadius - inner) * 34.0, 2.0)) * doppler;
    float ringWarp = 0.0045 * sin(angle * 7.0 + time * 0.16) + 0.0025 * sin(angle * 13.0 - time * 0.11);
    float photonRing = exp(-pow((radius - (0.208 + ringWarp)) * 118.0, 2.0));
    float photonHalo = exp(-pow((radius - 0.224) * 16.0, 2.0)) * 0.11;
    float lensedArc = exp(-pow((radius - 0.315 + 0.018 * cos(angle * 3.0)) * 13.0, 2.0)) *
                      smoothstep(0.02, 0.16, -p.y) *
                      (0.42 + 0.58 * fbm(vec2(angle * 4.0 + time * 0.18, radius * 9.0)));

    vec3 ember = vec3(0.34, 0.045, 0.008);
    vec3 copper = vec3(1.0, 0.28, 0.032);
    vec3 whiteHot = vec3(1.0, 0.92, 0.74);
    float heat = smoothstep(outer, inner, diskRadius);
    vec3 diskColor = mix(ember, copper, heat);
    diskColor = mix(diskColor, whiteHot, pow(1.0 - smoothstep(inner, 0.52, diskRadius), 3.0));

    float alpha = max(disk * 0.66, innerRim * 0.48);
    alpha = max(alpha, photonRing * 0.46 + photonHalo * 0.22);
    alpha = max(alpha, lensedArc * 0.50);
    vec3 color = diskColor * max(disk, innerRim * 1.05);
    color += whiteHot * (photonRing * 0.72 + photonHalo * 0.22);
    color += copper * lensedArc * 0.50;

    float shadow = 1.0 - smoothstep(0.185, 0.199, radius);
    float rim = exp(-pow((radius - 0.203) * 82.0, 2.0));
    color = mix(color, vec3(0.0), shadow);
    alpha = max(alpha, shadow);
    alpha = max(alpha, rim * 0.22);
    color += whiteHot * rim * 0.25;

    float frontSide = smoothstep(-0.018, 0.055, p.y);
    float frontDisk = disk * frontSide * 0.78;
    color = mix(color, diskColor, clamp(frontDisk, 0.0, 0.82));
    alpha = max(alpha, frontDisk);

    float corona = exp(-pow((radius - 0.42) * 3.4, 2.0)) * 0.055;
    alpha = max(alpha, corona);
    color += copper * corona * 0.5;

    alpha *= smoothstep(1.06, 0.74, radius) + shadow + rim;
    alpha = clamp(alpha, 0.0, 1.0);
    color *= 1.0 + (hash21(gl_FragCoord.xy + time) - 0.5) * 0.035;
    gl_FragColor = vec4(color, alpha);
  }
`;

export interface BlackHoleProps {
  hostRef: RefObject<HTMLElement | null>;
  worldRef: RefObject<SVGGElement | null>;
  centerX: number;
  centerY: number;
}

/**
 * Renders one high-quality frame to a transparent PNG and disposes WebGL.
 * The star map keeps its camera transform; the static layer only follows it
 * when the SVG camera actually changes. This avoids a continuous GPU/CPU load.
 */
export function BlackHole({ hostRef, worldRef, centerX, centerY }: BlackHoleProps) {
  const frameRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const overlay = frameRef.current;
    const host = hostRef.current;
    if (!overlay || !host) return;

    let renderer: WebGLRenderer;
    try {
      renderer = new WebGLRenderer({
        alpha: true,
        antialias: true,
        powerPreference: 'high-performance',
        preserveDrawingBuffer: true,
      });
    } catch {
      overlay.classList.add('is-webgl-unavailable');
      return;
    }

    const scene = new Scene();
    const camera = new OrthographicCamera(-1, 1, 1, -1, 0, 1);
    const geometry = new PlaneGeometry(2, 2);
    const material = new ShaderMaterial({
      transparent: true,
      depthWrite: false,
      uniforms: {
        uTime: { value: 8 },
        uResolution: { value: new Vector2(RENDER_SIZE, RENDER_SIZE) },
      },
      vertexShader,
      fragmentShader,
    });
    scene.add(new Mesh(geometry, material));
    renderer.outputColorSpace = SRGBColorSpace;
    renderer.setClearColor(0x000000, 0);
    renderer.setPixelRatio(1);
    renderer.setSize(RENDER_SIZE, RENDER_SIZE, false);
    renderer.render(scene, camera);

    const snapshot = renderer.domElement.toDataURL('image/png');
    const image = document.createElement('img');
    image.className = 'galaxy-black-hole-canvas';
    image.alt = '';
    image.decoding = 'async';
    image.src = snapshot;
    overlay.prepend(image);

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

    return () => {
      cameraObserver.disconnect();
      image.remove();
      geometry.dispose();
      material.dispose();
      renderer.dispose();
    };
  }, [centerX, centerY, hostRef, worldRef]);

  return (
    <div ref={frameRef} className="galaxy-black-hole" aria-hidden="true">
      <span className="galaxy-black-hole-caption">GONAVI</span>
    </div>
  );
}
