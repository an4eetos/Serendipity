import Link from "next/link";

export default function Home() {
  return (
    <div className="mx-auto max-w-2xl py-12">
      <h1 className="font-serif text-4xl leading-tight font-semibold sm:text-5xl">
        Read it for real.
        <br />
        Then say something new.
      </h1>
      <p className="mt-6 text-lg text-muted">
        Highlight a passage from your own copy of a book and write your reaction to it. If you want to take it further,
        talk it over with Socrates, Diogenes, Seneca or another character. Your quotes and reactions live on a profile
        that shows what you actually read — and what you think about it.
      </p>
      <div className="mt-8 flex gap-3">
        <Link href="/library" className="btn btn-primary px-5 py-2.5 text-base">
          Open your library
        </Link>
        <Link href="/login" className="btn px-5 py-2.5 text-base">
          Sign in
        </Link>
      </div>

      <section className="mt-16 grid gap-6 sm:grid-cols-3">
        <div>
          <h2 className="font-semibold">Your copy stays yours</h2>
          <p className="mt-1 text-sm text-muted">
            Upload an EPUB or PDF only you can open, or import your Kindle highlights. Books are never shared.
          </p>
        </div>
        <div>
          <h2 className="font-semibold">Your reaction is the point</h2>
          <p className="mt-1 text-sm text-muted">
            A quote goes public only with your own reaction and a citation — your perspective next to the author&apos;s.
          </p>
        </div>
        <div>
          <h2 className="font-semibold">Discuss it, if you like</h2>
          <p className="mt-1 text-sm text-muted">
            Bring your reaction to a character and see how it holds up. They answer you, in their own voice.
          </p>
        </div>
      </section>
    </div>
  );
}
