import { ArrowUpRight, BrandMark, GitHubIcon } from "./icons";

const GITHUB_URL = "https://github.com/drona-gyawali/Annales";
const ARCHITECTURE_URL =
  "https://github.com/drona-gyawali/Annales/blob/main/docs/blog/architecture.md";

export function SiteHeader() {
  return (
    <header className="masthead">
      <div className="shell masthead__inner">
        <a className="brand" href="#top" aria-label="Annales, back to top">
          <BrandMark className="brand__mark" />
          <span className="brand__word">Annales</span>
          <span className="brand__issue">Research Project</span>
        </a>

        <p className="masthead__note">A systems note on RAM-first durability</p>

        <nav className="masthead__nav" aria-label="Primary">
          <a className="masthead__link masthead__link--paper" href={ARCHITECTURE_URL} target="_blank" rel="noreferrer">
            Architecture note
            <ArrowUpRight />
          </a>
          <a className="masthead__link" href={GITHUB_URL} target="_blank" rel="noreferrer">
            <GitHubIcon />
            <span>GitHub</span>
          </a>
        </nav>
      </div>
    </header>
  );
}

export function SiteFooter() {
  return (
    <footer className="footer">
      <div className="shell">
        <div className="footer__main">
          <div className="footer__identity">
            <div className="footer__wordmark">
              <BrandMark size={18} />
              <span>Annales</span>
            </div>
            <p>
              An experimental storage system exploring what happens when reads live in RAM and
              durability lives in object storage.
            </p>
          </div>

          <div className="footer__links">
            <div>
              <p className="footer__label">Source</p>
              <a href={GITHUB_URL} target="_blank" rel="noreferrer">
                GitHub repository <ArrowUpRight />
              </a>
            </div>
            <div>
              <p className="footer__label">Reading</p>
              <a href={ARCHITECTURE_URL} target="_blank" rel="noreferrer">
                Architecture note <ArrowUpRight />
              </a>
              <a href="https://github.com/drona-gyawali/Annales#readme" target="_blank" rel="noreferrer">
                README <ArrowUpRight />
              </a>
            </div>
          </div>
        </div>

        <div className="footer__colophon">
          <span>EXPERIMENTAL · NOT PRODUCTION READY</span>
          <span className="footer__colophon-center">THE FIGURE IS CONCEPTUAL, NOT TELEMETRY</span>
          <span>GO · S3-COMPATIBLE STORAGE</span>
        </div>
      </div>
    </footer>
  );
}
