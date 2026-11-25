import { ArrowUpRight } from "./icons";

const ARCHITECTURE_URL =
  "https://github.com/drona-gyawali/Annales/blob/main/docs/blog/architecture.md";

export function WorkingNotes() {
  return (
    <section className="notes" aria-labelledby="notes-title" id="notes">
      <div className="shell">
        <div className="notes__intro">
          <p className="section-label">
            <span className="section-label__number">02</span>
            Working notes
          </p>
          <h2 className="notes__title" id="notes-title">
            The trade-offs
            <br />
            <em>are the point.</em>
          </h2>
          <p className="notes__lede">
            Annales is a small systems-engineering study: durable writes coordinated through object
            storage, reads served from memory, and recovery treated as part of the storage design.
          </p>
        </div>

        <div className="notes__grid">
          <article className="note">
            <p className="note__index">i.</p>
            <div>
              <h3>Reads without a network hop</h3>
              <p>
                A <code>GET</code> ends at a Go map guarded by an <code>RWMutex</code>. A cache hit
                does not become an S3 request.
              </p>
            </div>
            <p className="note__tag">RAM / NO S3</p>
          </article>

          <article className="note">
            <p className="note__index">ii.</p>
            <div>
              <h3>A manifest commits the write</h3>
              <p>
                Uploading an immutable log batch is not enough. The conditional update to{" "}
                <code>index.json</code> identifies the committed position; conflict retries keep
                writers coordinated.
              </p>
            </div>
            <p className="note__tag">LOG → CAS → APPLY</p>
          </article>

          <article className="note">
            <p className="note__index">iii.</p>
            <div>
              <h3>Recovery is a first-class path</h3>
              <p>
                Startup reads the latest snapshot and replays only the committed log chain after
                it. Compaction keeps that chain — and restart work — bounded.
              </p>
            </div>
            <p className="note__tag">SNAPSHOT + REPLAY</p>
          </article>
        </div>

        <div className="measure">
          <div className="measure__head">
            <p className="section-label">
              <span className="section-label__number">03</span>
              Observed, locally
            </p>
            <p className="measure__status">Preliminary</p>
          </div>

          <dl className="measure__grid">
            <div>
              <dt>Cache-hit read · P50</dt>
              <dd>0.12 <span>ms</span></dd>
            </div>
            <div>
              <dt>Cache-hit read · P99</dt>
              <dd>0.21 <span>ms</span></dd>
            </div>
            <div>
              <dt>Durable write throughput</dt>
              <dd>306–343 <span>ops/s</span></dd>
            </div>
          </dl>

          <div className="measure__foot">
            <p>
              Repository benchmark, single-key sequential reads and 64-byte writes at concurrency
              10–100, against local MinIO. These are preliminary figures from one machine, not a
              controlled comparison.
            </p>
            <a href={ARCHITECTURE_URL} target="_blank" rel="noreferrer">
              Read the measurements note
              <ArrowUpRight />
            </a>
          </div>
        </div>
      </div>
    </section>
  );
}
