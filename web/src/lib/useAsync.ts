import { useEffect, useState } from "react";

type AsyncState<T> = {
  loading: boolean;
  data: T | null;
  error: string | null;
};

export function useAsync<T>(load: () => Promise<T>, deps: readonly unknown[]): AsyncState<T> {
  const [state, setState] = useState<AsyncState<T>>({
    loading: true,
    data: null,
    error: null
  });

  useEffect(() => {
    let cancelled = false;
    setState({ loading: true, data: null, error: null });

    load()
      .then((data) => {
        if (!cancelled) {
          setState({ loading: false, data, error: null });
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setState({
            loading: false,
            data: null,
            error: err instanceof Error ? err.message : "请求失败"
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, deps);

  return state;
}
