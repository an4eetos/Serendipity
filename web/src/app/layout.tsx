import type { Metadata } from "next";
import { Inter, Literata } from "next/font/google";
import { Header } from "@/components/header";
import "./globals.css";

const inter = Inter({ variable: "--font-inter", subsets: ["latin", "cyrillic"] });
const literata = Literata({ variable: "--font-literata", subsets: ["latin", "cyrillic"] });

export const metadata: Metadata = {
  title: "Serendipity",
  description: "Prove you read it. Quote it, react to it, argue with Socrates about it.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={`${inter.variable} ${literata.variable} h-full antialiased`}>
      <body className="flex min-h-full flex-col font-sans">
        <Header />
        <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-8">{children}</main>
      </body>
    </html>
  );
}
