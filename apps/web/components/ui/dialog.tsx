import React from 'react';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import { cn } from '../../lib/cn';

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;

export const DialogContent = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content> & { title: React.ReactNode; description?: React.ReactNode }
>(function DialogContent({ className, children, title, description, ...props }, ref) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/50 data-[state=open]:animate-in data-[state=open]:fade-in" />
      <DialogPrimitive.Content
        ref={ref}
        className={cn(
          'fixed left-1/2 top-1/2 z-50 w-full max-w-lg -translate-x-1/2 -translate-y-1/2',
          'max-h-[calc(100dvh-2rem)] overflow-y-auto overscroll-contain',
          'rounded-lg border border-line bg-surface-overlay p-5 shadow-token-lg focus:outline-none',
          className,
        )}
        {...props}
      >
        <div className="mb-3 pr-8">
          <DialogPrimitive.Title className="text-sm font-semibold text-fg">{title}</DialogPrimitive.Title>
          {description ? (
            <DialogPrimitive.Description className="mt-1 text-xs text-fg-muted">
              {description}
            </DialogPrimitive.Description>
          ) : null}
        </div>
        {children}
        <DialogPrimitive.Close
          className="absolute right-4 top-4 rounded-md p-1 text-fg-muted transition-colors hover:bg-surface-sunken hover:text-fg focus:outline-none focus-visible:ring-2 focus-visible:ring-line-focus"
          aria-label="Close"
        >
          <X className="h-4 w-4" />
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
});

export function DialogFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('mt-4 flex justify-end gap-2', className)} {...props} />;
}
