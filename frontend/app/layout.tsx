import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono, Instrument_Serif } from "next/font/google";
import "./globals.css";

const geist = Geist({
  subsets: ["latin"],
  variable: "--font-geist-sans",
  display: "swap",
});

const geistMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-geist-mono",
  display: "swap",
});

const instrumentSerif = Instrument_Serif({
  subsets: ["latin"],
  weight: "400",
  style: ["normal", "italic"],
  variable: "--font-instrument-serif",
  display: "swap",
});

export const metadata: Metadata = {
  title: "Annales — Fast reads, durable writes",
  description:
    "An experimental RAM-first key-value store in Go, backed by immutable logs, a CAS manifest, snapshots, and S3-compatible object storage.",
  keywords: ["Annales", "Go", "key-value store", "S3", "storage engine", "systems"],
  openGraph: {
    title: "Annales — Fast reads, durable writes",
    description:
      "An experimental RAM-first key-value store that pairs in-memory reads with durable, S3-compatible storage.",
    type: "website",
  },
};

export const viewport: Viewport = {
  themeColor: "#fafaf8",
  colorScheme: "light",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${geist.variable} ${geistMono.variable} ${instrumentSerif.variable}`}>
      <body>{children}</body>
    </html>
  );
}
