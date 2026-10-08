import { Suspense } from "react";
import { ReadingRoom } from "@/components/reading-room";

export default function ReadPage({ params }: PageProps<"/read/[itemId]">) {
  return (
    <Suspense fallback={<p className="text-muted">Loading…</p>}>
      {params.then(({ itemId }) => (
        <ReadingRoom itemId={itemId} />
      ))}
    </Suspense>
  );
}
