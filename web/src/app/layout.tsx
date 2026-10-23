import type { Metadata } from "next";
import { Inter, Literata } from "next/font/google";
import Link from "next/link";
import { Header } from "@/components/header";
import "./globals.css";

const inter = Inter({ variable: "--font-inter", subsets: ["latin", "cyrillic"] });
const literata = Literata({ variable: "--font-literata", subsets: ["latin", "cyrillic"] });

export const metadata: Metadata = {
  title: "Serendipity",
  description: "Prove you read it. Quote it, react to it, and talk your reaction over with Socrates.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={`${inter.variable} ${literata.variable} h-full antialiased`}>
      <body className="flex min-h-full flex-col font-sans">
        <Header />
        <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-8">{children}</main>
        <footer className="border-t border-line">
          <nav className="mx-auto flex max-w-5xl flex-wrap gap-x-5 gap-y-1 px-4 py-4 text-sm text-muted">
            <Link href="/terms" className="hover:text-ink">
              Terms
            </Link>
            <Link href="/takedown" className="hover:text-ink">
              Copyright takedown
            </Link>
          </nav>
        </footer>
      </body>
    </html>
  );
}
