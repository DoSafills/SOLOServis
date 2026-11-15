import { useEffect, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

export interface SessionStorageCodec<T> {
  decode: (value: unknown) => T | undefined;
  encode: (value: T) => unknown;
}

export function useSessionStorageState<T>(
  key: string,
  initialValue: () => T,
  codec: SessionStorageCodec<T>,
): [T, Dispatch<SetStateAction<T>>] {
  const [value, setValue] = useState<T>(() => {
    try {
      const saved = sessionStorage.getItem(key);
      if (saved !== null) {
        const decoded = codec.decode(JSON.parse(saved));
        if (decoded !== undefined) return decoded;
      }
    } catch (error) {
      console.error(`No se pudo restaurar el estado de página "${key}".`, error);
    }
    return initialValue();
  });

  useEffect(() => {
    try {
      sessionStorage.setItem(key, JSON.stringify(codec.encode(value)));
    } catch (error) {
      console.error(`No se pudo guardar el estado de página "${key}".`, error);
    }
  }, [codec, key, value]);

  return [value, setValue];
}

export const stringSessionCodec: SessionStorageCodec<string> = {
  decode: (value) => (typeof value === "string" ? value : undefined),
  encode: (value) => value,
};

export const positiveIntegerSessionCodec: SessionStorageCodec<number> = {
  decode: (value) =>
    typeof value === "number" && Number.isInteger(value) && value > 0 ? value : undefined,
  encode: (value) => value,
};

export const nullableStringSessionCodec: SessionStorageCodec<string | null> = {
  decode: (value) => (value === null || typeof value === "string" ? value : undefined),
  encode: (value) => value,
};

export const nullablePositiveIntegerSessionCodec: SessionStorageCodec<number | null> = {
  decode: (value) =>
    value === null || (typeof value === "number" && Number.isInteger(value) && value > 0)
      ? value
      : undefined,
  encode: (value) => value,
};
