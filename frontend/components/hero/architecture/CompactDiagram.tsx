import { MODE_BY_ID, type Mode } from "@/lib/modes";
import { ArrowMarker, CrossMark, Packet, RefreshGlyph } from "./primitives";

const PATHS = {
  apiRequest: "M140 118 C96 120 88 148 112 148",
  apiResponse: "M304 148 C332 148 324 118 282 118",
  writeManifest: "M260 352V412",
  writeLog: "M368 352 C392 370 388 480 384 500 C383 512 380 523 376 523",
  recoverManifest: "M260 412V352",
  recoverSnapshot: "M376 591 C392 591 388 550 388 500 C388 430 388 380 368 352",
  recoverLogs: "M376 523 C390 523 386 490 386 450 C386 410 386 380 368 352",
  compactLogs: "M32 650H22V523H32",
  compactManifest: "M376 650H386V466H376",
  compactSnapshot: "M210 632V612",
};

type Props = {
  mode: Mode;
  motion: boolean;
};

export function CompactDiagram({ mode, motion }: Props) {
  const spec = MODE_BY_ID[mode];

  return (
    <svg
      className="diagram diagram--compact"
      viewBox="0 0 420 720"
      role="img"
      aria-labelledby="compact-diagram-title compact-diagram-desc"
      data-mode={mode}
    >
      <title id="compact-diagram-title">Annales storage architecture, compact view</title>
      <desc id="compact-diagram-desc">
        A vertical diagram of Annales. The HTTP API sits above the RAM engine, which contains the
        in-memory state. Below it, inside S3-compatible object storage, are the manifest, immutable
        logs, a snapshot and the background compactor. Currently showing the {spec.label.toLowerCase()} flow.
      </desc>

      <defs>
        <ArrowMarker id="compact-arrow-base" color="var(--dia-faint)" />
        <ArrowMarker id="compact-arrow-read" color="#456B91" />
        <ArrowMarker id="compact-arrow-write" color="#202124" />
        <ArrowMarker id="compact-arrow-compact" color="#8A735B" />
        <ArrowMarker id="compact-arrow-recover" color="#56707F" />
      </defs>

      <CrossMark x={6} y={8} size={3} />
      <CrossMark x={414} y={8} size={3} />
      <CrossMark x={6} y={712} size={3} />
      <CrossMark x={414} y={712} size={3} />

      <g aria-hidden="true">
        <text x={16} y={28} className="label label--tracked">
          ANNALES · GO PROCESS
        </text>
        <text x={394} y={28} textAnchor="end" className="label label--micro">
          SINGLE NODE
        </text>
        <text x={16} y={384} className="label label--tracked">
          S3-COMPATIBLE STORAGE
        </text>
        <text x={394} y={384} textAnchor="end" className="label label--micro">
          MINIO / AWS
        </text>

        <rect x={16} y={40} width={376} height={328} rx={2} className="frame frame--process" />
        <rect x={16} y={396} width={376} height={300} rx={2} className="frame frame--storage" />
        <path d="M16 48V40H24M392 40H384M16 360V368H24M392 368H384" className="frame-tick" />
        <path d="M16 404V396H24M392 396H384M16 688V696H24M392 696H384" className="frame-tick" />
      </g>

      <g aria-hidden="true" className="base-paths">
        <path d={PATHS.apiRequest} className="flow-path flow-path--base" markerEnd="url(#compact-arrow-base)" />
        <path d={PATHS.apiResponse} className="flow-path flow-path--base" markerEnd="url(#compact-arrow-base)" />
      </g>

      {/* API */}
      <g className="node node--api" aria-hidden="true">
        <rect x={126} y={48} width={168} height={70} rx={2} className="node-surface" />
        <text x={138} y={63} className="label label--micro">
          01 · INGRESS
        </text>
        <circle cx={280} cy={60} r={1.8} className="port-dot" />
        <text x={138} y={80} className="node-title node-title--sans">
          HTTP API
        </text>
        <line x1={126} y1={89} x2={294} y2={89} className="hairline" />
        <text x={138} y={100} className="label label--endpoint">
          GET · PUT · DELETE
        </text>
        <text x={138} y={111} className="label label--micro label--dim">
          /kv/&#123;key&#125; · :8080 · net/http
        </text>
      </g>

      {/* Engine */}
      <g className="node node--engine" aria-hidden="true">
        <rect x={36} y={156} width={352} height={204} rx={2} className="engine-layer engine-layer--back" />
        <rect x={32} y={152} width={352} height={204} rx={2} className="engine-layer engine-layer--middle" />
        <rect x={28} y={148} width={352} height={204} rx={2} className="node-surface engine-card" />
        <rect x={28} y={148} width={352} height={2.4} className="engine-accent" />
        <text x={44} y={168} className="label label--micro">
          02 · ENGINE
        </text>
        <text x={364} y={168} textAnchor="end" className="label label--micro label--dim engine-state">
          READY
        </text>
        <text x={44} y={190} className="node-title node-title--serif">
          RAM Engine
        </text>
        <text x={44} y={203} className="label label--micro label--dim">
          READ + WRITE COORDINATOR
        </text>
        <line x1={44} y1={211} x2={364} y2={211} className="hairline" />
        <text x={44} y={227} className="label label--tracked label--dim">
          IN-MEMORY STATE
        </text>
        <text x={364} y={227} textAnchor="end" className="label label--micro label--dim">
          map
        </text>
        <g className="map-sheet">
          <rect x={44} y={235} width={320} height={96} rx={1.5} className="map-sheet__surface" />
          <rect x={44} y={235} width={320} height={19} className="map-sheet__header" />
          <text x={52} y={248} className="label label--micro label--dim">
            KEY
          </text>
          <text x={356} y={248} textAnchor="end" className="label label--micro label--dim">
            VALUE
          </text>
          {[
            { key: "greeting", value: "hello", y: 266 },
            { key: "seq", value: "0042", y: 286 },
            { key: "region", value: "local", y: 306 },
          ].map((row, index) => (
            <g key={row.key} className={`map-row map-row--${index + 1}`}>
              <rect x={44} y={254 + index * 20} width={320} height={20} className="map-row__bg" />
              <text x={52} y={row.y} className="map-row__key">
                {row.key}
              </text>
              <line x1={125} y1={row.y - 2.5} x2={320} y2={row.y - 2.5} className="map-row__leader" />
              <text x={356} y={row.y} textAnchor="end" className="map-row__value">
                {row.value}
              </text>
            </g>
          ))}
          <line x1={44} y1={314} x2={364} y2={314} className="hairline" />
          <text x={52} y={325} className="label label--micro label--dim">
            RWMUTEX · AUTHORITATIVE FOR READS
          </text>
        </g>
        <text x={44} y={344} className="label label--micro label--dim">
          ILLUSTRATIVE KEYS
        </text>
      </g>

      {/* Manifest */}
      <g className="node node--manifest" aria-hidden="true">
        <path d="M32 412H350L376 438V474H32Z" className="node-surface manifest-card" />
        <path d="M350 412V438H376" className="fold" />
        <text x={44} y={431} className="object-title">
          index.json
        </text>
        <rect x={322} y={419} width={38} height={15} rx={1} className="stamp stamp--blue" />
        <text x={341} y={429.5} textAnchor="middle" className="stamp__text">
          CAS
        </text>
        <line x1={44} y1={438} x2={360} y2={438} className="hairline" />
        <text x={44} y={452} className="label label--micro label--dim">
          current_seq
        </text>
        <text x={360} y={452} textAnchor="end" className="label label--value">
          0042
        </text>
        <text x={44} y={466} className="label label--micro label--dim">
          current_snapshot_seq
        </text>
        <g className="manifest-seq">
          <text x={360} y={466} textAnchor="end" className="label label--value manifest-seq__old">
            0038
          </text>
          <text x={360} y={466} textAnchor="end" className="label label--value manifest-seq__new">
            0042
          </text>
        </g>
      </g>

      {/* Logs */}
      <g className="node node--logs" aria-hidden="true">
        <text x={32} y={492} className="label label--tracked label--dim">
          IMMUTABLE LOGS
        </text>
        <text x={376} y={492} textAnchor="end" className="label label--micro label--dim">
          APPEND-ONLY
        </text>
        {[
          { label: "LOG 001", seq: "0040" },
          { label: "LOG 002", seq: "0041" },
          { label: "LOG 003", seq: "0042" },
        ].map((log, index) => (
          <g key={log.label} className={`log-card ${index === 2 ? "log-card--top" : ""}`}>
            <rect x={32 + index * 120} y={502} width={104} height={50} rx={1} fill="var(--figure)" className="log-card__surface" />
            <rect x={32 + index * 120} y={502} width={104} height={50} rx={1} className="log-card__outline" />
            <line x1={40 + index * 120} y1={520} x2={40 + index * 120} y2={540} className="log-card__perf" />
            <circle cx={38 + index * 120} cy={523} r={1.1} className="log-card__hole" />
            <circle cx={38 + index * 120} cy={536} r={1.1} className="log-card__hole" />
            <text x={44 + index * 120} y={519} className="object-title object-title--small">
              {log.label}
            </text>
            <text x={44 + index * 120} y={532} className="label label--micro label--dim">
              seq {log.seq}
            </text>
            <circle cx={124 + index * 120} cy={531} r={1.4} className="log-card__dot" />
            <line
              x1={44 + index * 120}
              y1={527}
              x2={124 + index * 120}
              y2={527}
              pathLength={1}
              className="log-strike"
              style={{ animationDelay: `${index * 0.12}s` }}
            />
          </g>
        ))}
      </g>

      {/* Snapshot */}
      <g className="node node--snapshot" aria-hidden="true">
        <rect x={40} y={568} width={344} height={48} rx={1} className="snapshot-back" />
        <rect x={32} y={564} width={344} height={48} rx={1} className="node-surface snapshot-card" />
        <text x={44} y={581} className="object-title object-title--small">
          SNAPSHOT
        </text>
        <g className="snapshot-version snapshot-version--old">
          <text x={364} y={581} textAnchor="end" className="label label--micro label--dim">
            seq 0038
          </text>
          <text x={44} y={594} className="label label--micro label--dim">
            snapshot-000038.json · materialized state
          </text>
        </g>
        <g className="snapshot-version snapshot-version--new">
          <text x={364} y={581} textAnchor="end" className="label label--micro label--dim">
            seq 0042
          </text>
          <text x={44} y={594} className="label label--micro label--dim">
            snapshot-000042.json · materialized state
          </text>
        </g>
        <line x1={44} y1={599} x2={364} y2={599} className="hairline" />
        <text x={44} y={607} className="label label--micro label--dim">
          LAST LOG KEY
        </text>
      </g>

      {/* Compactor */}
      <g className="node node--compactor" aria-hidden="true">
        <rect x={32} y={632} width={344} height={38} rx={2} className="compactor-card" />
        <g className="compactor-icon" transform="translate(46 651)">
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
        <text x={61} y={649} className="label label--micro label--wide">
          BACKGROUND COMPACTOR
        </text>
        <text x={61} y={661} className="label label--micro label--dim">
          SNAPSHOT · CAS · GC
        </text>
        <text x={364} y={653} textAnchor="end" className="label label--micro">
          5 MIN
        </text>
      </g>

      {/* Active flow */}
      <g aria-hidden="true" className="active-flow" key={`flow-${mode}`}>
        {(mode === "read" || mode === "write") && (
          <>
            <path d={PATHS.apiRequest} className="flow-path flow-path--active" markerEnd={`url(#compact-arrow-${mode})`} />
          </>
        )}

        {mode === "read" && (
          <path d={PATHS.apiResponse} className="flow-path flow-path--active" markerEnd="url(#compact-arrow-read)" />
        )}

        {mode === "write" && (
          <>
            <path d={PATHS.writeLog} className="flow-path flow-path--active" markerEnd="url(#compact-arrow-write)" />
            <g className="reveal reveal--cas">
              <path d={PATHS.writeManifest} className="flow-path flow-path--active" markerEnd="url(#compact-arrow-write)" />
            </g>
            <g className="reveal reveal--ack">
              <path d={PATHS.apiResponse} className="flow-path flow-path--active" markerEnd="url(#compact-arrow-write)" />
            </g>
          </>
        )}

        {mode === "recover" && (
          <>
            <g className="reveal reveal--first">
              <path d={PATHS.recoverManifest} className="flow-path flow-path--active" markerEnd="url(#compact-arrow-recover)" />
            </g>
            <g className="reveal reveal--second">
              <path d={PATHS.recoverSnapshot} className="flow-path flow-path--active" markerEnd="url(#compact-arrow-recover)" />
            </g>
            <g className="reveal reveal--third">
              <path d={PATHS.recoverLogs} className="flow-path flow-path--active" markerEnd="url(#compact-arrow-recover)" />
            </g>
          </>
        )}

        {mode === "compact" && (
          <>
            <path
              d={PATHS.compactLogs}
              className="flow-path flow-path--active flow-path--dashed"
              markerEnd="url(#compact-arrow-compact)"
            />
            <path
              d={PATHS.compactManifest}
              className="flow-path flow-path--active flow-path--dashed"
              markerEnd="url(#compact-arrow-compact)"
            />
            <path
              d={PATHS.compactSnapshot}
              className="flow-path flow-path--active flow-path--dashed"
              markerEnd="url(#compact-arrow-compact)"
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
            <Packet path={PATHS.writeLog} color="#202124" label="LOG" cycle={13} start={2.1} end={4.3} square />
            <Packet path={PATHS.writeManifest} color="#202124" label="CAS" cycle={13} start={4.6} end={6.4} square />
            <Packet path={PATHS.apiResponse} color="#202124" label="204" cycle={13} start={8.8} end={10.2} square />
          </>
        )}

        {motion && mode === "recover" && (
          <>
            <Packet path={PATHS.recoverManifest} color="#56707F" label="READ" cycle={10} start={0.4} end={1.9} labelBelow />
            <Packet path={PATHS.recoverSnapshot} color="#56707F" label="SNAPSHOT" cycle={10} start={2.4} end={5.6} />
            <Packet path={PATHS.recoverLogs} color="#56707F" label="LOG" cycle={10} start={6.2} end={8} />
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
