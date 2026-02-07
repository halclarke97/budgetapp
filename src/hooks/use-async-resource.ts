import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

type AsyncStatus = 'idle' | 'loading' | 'success' | 'error';

interface AsyncState<TData> {
  status: AsyncStatus;
  data: TData | null;
  error: Error | null;
}

interface UseAsyncResourceResult<TData> extends AsyncState<TData> {
  reload: () => Promise<void>;
}

export function useAsyncResource<TData>(
  fetcher: (signal: AbortSignal) => Promise<TData>,
  dependencies: ReadonlyArray<unknown>
): UseAsyncResourceResult<TData> {
  const [state, setState] = useState<AsyncState<TData>>({
    status: 'idle',
    data: null,
    error: null
  });

  const mountedRef = useRef(true);
  const fetcherRef = useRef(fetcher);

  useEffect(() => {
    fetcherRef.current = fetcher;
  }, [fetcher]);

  useEffect(() => {
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const runFetch = useCallback(
    async (controller: AbortController) => {
      setState((previous) => ({
        status: 'loading',
        data: previous.data,
        error: null
      }));

      try {
        const data = await fetcherRef.current(controller.signal);
        if (mountedRef.current && !controller.signal.aborted) {
          setState({
            status: 'success',
            data,
            error: null
          });
        }
      } catch (error) {
        if (!controller.signal.aborted && mountedRef.current) {
          setState({
            status: 'error',
            data: null,
            error: error instanceof Error ? error : new Error('Unknown error')
          });
        }
      }
    },
    []
  );

  const load = useCallback(async () => {
    const controller = new AbortController();
    await runFetch(controller);
  }, [runFetch]);

  useEffect(() => {
    const controller = new AbortController();
    void runFetch(controller);

    return () => {
      controller.abort();
    };
  }, [runFetch, ...dependencies]);

  return useMemo(
    () => ({
      ...state,
      reload: load
    }),
    [state, load]
  );
}
