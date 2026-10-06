import { useEffect, useState } from "react";

interface FetchState<T> {
  key: string;
  data?: T;
  error?: string;
}

/**
 * Ejecuta `load` y vuelve a hacerlo cada vez que cambia `key`. Descarta las
 * respuestas que llegan después de que la key cambió, así una carga lenta
 * anterior nunca pisa a la actual.
 */
export function useFetch<T>(key: string, load: () => Promise<T>) {
  const [state, setState] = useState<FetchState<T> | null>(null);

  useEffect(() => {
    let cancelled = false;

    load().then(
      (data) => {
        if (!cancelled) setState({ key, data });
      },
      (err: unknown) => {
        if (!cancelled) {
          setState({
            key,
            error: err instanceof Error ? err.message : "Error al cargar los datos",
          });
        }
      },
    );

    return () => {
      cancelled = true;
    };
    // `load` se recrea en cada render; `key` es lo que identifica la carga.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  // Mientras no llegue la respuesta de la key actual, se considera cargando.
  const current = state?.key === key ? state : null;

  return { data: current?.data, error: current?.error, loading: current === null };
}
