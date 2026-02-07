import { isApiError } from '../../lib/api/client';

interface ErrorStateProps {
  error: Error;
  onRetry: () => void;
}

export function ErrorState({ error, onRetry }: ErrorStateProps) {
  const message = isApiError(error)
    ? `${error.message} (status ${error.status}, code ${error.code})`
    : error.message;

  return (
    <div className="async-state">
      <h3>Could not load data</h3>
      <p>{message}</p>
      <button type="button" className="btn btn-primary" onClick={onRetry}>
        Retry
      </button>
    </div>
  );
}
