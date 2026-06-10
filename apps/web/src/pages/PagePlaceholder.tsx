import { type ReactNode } from "react";

interface PagePlaceholderProps {
  title: string;
  description: string;
  children?: ReactNode;
}

/**
 * PagePlaceholder renders a consistent, accessible heading + description for the
 * skeleton pages. Feature content replaces `children` in later issues.
 */
export function PagePlaceholder({
  title,
  description,
  children,
}: PagePlaceholderProps) {
  return (
    <section aria-labelledby="page-heading" className="space-y-4">
      <div className="space-y-1">
        <h1 id="page-heading" className="text-2xl font-semibold tracking-tight">
          {title}
        </h1>
        <p className="text-muted-foreground">{description}</p>
      </div>
      <div className="rounded-lg border border-border p-6 text-sm text-muted-foreground">
        {children ?? "Placeholder — implemented in a later issue."}
      </div>
    </section>
  );
}
