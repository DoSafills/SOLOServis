import { API_URLS } from "./config";
import { request } from "./personalization";

// Debe coincidir con assistant/config/limits.json → question.maxLength.
export const ASSISTANT_QUESTION_MAX_LENGTH = 300;

export interface ApiAssistantAnswer {
  answer: string;
  sources: string[];
  provider: string | null;
  model: string | null;
  cached: boolean;
}

interface ApiAssistantAnswerBody {
  answer?: unknown;
  sources?: unknown;
  provider?: unknown;
  model?: unknown;
  cached?: unknown;
}

/**
 * Pregunta al asistente de comparación (Comparison API).
 *
 * Solo se envían los IDs de los productos comparados y la pregunta: el
 * contexto con precios, atributos y especificaciones lo arma el backend desde
 * PostgreSQL. El frontend nunca envía datos de producto.
 */
export async function askComparisonAssistant(
  productIds: string[],
  question: string,
): Promise<ApiAssistantAnswer> {
  const body = await request<ApiAssistantAnswerBody>(
    API_URLS.comparison,
    "/comparisons/assistant",
    {
      method: "POST",
      body: JSON.stringify({ productIds, question }),
    },
  );

  if (typeof body.answer !== "string" || body.answer.trim().length === 0) {
    throw new Error("El asistente devolvió una respuesta vacía.");
  }

  return {
    answer: body.answer.trim(),
    sources: Array.isArray(body.sources)
      ? body.sources.filter((source): source is string => typeof source === "string")
      : [],
    provider: typeof body.provider === "string" ? body.provider : null,
    model: typeof body.model === "string" ? body.model : null,
    cached: body.cached === true,
  };
}
