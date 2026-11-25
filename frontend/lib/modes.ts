export type Mode = "read" | "write" | "compact" | "recover";

export type ModeSpec = {
  id: Mode;
  index: string;
  label: string;
  color: string;
  captionTitle: string;
  caption: string;
  invariant: string;
  noteTitle: string;
  noteLines: string[];
};

export const MODES: ModeSpec[] = [
  {
    id: "read",
    index: "01",
    label: "Read",
    color: "#456B91",
    captionTitle: "Reads end in memory.",
    caption:
      "A GET resolves against the materialized map inside the engine. Object storage stays off the read path.",
    invariant: "ZERO S3 REQUESTS",
    noteTitle: "READ PATH",
    noteLines: ["API → RAM → API", "source · in-memory map"],
  },
  {
    id: "write",
    index: "02",
    label: "Write",
    color: "#202124",
    captionTitle: "Durable before visible.",
    caption:
      "The engine uploads an immutable log object, commits its position with a CAS update to index.json, then applies the mutation to RAM.",
    invariant: "LOG → CAS → APPLY",
    noteTitle: "WRITE PATH",
    noteLines: ["log object uploaded", "manifest CAS commits", "then apply to RAM"],
  },
  {
    id: "compact",
    index: "03",
    label: "Compact",
    color: "#8A735B",
    captionTitle: "History is periodically bounded.",
    caption:
      "Every five minutes the background compactor materializes committed state as a snapshot, advances CurrentSnapshotSeq, and retires superseded logs.",
    invariant: "SNAPSHOT + CAS + GC",
    noteTitle: "COMPACTION",
    noteLines: ["snapshot committed state", "advance snapshot seq", "delete old log objects"],
  },
  {
    id: "recover",
    index: "04",
    label: "Recover",
    color: "#56707F",
    captionTitle: "State rebuilt from the commit point.",
    caption:
      "At startup the materializer reads index.json, loads the latest snapshot, then replays only committed logs after it. Uncommitted orphan objects are ignored.",
    invariant: "SNAPSHOT + LOG CHAIN",
    noteTitle: "RECOVERY",
    noteLines: ["read manifest", "load latest snapshot", "replay committed log chain"],
  },
];

export const MODE_BY_ID = Object.fromEntries(MODES.map((mode) => [mode.id, mode])) as Record<
  Mode,
  ModeSpec
>;
