import { ArchitectureFigure } from "./hero/ArchitectureFigure";
import { ArrowRight, ArrowUpRight, GitHubIcon } from "./icons";

const GITHUB_URL = "https://github.com/drona-gyawali/Annales";
const ARCHITECTURE_URL =
  "https://github.com/drona-gyawali/Annales/blob/main/docs/blog/architecture.md";

export function Hero() {
  return (
    <section className="hero" aria-labelledby="hero-title" id="top">
      <div className="shell hero__grid">
        <div className="hero__copy">
          <p className="eyebrow">
            <span className="eyebrow__rule" aria-hidden="true" />
            <span>OPEN SOURCE</span>
            <span className="eyebrow__dot" aria-hidden="true">
              ·
            </span>
            <span>GO</span>
            <span className="eyebrow__ref" aria-hidden="true">
              buit by Drona
            </span>
          </p>

          <h1 className="hero__title" id="hero-title">
            Fast reads,
            <br />
            <em>durable</em> writes.
          </h1>

          <p className="hero__lede">
            Annales is an experimental RAM-first key-value store that pairs in-memory reads with
            immutable logs, a CAS manifest, and snapshots in S3-compatible object storage.
          </p>

          <div className="hero__actions">
            <a className="button button--primary" href={GITHUB_URL} target="_blank" rel="noreferrer">
              <GitHubIcon />
              <span>Explore on GitHub</span>
              <ArrowUpRight />
            </a>
            <a className="button button--secondary" href={ARCHITECTURE_URL} target="_blank" rel="noreferrer">
              <span>Read the architecture</span>
              <ArrowRight />
            </a>
          </div>

          <div className="hero__ledger">
            <span className="hero__status">
              <span className="hero__status-mark" aria-hidden="true" />
              Experimental · not production ready
            </span>
            <span className="hero__ledger-meta">Single node · MinIO / S3</span>
          </div>
        </div>

        <div className="hero__figure">
          <ArchitectureFigure />
          <p className="hero__figure-note">
            <span>Interactive figure</span>
            <span aria-hidden="true">—</span>
            <span>four flows from the architecture note</span>
          </p>
        </div>
      </div>
    </section>
  );
}
