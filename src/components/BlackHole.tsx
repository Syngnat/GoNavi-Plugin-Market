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
    for (int i = 0; i < 4; i++) {
      value += amplitude * noise(p);
      p = rotate2d(0.63) * p * 2.03 + 17.1;
      amplitude *= 0.52;
    }
    return value;
  }

  vec3 diskSample(vec3 q, vec3 rayDir, float time) {
    float r = length(q.xz);
    float theta = atan(q.z, q.x);
    float spin = theta * 3.1 + log(max(r, 0.01)) * 5.4 + time * 0.38;
    float turbulence = fbm(vec2(spin, r * 5.2 - time * 0.14));
    float filaments = fbm(vec2(spin * 2.4 + 8.0, r * 11.0 + time * 0.06));
    float texture = 0.34 + 0.64 * turbulence + 0.23 * filaments;

    // Temperature falls outward; the inner edge is nearly white-hot.
    float heat = pow(clamp(1.0 - (r - 3.0) / 6.0, 0.0, 1.0), 2.15);
    vec3 ember = vec3(0.28, 0.032, 0.006);
    vec3 copper = vec3(1.0, 0.27, 0.028);
    vec3 whiteHot = vec3(1.0, 0.93, 0.76);
    vec3 color = mix(ember, copper, smoothstep(0.0, 0.70, heat));
    color = mix(color, whiteHot, smoothstep(0.62, 1.0, heat));

    // Relativistic beaming: the approaching side is brighter and bluer-white.
    vec3 tangent = normalize(vec3(-q.z, 0.0, q.x));
    float mu = clamp(dot(normalize(rayDir), tangent), -1.0, 1.0);
    float boost = clamp(pow(1.0 / max(0.28, 1.0 - 0.48 * mu), 2.2), 0.35, 3.4);
    color = mix(color, whiteHot, min(0.45, (boost - 1.0) * 0.22));
    color *= boost;

    float band = smoothstep(3.0, 3.55, r) * (1.0 - smoothstep(8.15, 9.0, r));
    float alpha = band * (0.44 + 0.40 * texture);
    // A narrow dark lane and bright rim prevent the disk from reading as a solid blob.
    alpha *= 1.0 - 0.55 * smoothstep(0.58, 0.82, fbm(vec2(spin * 1.4, r * 7.0)));
    return vec4(color, clamp(alpha, 0.0, 0.92));
  }

  void main() {
    vec2 screen = vUv * 2.0 - 1.0;
    screen.x *= uResolution.x / max(uResolution.y, 1.0);

    vec3 ro = vec3(0.0, 0.72, 14.0);
    vec3 target = vec3(0.0);
    vec3 forward = normalize(target - ro);
    vec3 right = normalize(cross(vec3(0.0, 1.0, 0.0), forward));
    vec3 up = cross(forward, right);
    vec3 rd = normalize(screen.x * right + screen.y * up + 1.2 * forward);

    vec3 p = ro;
    vec3 v = rd;
    vec3 angularMomentum = cross(p, v);
    float h2 = dot(angularMomentum, angularMomentum);
    bool captured = false;
    float diskDistance = 1e9;
    vec3 diskColor = vec3(0.0);
    float diskAlpha = 0.0;

    // Integrate a null geodesic in the Schwarzschild metric. The acceleration
    // term bends the ray around the mass; the first disk-plane crossing is the
    // visible front of the accretion disk, while later crossings form the
    // lensed arcs above and below the shadow.
    for (int i = 0; i < 128; i++) {
      float r = length(p);
      if (r < 1.0) {
        captured = true;
        break;
      }
      if (r > 26.0 && dot(p, v) > 0.0) {
        break;
      }

      float dt = 0.075 + min(r, 5.0) * 0.012;
      vec3 nextP = p + v * dt;

      if (p.y * nextP.y < 0.0) {
        float t = -p.y / v.y;
        if (t > 0.0 && t < dt) {
          vec3 hit = p + v * t;
          float radius = length(hit.xz);
          if (radius > 2.85 && radius < 9.0 && t < diskDistance) {
            vec4 sampleDisk = diskSample(hit, v, uTime);
            diskDistance = t;
            diskColor = sampleDisk.rgb;
            diskAlpha = sampleDisk.a;
          }
        }
      }

      vec3 accel = -1.5 * h2 * p / pow(max(dot(p, p), 0.0001), 2.5);
      v += accel * dt;
      p = nextP;
    }

    vec3 color = vec3(0.0);
    float alpha = 0.0;
    if (diskDistance < 1e8) {
      color = diskColor;
      alpha = diskAlpha;
    }
    if (captured) {
      alpha = 1.0;
      color = mix(vec3(0.0), color, diskAlpha);
    }

    // A restrained halo around the shadow, not a painted ring.
    float screenRadius = length(screen);
    float halo = exp(-pow((screenRadius - 0.205) * 18.0, 2.0)) * 0.10;
    alpha = max(alpha, halo * 0.45);
    color += vec3(1.0, 0.42, 0.10) * halo * 0.45;

    alpha = clamp(alpha, 0.0, 1.0);
    color *= 1.0 + (hash21(gl_FragCoord.xy + uTime) - 0.5) * 0.03;
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
