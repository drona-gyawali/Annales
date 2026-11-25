import type { ReactNode } from "react";

type PacketProps = {
  path: string;
  color: string;
  label: string;
  /** Total animation cycle in seconds. */
  cycle: number;
  /** Packet appearance in seconds, relative to the start of the cycle. */
  start: number;
  /** Packet arrival in seconds, relative to the start of the cycle. */
  end: number;
  /** Optional delay before the first packet leaves. */
  begin?: number;
  square?: boolean;
  labelBelow?: boolean;
};

/**
 * A small payload that follows an SVG path using SMIL. The animation is
 * declarative and browser-driven — there is no JavaScript requestAnimationFrame
 * loop. Only rendered when motion is allowed.
 */
export function Packet({
  path,
  color,
  label,
  cycle,
  start,
  end,
  begin = 0,
  square = false,
  labelBelow = false,
}: PacketProps) {
  const first = clamp(start / cycle);
  const last = clamp(end / cycle);
  const fadeIn = clamp(first + 0.012);
  const fadeOut = clamp(last + 0.018);
  const opacityTimes = uniqueSorted([0, first, fadeIn, last, fadeOut, 1]);
  const opacityValues = opacityTimes
    .map((time) => (time >= fadeIn && time <= last ? 1 : 0))
    .join(";");

  return (
    <g opacity={0}>
      <animate
        attributeName="opacity"
        dur={`${cycle}s`}
        begin={`${begin}s`}
        repeatCount="indefinite"
        keyTimes={opacityTimes.join(";")}
        values={opacityValues}
      />
      <g>
        <animateMotion
          dur={`${cycle}s`}
          begin={`${begin}s`}
          repeatCount="indefinite"
          path={path}
          keyPoints="0;0;1;1"
          keyTimes={`0;${first};${last};1`}
          calcMode="linear"
        />
        {square ? (
          <rect
            x={-2.4}
            y={-2.4}
            width={4.8}
            height={4.8}
            fill={color}
            stroke="var(--figure, #fff)"
            strokeWidth={0.7}
          />
        ) : (
          <circle r={2.5} fill={color} stroke="var(--figure, #fff)" strokeWidth={0.7} />
        )}
        <text
          x={6}
          y={labelBelow ? 8 : 2.2}
          fontSize={5.6}
          letterSpacing="0.08em"
          fill={color}
          stroke="var(--figure, #fff)"
          strokeWidth={1.6}
          paintOrder="stroke"
          style={{ fontFamily: "var(--font-geist-mono), monospace" }}
        >
          {label}
        </text>
      </g>
    </g>
  );
}

type ArrowMarkerProps = {
  id: string;
  color: string;
};

export function ArrowMarker({ id, color }: ArrowMarkerProps) {
  return (
    <marker
      id={id}
      viewBox="0 0 7 7"
      refX="6"
      refY="3.5"
      markerWidth="5.5"
      markerHeight="5.5"
      orient="auto-start-reverse"
    >
      <path d="M0.5 0.9 L6 3.5 L0.5 6.1" fill="none" stroke={color} strokeWidth="0.8" />
    </marker>
  );
}

export function CrossMark({ x, y, size = 4 }: { x: number; y: number; size?: number }) {
  return (
    <g stroke="var(--dia-faint)" strokeWidth={0.6} aria-hidden="true">
      <path d={`M${x - size} ${y}H${x + size}M${x} ${y - size}V${y + size}`} />
    </g>
  );
}

export function SmallArrow({ direction = "right" }: { direction?: "right" | "down" }) {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 12 12"
      fill="none"
      aria-hidden="true"
      style={direction === "down" ? { transform: "rotate(90deg)" } : undefined}
    >
      <path d="M1.5 6h9M6.5 2l4 4-4 4" stroke="currentColor" strokeWidth="1.1" />
    </svg>
  );
}

export function RefreshGlyph({ color = "currentColor" }: { color?: string }) {
  return (
    <g fill="none" stroke={color} strokeWidth="1" strokeLinecap="round">
      <path d="M-5.2 -1.1a5.4 5.4 0 0 1 9.2 -2.2" />
      <path d="M4.6 -4.3l-0.8 2.6-2.6-0.8" />
      <path d="M5.2 1.1a5.4 5.4 0 0 1 -9.2 2.2" />
      <path d="M-4.6 4.3l0.8 -2.6 2.6 0.8" />
    </g>
  );
}

export function FigureLayers({ children }: { children: ReactNode }) {
  return <>{children}</>;
}

function clamp(value: number) {
  return Math.min(1, Math.max(0, value));
}

function uniqueSorted(values: number[]) {
  return [...new Set(values.map((value) => Number(clamp(value).toFixed(4))))].sort(
    (a, b) => a - b,
  );
}
