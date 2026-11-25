import { SiteFooter, SiteHeader } from "@/components/SiteChrome";
import { Hero } from "@/components/Hero";
import { WorkingNotes } from "@/components/WorkingNotes";

export default function Home() {
  return (
    <>
      <a className="skip-link" href="#main">
        Skip to content
      </a>
      <SiteHeader />
      <main id="main">
        <Hero />
        <WorkingNotes />
      </main>
      <SiteFooter />
    </>
  );
}
