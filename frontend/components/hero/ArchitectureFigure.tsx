"use client";

import { useCallback, useEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from "react";
import { MODES, MODE_BY_ID, type Mode } from "@/lib/modes";
import { useReducedMotion } from "@/lib/useReducedMotion";
import { CompactDiagram } from "./architecture/CompactDiagram";
import { DesktopDiagram } from "./architecture/DesktopDiagram";

const CYCLE_MS = 12_000;

export function ArchitectureFigure() {
  const [mode, setMode] = useState<Mode>("read");
  const [paused, setPaused] = useState(false);
  const { reduced, ready } = useReducedMotion();
  const figureRef = useRef<HTMLElement>(null);
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const timelineMode = useRef<Mode | null>(null);

  const motionEnabled = ready && !reduced;

  useEffect(() => {
    if (!ready || reduced || paused) return;
    const timer = window.setInterval(() => {
      setMode((current) => {
        const index = MODES.findIndex((item) => item.id === current);
        return MODES[(index + 1) % MODES.length].id;
      });
    }, CYCLE_MS);
    return () => window.clearInterval(timer);
  }, [ready, reduced, paused, mode]);

  // SMIL timelines are reset when the reader selects a new flow, then paused
  // through the SVG API when they pause the figure. CSS animations restart from
  // their selectors and are paused by the data-paused attribute in CSS.
  useEffect(() => {
    const svgs = figureRef.current?.querySelectorAll("svg");
    if (!svgs) return;
    svgs.forEach((svg) => {
      try {
        if (timelineMode.current !== mode) {
          svg.setCurrentTime(0);
        }
        if (paused || !motionEnabled) svg.pauseAnimations();
        else svg.unpauseAnimations();
      } catch {
        // SMIL timeline controls are not implemented everywhere. The figure
        // remains fully usable when the browser declines.
      }
    });
    timelineMode.current = mode;
  }, [paused, mode, motionEnabled]);

  const selectTab = useCallback((next: Mode) => {
    setMode(next);
  }, []);

  const handleTabKeyDown = useCallback(
    (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
      let nextIndex: number | null = null;
      if (event.key === "ArrowRight" || event.key === "ArrowDown") {
        nextIndex = (index + 1) % MODES.length;
      } else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
        nextIndex = (index - 1 + MODES.length) % MODES.length;
      } else if (event.key === "Home") {
        nextIndex = 0;
      } else if (event.key === "End") {
        nextIndex = MODES.length - 1;
      }
      if (nextIndex === null) return;
      event.preventDefault();
      selectTab(MODES[nextIndex].id);
      tabRefs.current[nextIndex]?.focus();
    },
    [selectTab],
  );

  const spec = MODE_BY_ID[mode];
  const statusLabel = !ready ? "PAUSE" : reduced ? "STATIC" : paused ? "PLAY" : "PAUSE";

  return (
    <figure
      ref={figureRef}
      className="plate"
      data-mode={mode}
      data-motion={motionEnabled ? "on" : "off"}
      data-paused={paused ? "true" : "false"}
      aria-labelledby="figure-caption"
    >
      <div className="plate__meta">
        <p className="plate__meta-left">
          <span className="plate__fig">FIG. 01</span>
          <span className="plate__meta-divider" aria-hidden="true">
            /
          </span>
          <span>ANNALES STORAGE ARCHITECTURE</span>
        </p>
        <div className="plate__meta-right">
          <span className="plate__meta-system">SYSTEM OVERVIEW</span>
          <button
            type="button"
            className="plate__motion"
            onClick={() => setPaused((value) => !value)}
            disabled={!ready || reduced}
            aria-pressed={paused}
            aria-label={
              reduced
                ? "Animation disabled by the reduced-motion preference"
                : paused
                  ? "Resume the architecture animation"
                  : "Pause the architecture animation"
            }
          >
            <span className="plate__motion-icon" aria-hidden="true">
              {statusLabel === "PLAY" ? "▶" : statusLabel === "STATIC" ? "—" : "❙❙"}
            </span>
            {statusLabel}
          </button>
        </div>
      </div>

      <div
        className="plate__canvas"
        id="architecture-panel"
        role="tabpanel"
        aria-labelledby={`tab-${mode}`}
      >
        <DesktopDiagram mode={mode} motion={motionEnabled} />
        <CompactDiagram mode={mode} motion={motionEnabled} />
      </div>

      <div className="plate__tabs" role="tablist" aria-label="Architecture flow">
        {MODES.map((item, index) => (
          <button
            key={item.id}
            ref={(element) => {
              tabRefs.current[index] = element;
            }}
            id={`tab-${item.id}`}
            type="button"
            role="tab"
            aria-selected={mode === item.id}
            aria-controls="architecture-panel"
            tabIndex={mode === item.id ? 0 : -1}
            className="plate__tab"
            style={{ "--tab-accent": item.color } as CSSProperties}
            onClick={() => selectTab(item.id)}
            onKeyDown={(event) => handleTabKeyDown(event, index)}
          >
            <span className="plate__tab-index">{item.index}</span>
            <span>{item.label}</span>
            <span className="plate__tab-rule" aria-hidden="true" />
          </button>
        ))}
      </div>

      <figcaption className="plate__caption" id="figure-caption">
        <div className="plate__caption-index" aria-hidden="true">
          {spec.index}
        </div>
        <div className="plate__caption-copy" key={mode}>
          <p className="plate__caption-kicker">
            {spec.label} path
            {reduced ? <span className="plate__static-note"> · static figure</span> : null}
          </p>
          <p className="plate__caption-title">{spec.captionTitle}</p>
          <p className="plate__caption-text">{spec.caption}</p>
        </div>
        <div className="plate__invariant">
          <span className="plate__invariant-label">invariant</span>
          <span className="plate__invariant-value">{spec.invariant}</span>
        </div>
      </figcaption>
    </figure>
  );
}
