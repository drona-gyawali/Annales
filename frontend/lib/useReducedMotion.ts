"use client";

import { useEffect, useState } from "react";

/**
 * Tracks the user's reduced-motion preference. Starts `false` on the server and
 * on the first client render, then settles after mount. The diagram only starts
 * its SVG timeline once this has resolved, which avoids a flash of motion for
 * people who have asked for less of it.
 */
export function useReducedMotion() {
  const [reduced, setReduced] = useState(false);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    const sync = () => {
      setReduced(query.matches);
      setReady(true);
    };

    sync();
    query.addEventListener("change", sync);
    return () => query.removeEventListener("change", sync);
  }, []);

  return { reduced, ready };
}
