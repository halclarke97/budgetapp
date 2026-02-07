interface LoadingStateProps {
  label?: string;
}

export function LoadingState({ label = 'Loading...' }: LoadingStateProps) {
  return (
    <div className="async-state" role="status" aria-live="polite">
      <div className="loading-spinner" />
      <p>{label}</p>
    </div>
  );
}
