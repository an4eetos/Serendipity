import { Suspense } from "react";
import { QuotePage } from "@/components/quote-page";

export default function Page({ params }: PageProps<"/quotes/[id]">) {
  return (
    <Suspense fallback={<p className="text-muted">Loading…</p>}>
      {params.then(({ id }) => (
        <QuotePage id={id} />
      ))}
    </Suspense>
  );
}
