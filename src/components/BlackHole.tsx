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

const BASE_SIZE = 480;

const vertexShader = /* glsl */ `
  varying vec2 vUv;
  void main() {
    vUv = uv;
    gl_Position = vec4(position.xy, 0.0, 1.0);
  }
`;

const fragmentShader = /* glsl */ `
  precision highp float;
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
    for (int i = 0; i < 5; i++) {
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

    // Thin, tilted accretion disk. The extra vertical scale turns the ring
    // into the flattened ellipse seen from a low inclination.
    vec2 diskP = rotate2d(-0.24) * p;
    diskP.y *= 2.15;
    float diskRadius = length(diskP);
    float diskAngle = atan(diskP.y, diskP.x);
    float inner = 0.32;
    float outer = 1.02;
    float band = smoothstep(inner, inner + 0.045, diskRadius) *
                 (1.0 - smoothstep(outer - 0.19, outer, diskRadius));

    // Turbulent spiral structure; the phase moves so the disk visibly flows.
    float spiral = diskAngle * 2.6 + time * 0.34 + log(max(diskRadius, 0.001)) * 4.2;
    float turbulence = fbm(vec2(spiral, diskRadius * 9.0 - time * 0.18));
    float filaments = fbm(vec2(spiral * 2.2 + 13.0, diskRadius * 18.0 + time * 0.08));
    float diskTexture = 0.38 + 0.68 * turbulence + 0.24 * filaments;

    // Relativistic beaming: the approaching side is brighter and whiter.
    float doppler = 0.60 + 0.62 * pow(max(0.0, cos(diskAngle + 0.72)), 2.8);
    float dust = smoothstep(0.54, 0.78, fbm(vec2(spiral * 0.8 + 4.0, diskRadius * 5.0 - time * 0.06)));
    float disk = band * diskTexture * doppler * (1.0 - dust * 0.42);

    // Hot inner rim and the thin photon ring around the shadow.
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

    // Event horizon: an opaque shadow, with a fine warm rim outside it.
    float shadow = 1.0 - smoothstep(0.185, 0.199, radius);
    float rim = exp(-pow((radius - 0.203) * 82.0, 2.0));
    color = mix(color, vec3(0.0), shadow);
    alpha = max(alpha, shadow);
    alpha = max(alpha, rim * 0.22);
    color += whiteHot * rim * 0.25;

    // The near side of the disk passes in front of the horizon; without this
    // the hole reads as a flat black sticker with a halo behind it.
    float frontBand = smoothstep(0.12, 0.17, diskRadius) * (1.0 - smoothstep(0.92, 1.04, diskRadius));
    float frontTexture = 0.52 + 0.48 * fbm(vec2(spiral * 1.3, diskRadius * 12.0 - time * 0.12));
    float frontDisk = frontBand * smoothstep(-0.025, 0.065, p.y) * frontTexture * 0.72;
    color = mix(color, diskColor, clamp(frontDisk, 0.0, 0.72));
    alpha = max(alpha, frontDisk);

    // A second, thin upper image of the far side; this is the lensed arc that
    // makes the disk wrap over the shadow instead of ending at its edge.
    float upperArc = exp(-pow((diskRadius - 0.255 + 0.018 * cos(angle * 3.0)) * 18.0, 2.0)) *
                     smoothstep(0.02, 0.15, -p.y) *
                     smoothstep(0.22, 0.32, radius) *
                     (0.35 + 0.65 * fbm(vec2(spiral * 0.7, radius * 7.0 - time * 0.08)));
    color += copper * upperArc * 0.38;
    alpha = max(alpha, upperArc * 0.34);

    // Sparse lensed star glints around the Einstein ring.
    float starCell = hash21(floor(vec2(angle * 28.0, radius * 34.0)));
    float glint = step(0.992, starCell) * exp(-pow((radius - 0.46) * 9.0, 2.0));
    color += whiteHot * glint * 0.45;
    alpha = max(alpha, glint * 0.35);

    // Soft outer corona so the black hole sits naturally on the star field.
    float corona = exp(-pow((radius - 0.42) * 3.4, 2.0)) * 0.055;
    alpha = max(alpha, corona);
    color += copper * corona * 0.5;

    // Alpha edges and a tiny grain keep the disk from looking like a flat ring.
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

    const scene = new Scene();
    const camera = new OrthographicCamera(-1, 1, 1, -1, 0, 1);
    const geometry = new PlaneGeometry(2, 2);
    const material = new ShaderMaterial({
      transparent: true,
      depthWrite: false,
      uniforms: {
        uTime: { value: 0 },
        uResolution: { value: new Vector2(BASE_SIZE, BASE_SIZE) },
      },
      vertexShader,
      fragmentShader,
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
