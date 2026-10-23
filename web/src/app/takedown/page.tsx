// The contact comes from web config, inlined at build time. There is no fallback address.
const TAKEDOWN_EMAIL = process.env.NEXT_PUBLIC_TAKEDOWN_EMAIL?.trim();

export default function TakedownPage() {
  return (
    <article className="mx-auto max-w-2xl space-y-6">
      <header>
        <h1 className="font-serif text-3xl font-semibold">Copyright takedown</h1>
        <p className="mt-2 text-muted">
          If a quote or conversation on Serendipity uses your work without permission, send us a notice and we will
          review it.
        </p>
      </header>

      <section className="card space-y-1 p-4">
        <h2 className="font-semibold">Where to send it</h2>
        {TAKEDOWN_EMAIL ? (
          <p>
            Email{" "}
            <a href={`mailto:${TAKEDOWN_EMAIL}`} className="break-all text-accent underline">
              {TAKEDOWN_EMAIL}
            </a>
          </p>
        ) : (
          <p className="text-muted">The takedown contact isn&apos;t configured yet.</p>
        )}
      </section>

      <section className="space-y-2">
        <h2 className="font-semibold">What your notice must contain</h2>
        <ol className="list-decimal space-y-1 pl-5">
          <li>Your name and contact details, and whether you own the rights or act for the rights holder.</li>
          <li>The work you believe is infringed: its title, author and, if it matters, the edition.</li>
          <li>
            A link to each quote or conversation you want removed (for example the reader&apos;s profile page at{" "}
            <code>/u/&lt;username&gt;</code>), and which passage it is.
          </li>
          <li>A statement that you believe in good faith that this use isn&apos;t authorized by you, your agent or the law.</li>
          <li>A statement that the information in your notice is accurate and that you are the rights holder or are authorized to act for them.</li>
          <li>Your physical or electronic signature. A typed full name is enough.</li>
        </ol>
      </section>

      <section className="space-y-2">
        <h2 className="font-semibold">What happens next</h2>
        <p>
          We review every notice. If it holds up, we hide the content from all public pages. The reader who posted it
          still sees it, marked as removed with the reason. Signed-in readers can also use the Report button on a
          public profile.
        </p>
      </section>
    </article>
  );
}
