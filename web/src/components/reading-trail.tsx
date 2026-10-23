// A strip across a book, 0–100 %, with one dot per quote that has a position.
// Plain component (no hooks, no client directive): the server-rendered profile uses
// it with hrefs, and the client reading room with onSelect.

export type TrailDot = { id: string; position: number; href?: string };

type Props = {
  dots: TrailDot[];
  // Quotes without a position in the book, such as Kindle highlights.
  unplaced: number;
  onSelect?: (id: string) => void;
  activeId?: string;
};

// percent clamps a 0–1 position to 0–100.
function percent(position: number): number {
  if (!Number.isFinite(position)) return 0;
  return Math.min(1, Math.max(0, position)) * 100;
}

export function ReadingTrail({ dots, unplaced, onSelect, activeId }: Props) {
  if (dots.length === 0 && unplaced <= 0) return null;
  return (
    <div className="w-full">
      {/* The padding keeps dots at 0 % and 100 % inside the strip's box. */}
      <div className="px-2.5">
        <div role="group" aria-label="Reading trail" className="relative h-5">
          <div className="absolute inset-x-0 top-1/2 h-1 -translate-y-1/2 rounded-full bg-line" />
          {dots.map((d) => {
            const pct = percent(d.position);
            const label = `Quote at ${Math.round(pct)}%`;
            const active = d.id === activeId;
            const box =
              "group absolute top-1/2 flex size-5 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full";
            const focus = "focus-visible:outline-2 focus-visible:outline-accent";
            const dot = (
              <span
                className={`size-3 rounded-full transition ${
                  active ? "bg-ink ring-2 ring-accent" : "bg-accent group-hover:bg-ink"
                }`}
              />
            );
            const style = { left: `${pct}%` };
            if (onSelect) {
              return (
                <button
                  key={d.id}
                  type="button"
                  onClick={() => onSelect(d.id)}
                  aria-label={label}
                  aria-current={active || undefined}
                  title={label}
                  className={`${box} ${focus} cursor-pointer`}
                  style={style}
                >
                  {dot}
                </button>
              );
            }
            if (d.href) {
              return (
                <a
                  key={d.id}
                  href={d.href}
                  aria-label={label}
                  title={label}
                  className={`${box} ${focus}`}
                  style={style}
                >
                  {dot}
                </a>
              );
            }
            return (
              <span key={d.id} role="img" aria-label={label} title={label} className={box} style={style}>
                {dot}
              </span>
            );
          })}
        </div>
      </div>
      <div className="mt-1 flex justify-between gap-2 text-xs text-muted">
        <span>0%</span>
        {unplaced > 0 && (
          <span title="Quotes without a position in the book, such as Kindle highlights">{unplaced} unplaced</span>
        )}
        <span>100%</span>
      </div>
    </div>
  );
}
