import React, { type HTMLAttributes, type ReactNode } from 'react';
import { cn } from '../../lib/cn';

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn('rounded-lg border border-line bg-surface-base shadow-token-sm', className)}
      {...props}
    />
  );
}

export function CardHeader({
  className,
  title,
  description,
  actions,
  ...props
}: Omit<HTMLAttributes<HTMLDivElement>, 'title'> & { title?: ReactNode; description?: ReactNode; actions?: ReactNode }) {
  return (
    <div className={cn('flex flex-col gap-3 px-4 pt-4 pb-3 sm:flex-row sm:items-start sm:justify-between', className)} {...props}>
      <div className="min-w-0">
        {title ? <h3 className="text-sm font-semibold text-fg leading-5">{title}</h3> : null}
        {description ? <p className="mt-0.5 text-xs text-fg-muted leading-4">{description}</p> : null}
      </div>
      {actions ? <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto sm:shrink-0">{actions}</div> : null}
    </div>
  );
}

export function CardContent({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('px-4 pb-4', className)} {...props} />;
}

export function CardFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn('flex items-center gap-2 border-t border-line px-4 py-3', className)} {...props} />
  );
}
