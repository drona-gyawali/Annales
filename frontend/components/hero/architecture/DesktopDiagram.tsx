import { MODE_BY_ID, type Mode } from "@/lib/modes";
import { ArrowMarker, CrossMark, Packet, RefreshGlyph } from "./primitives";

const PATHS = {
  apiRequest: "M178 142 C198 142 204 126 226 126",
  apiResponse: "M226 178 C202 178 196 194 178 194",
  writeLog: "M422 280 C460 296 474 308 512 308",
  manifestCas: "M422 156 C458 146 470 114 512 114",
  manifestReturn: "M512 114 C470 114 458 146 422 156",
  recoverLogs: "M512 316 C470 316 460 350 422 350",
  recoverSnapshot: "M512 386 C470 386 460 396 422 396",
  compactLogs: "M512 434H502V316H512",
  compactManifest: "M728 441H736V150H728",
  compactSnapshot: "M620 422V398",
};

const ACTIVE_MARKERS: Record<Mode, string> = {
  read: "url(#arrow-read)",
  write: "url(#arrow-write)",
  compact: "url(#arrow-compact)",
  recover: "url(#arrow-recover)",
};

type Props = {
  mode: Mode;
  motion: boolean;
};

export function DesktopDiagram({ mode, motion }: Props) {
  const spec = MODE_BY_ID[mode];
  const arrow = ACTIVE_MARKERS[mode];

  return (
    <svg
      className="diagram diagram--desktop"
      viewBox="0 0 760 548"
      role="img"
      aria-labelledby="desktop-diagram-title desktop-diagram-desc"
      data-mode={mode}
    >
      <title id="desktop-diagram-title">Annales storage architecture</title>
      <desc id="desktop-diagram-desc">
        A technical diagram of the Annales Go process and S3-compatible object storage. The HTTP
        API connects to a central RAM engine containing the in-memory state. Immutable logs, a CAS
        manifest, snapshots and a background compactor live in object storage. The read flow goes
        from the API to the RAM engine and back; the write flow uploads a log object, commits the
        manifest with CAS, then applies the mutation to RAM. Currently showing the {spec.label.toLowerCase()} flow.
      </desc>

      <defs>
        <ArrowMarker id="arrow-base" color="var(--dia-faint)" />
        <ArrowMarker id="arrow-read" color="#456B91" />
        <ArrowMarker id="arrow-write" color="#202124" />
        <ArrowMarker id="arrow-compact" color="#8A735B" />
        <ArrowMarker id="arrow-recover" color="#56707F" />
      </defs>

      <CrossMark x={8} y={10} />
      <CrossMark x={752} y={10} />
      <CrossMark x={8} y={538} />
      <CrossMark x={752} y={538} />

      <g aria-hidden="true">
        {/* Frame labels */}
        <text x={20} y={26} className="label label--tracked">
          ANNALES · GO PROCESS
        </text>
        <text x={448} y={26} textAnchor="end" className="label label--micro">
          SINGLE NODE
        </text>
        <text x={496} y={26} className="label label--tracked">
          S3-COMPATIBLE STORAGE
        </text>
        <text x={740} y={26} textAnchor="end" className="label label--micro">
          MINIO / AWS
        </text>

        {/* Boundaries */}
        <rect
          x={20}
          y={38}
          width={428}
          height={448}
          rx={2}
          className="frame frame--process"
        />
        <rect x={496} y={38} width={248} height={448} rx={2} className="frame frame--storage" />
        <path d="M20 46V38H28M448 38H440M496 38H504M740 38H732" className="frame-tick" />
        <path d="M20 478V486H28M448 486H440M496 486H504M740 486H732" className="frame-tick" />

        {/* Fine drafting rule */}
        <line x1={20} y1={506} x2={740} y2={506} className="hairline" />
        <line x1={20} y1={501} x2={20} y2={511} className="hairline" />
        <line x1={380} y1={501} x2={380} y2={511} className="hairline" />
        <line x1={740} y1={501} x2={740} y2={511} className="hairline" />
        <text x={380} y={523} textAnchor="middle" className="label label--micro label--wide">
          LOGICAL ARCHITECTURE · NOT TO SCALE
        </text>
      </g>

      {/* Connection paths, always visible but quiet */}
      <g aria-hidden="true" className="base-paths">
        <path d={PATHS.apiRequest} className="flow-path flow-path--base" markerEnd="url(#arrow-base)" />
        <path d={PATHS.apiResponse} className="flow-path flow-path--base" markerEnd="url(#arrow-base)" />
        <path d={PATHS.writeLog} className="flow-path flow-path--base" markerEnd="url(#arrow-base)" />
        <path d={PATHS.manifestCas} className="flow-path flow-path--base" markerEnd="url(#arrow-base)" />
        <path
          d={PATHS.recoverSnapshot}
          className="flow-path flow-path--base flow-path--faint"
          markerEnd="url(#arrow-base)"
        />
      </g>

      {/* HTTP API */}
      <g className="node node--api" aria-hidden="true">
        <rect x={44} y={104} width={134} height={126} rx={2} className="node-surface" />
        <text x={56} y={121} className="label label--micro">
          01 · INGRESS
        </text>
        <circle cx={164} cy={116} r={1.8} className="port-dot" />
        <text x={56} y={140} className="node-title node-title--sans">
          HTTP API
        </text>
        <line x1={44} y1={150} x2={178} y2={150} className="hairline" />
        {[
          { y: 165, method: "GET", path: "/kv/{key}" },
          { y: 181, method: "PUT", path: "/kv/{key}" },
          { y: 197, method: "DEL", path: "/kv/{key}" },
        ].map((row) => (
          <g key={row.method}>
            <rect x={56} y={row.y - 9} width={27} height={12} rx={1} className="method-chip" />
            <text x={69.5} y={row.y} textAnchor="middle" className="label label--method">
              {row.method}
            </text>
            <text x={91} y={row.y} className="label label--endpoint">
              {row.path}
            </text>
          </g>
        ))}
        <text x={56} y={219} className="label label--micro label--dim">
          :8080 · net/http · json
        </text>
      </g>

      {/* Mode note in otherwise open space */}
      <g className="path-note" key={`note-${mode}`} style={{ color: spec.color }} aria-hidden="true">
        <text x={44} y={273} className="label label--micro label--wide">
          PATH NOTE · {spec.index}
        </text>
        <line x1={44} y1={281} x2={178} y2={281} className="hairline" />
        <text x={44} y={298} className="path-note__title">
          {spec.noteTitle}
        </text>
        <text x={44} y={315} className="path-note__invariant">
          {spec.invariant}
        </text>
        <line x1={44} y1={326} x2={178} y2={326} className="hairline" />
        {spec.noteLines.map((line, index) => (
          <text key={line} x={44} y={341 + index * 13} className="path-note__line">
            {line}
          </text>
        ))}
        <g transform="translate(44 393)">
          <circle cx={2} cy={2} r={1.6} fill="currentColor" />
          <text x={9} y={4.5} className="path-note__foot">
            ACTIVE FLOW
          </text>
        </g>
      </g>

      {/* RAM engine, the focal object */}
      <g className="node node--engine" aria-hidden="true">
        <rect x={234} y={63} width={196} height={374} rx={2} className="engine-layer engine-layer--back" />
        <rect x={230} y={59.5} width={196} height={374} rx={2} className="engine-layer engine-layer--middle" />
        <rect x={226} y={56} width={196} height={374} rx={2} className="node-surface engine-card" />
        <rect x={226} y={56} width={196} height={2.4} className="engine-accent" />

        <text x={242} y={77} className="label label--micro">
          02 · ENGINE
        </text>
        <text x={406} y={77} textAnchor="end" className="label label--micro label--dim engine-state">
          READY
        </text>
        <text x={242} y={101} className="node-title node-title--serif">
          RAM Engine
        </text>
        <text x={242} y={115} className="label label--micro label--dim">
          READ + WRITE COORDINATOR
        </text>
        <line x1={242} y1={126} x2={406} y2={126} className="hairline" />

        <text x={242} y={143} className="label label--tracked label--dim">
          IN-MEMORY STATE
        </text>
        <text x={406} y={143} textAnchor="end" className="label label--micro label--dim">
          map
        </text>

        <g className="map-sheet">
          <rect x={242} y={152} width={164} height={168} rx={1.5} className="map-sheet__surface" />
          <rect x={242} y={152} width={164} height={22} className="map-sheet__header" />
          <text x={250} y={166} className="label label--micro label--dim">
            KEY
          </text>
          <text x={398} y={166} textAnchor="end" className="label label--micro label--dim">
            VALUE
          </text>
          {[
            { key: "greeting", value: "hello", y: 184 },
            { key: "seq", value: "0042", y: 218 },
            { key: "region", value: "local", y: 252 },
          ].map((row, index) => (
            <g key={row.key} className={`map-row map-row--${index + 1}`}>
              <rect x={242} y={174 + index * 34} width={164} height={34} className="map-row__bg" />
              <text x={250} y={row.y} className="map-row__key">
                {row.key}
              </text>
              <line
                x1={row.key === "greeting" ? 292 : row.key === "seq" ? 272 : 284}
                y1={row.y - 2.5}
                x2={362}
                y2={row.y - 2.5}
                className="map-row__leader"
              />
              <text x={398} y={row.y} textAnchor="end" className="map-row__value">
                {row.value}
              </text>
            </g>
          ))}
          <line x1={242} y1={276} x2={406} y2={276} className="hairline" />
          <text x={250} y={293} className="label label--micro label--dim">
            RWMUTEX · AUTHORITATIVE FOR READS
          </text>
          <text x={250} y={306} className="label label--micro label--dim">
            ILLUSTRATIVE KEYS
          </text>
        </g>

        <line x1={242} y1={336} x2={406} y2={336} className="hairline" />
        <text x={242} y={352} className="label label--micro label--dim">
          COMMIT ORDER
        </text>
        <g className="commit-order">
          <text x={242} y={368} className="label label--micro">
            1&nbsp;&nbsp;log object
          </text>
          <text x={242} y={381} className="label label--micro">
            2&nbsp;&nbsp;manifest CAS
          </text>
          <text x={242} y={394} className="label label--micro">
            3&nbsp;&nbsp;apply to RAM
          </text>
          <path d="M238 362V392" className="order-rule" />
        </g>
        <text x={406} y={410} textAnchor="end" className="label label--micro label--dim">
          single writer lock
        </text>
      </g>

      {/* Manifest */}
      <g className="node node--manifest" aria-hidden="true">
        <path d="M512 72H712L728 88V164H512Z" className="node-surface manifest-card" />
        <path d="M712 72V88H728" className="fold" />
        <text x={524} y={91} className="object-title">
          index.json
        </text>
        <rect x={678} y={79} width={36} height={14} rx={1} className="stamp stamp--blue" />
        <text x={696} y={88.6} textAnchor="middle" className="stamp__text">
          CAS
        </text>
        <line x1={524} y1={98} x2={716} y2={98} className="hairline" />
        <text x={524} y={113} className="label label--micro label--dim">
          current_seq
        </text>
        <text x={716} y={113} textAnchor="end" className="label label--value">
          0042
        </text>
        <text x={524} y={129} className="label label--micro label--dim">
          current_snapshot_seq
        </text>
        <g className="manifest-seq">
          <text x={716} y={129} textAnchor="end" className="label label--value manifest-seq__old">
            0038
          </text>
          <text x={716} y={129} textAnchor="end" className="label label--value manifest-seq__new">
            0042
          </text>
        </g>
        <text x={524} y={145} className="label label--micro label--dim">
          current_log_key
        </text>
        <text x={716} y={145} textAnchor="end" className="label label--micro label--value-muted">
          logs/0042-…
        </text>
        <text x={524} y={157} className="label label--micro label--dim">
          commit point · conditional write
        </text>
      </g>

      {/* Logs */}
      <g className="node node--logs" aria-hidden="true">
        <text x={512} y={185} className="label label--tracked label--dim">
          IMMUTABLE LOGS
        </text>
        <text x={728} y={185} textAnchor="end" className="label label--micro label--dim">
          APPEND-ONLY
        </text>
        {[
          { label: "LOG 001", ops: "SET greeting", seq: "seq 0040" },
          { label: "LOG 002", ops: "SET seq", seq: "seq 0041" },
          { label: "LOG 003", ops: "DELETE cache", seq: "seq 0042" },
        ].map((log, index) => (
          <g key={log.label} className={`log-card ${index === 2 ? "log-card--top" : ""}`}>
            <rect x={512} y={196 + index * 46} width={216} height={40} rx={1} fill="var(--figure)" className="log-card__surface" />
            <rect x={512} y={196 + index * 46} width={216} height={40} rx={1} className="log-card__outline" />
            <line x1={528} y1={201 + index * 46} x2={528} y2={231 + index * 46} className="log-card__perf" />
            <circle cx={520} cy={210 + index * 46} r={1.2} className="log-card__hole" />
            <circle cx={520} cy={222 + index * 46} r={1.2} className="log-card__hole" />
            <text x={537} y={212 + index * 46} className="object-title object-title--small">
              {log.label}
            </text>
            <text x={537} y={225 + index * 46} className="label label--micro label--dim">
              {log.ops}
            </text>
            <text x={716} y={212 + index * 46} textAnchor="end" className="label label--micro label--dim">
              {log.seq}
            </text>
            <circle cx={718} cy={224 + index * 46} r={1.5} className="log-card__dot" />
            <line
              x1={537}
              y1={216 + index * 46}
              x2={716}
              y2={216 + index * 46}
              pathLength={1}
              className="log-strike"
              style={{ animationDelay: `${index * 0.12}s` }}
            />
          </g>
        ))}
        <text x={512} y={338} className="label label--micro label--dim">
          one immutable object per committed batch
        </text>
      </g>

      {/* Snapshot */}
      <g className="node node--snapshot" aria-hidden="true">
        <rect x={520} y={351} width={200} height={52} rx={1} className="snapshot-back" />
        <rect x={512} y={346} width={208} height={52} rx={1} className="node-surface snapshot-card" />
        <text x={524} y={363} className="object-title object-title--small">
          SNAPSHOT
        </text>
        <g className="snapshot-version snapshot-version--old">
          <text x={716} y={363} textAnchor="end" className="label label--micro label--dim">
            seq 0038
          </text>
          <text x={524} y={377} className="label label--micro label--dim">
            snapshots/snapshot-000038.json
          </text>
        </g>
        <g className="snapshot-version snapshot-version--new">
          <text x={716} y={363} textAnchor="end" className="label label--micro label--dim">
            seq 0042
          </text>
          <text x={524} y={377} className="label label--micro label--dim">
            snapshots/snapshot-000042.json
          </text>
        </g>
        <line x1={524} y1={383} x2={716} y2={383} className="hairline" />
        <text x={524} y={393} className="label label--micro label--dim">
          MATERIALIZED STATE · LAST LOG KEY
        </text>
      </g>

      {/* Background compactor */}
      <g className="node node--compactor" aria-hidden="true">
        <rect x={512} y={422} width={216} height={38} rx={2} className="compactor-card" />
        <g className="compactor-icon" transform="translate(526 441)">
          <g>
            {motion && mode === "compact" ? (
              <animateTransform
                attributeName="transform"
                type="rotate"
                from="0"
                to="360"
                dur="8s"
                repeatCount="indefinite"
              />
            ) : null}
            <RefreshGlyph color="var(--dia-muted)" />
          </g>
        </g>
        <text x={541} y={439} className="label label--micro label--wide">
          BACKGROUND COMPACTOR
        </text>
        <text x={541} y={451} className="label label--micro label--dim">
          SNAPSHOT · CAS · GC
        </text>
        <text x={716} y={443} textAnchor="end" className="label label--micro">
          5 MIN
        </text>
      </g>

      {/* Active flow — the only colored layer */}
      <g aria-hidden="true" className="active-flow" key={`flow-${mode}`}>
        {(mode === "read" || mode === "write") && (
          <>
            <path d={PATHS.apiRequest} className="flow-path flow-path--active" markerEnd={arrow} />
            <text x={202} y={117} textAnchor="middle" className="path-caption">
              request
            </text>
          </>
        )}

        {mode === "read" && (
          <>
            <path d={PATHS.apiResponse} className="flow-path flow-path--active" markerEnd={arrow} />
            <text x={202} y={211} textAnchor="middle" className="path-caption">
              response
            </text>
          </>
        )}

        {mode === "write" && (
          <>
            <path d={PATHS.writeLog} className="flow-path flow-path--active" markerEnd={arrow} />
            <g className="reveal reveal--cas">
              <path d={PATHS.manifestCas} className="flow-path flow-path--active" markerEnd={arrow} />
            </g>
            <g className="reveal reveal--ack">
              <path d={PATHS.manifestReturn} className="flow-path flow-path--active flow-path--ghost" />
              <path d={PATHS.apiResponse} className="flow-path flow-path--active" markerEnd={arrow} />
              <text x={202} y={211} textAnchor="middle" className="path-caption">
                response
              </text>
            </g>
          </>
        )}

        {mode === "recover" && (
          <>
            <g className="reveal reveal--first">
              <path d={PATHS.manifestReturn} className="flow-path flow-path--active" markerEnd={arrow} />
            </g>
            <g className="reveal reveal--second">
              <path d={PATHS.recoverSnapshot} className="flow-path flow-path--active" markerEnd={arrow} />
            </g>
            <g className="reveal reveal--third">
              <path d={PATHS.recoverLogs} className="flow-path flow-path--active" markerEnd={arrow} />
            </g>
          </>
        )}

        {mode === "compact" && (
          <>
            <path
              d={PATHS.compactLogs}
              className="flow-path flow-path--active flow-path--dashed"
              markerEnd={arrow}
            />
            <path
              d={PATHS.compactManifest}
              className="flow-path flow-path--active flow-path--dashed"
              markerEnd={arrow}
            />
            <path
              d={PATHS.compactSnapshot}
              className="flow-path flow-path--active flow-path--dashed"
              markerEnd={arrow}
            />
          </>
        )}

        {motion && mode === "read" && (
          <>
            <Packet path={PATHS.apiRequest} color="#456B91" label="GET" cycle={8} start={0.3} end={1.9} square />
            <Packet path={PATHS.apiResponse} color="#456B91" label="200" cycle={8} start={2.5} end={4.1} square />
          </>
        )}

        {motion && mode === "write" && (
          <>
            <Packet path={PATHS.apiRequest} color="#202124" label="PUT" cycle={13} start={0.25} end={1.5} square />
            <Packet path={PATHS.writeLog} color="#202124" label="LOG" cycle={13} start={2.1} end={3.7} square />
            <Packet path={PATHS.manifestCas} color="#202124" label="CAS" cycle={13} start={4.3} end={5.5} square />
            <Packet path={PATHS.manifestReturn} color="#202124" label="COMMIT" cycle={13} start={6.2} end={7.4} square labelBelow />
            <Packet path={PATHS.apiResponse} color="#202124" label="204" cycle={13} start={8.8} end={10.2} square />
          </>
        )}

        {motion && mode === "recover" && (
          <>
            <Packet path={PATHS.manifestReturn} color="#56707F" label="READ" cycle={10} start={0.4} end={1.9} labelBelow />
            <Packet path={PATHS.recoverSnapshot} color="#56707F" label="SNAP" cycle={10} start={2.4} end={4.4} />
            <Packet path={PATHS.recoverLogs} color="#56707F" label="REPLAY" cycle={10} start={5.2} end={7.5} labelBelow />
          </>
        )}

        {motion && mode === "compact" && (
          <>
            <Packet path={PATHS.compactSnapshot} color="#8A735B" label="SNAP" cycle={11} start={4} end={5.2} />
            <Packet
              path={PATHS.compactManifest}
              color="#8A735B"
              label="CAS"
              cycle={11}
              start={6}
              end={7.4}
              labelBelow
            />
            <Packet
              path={PATHS.compactLogs}
              color="#8A735B"
              label="GC"
              cycle={11}
              start={7.8}
              end={9.2}
              labelBelow
            />
          </>
        )}
      </g>
    </svg>
  );
}
