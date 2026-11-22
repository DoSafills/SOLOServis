import { useEffect, useRef, useState, type FormEvent } from "react";
import {
  ASSISTANT_QUESTION_MAX_LENGTH,
  askComparisonAssistant,
} from "../../services/api/assistant";
import { ApiError } from "../../services/api/personalization";

interface Props {
  productIds: string[];
}

interface ChatMessage {
  id: number;
  role: "user" | "assistant";
  text: string;
  tone?: "error";
}

// Fuente de verdad: assistant/prompts/suggested-questions.json.
const suggestedQuestions = [
  "¿Cuál tiene más capacidad?",
  "¿Cuál tiene mejores materiales?",
  "¿Qué diferencias técnicas importan?",
  "¿Cuál me recomiendas y por qué?",
];

const unavailableText =
  "El asistente todavía no está disponible en este entorno. La comparación sigue funcionando con normalidad.";

export default function ComparisonAssistantCard({ productIds }: Props) {
  const [expanded, setExpanded] = useState(true);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [question, setQuestion] = useState("");
  const [sending, setSending] = useState(false);
  const [unavailable, setUnavailable] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const nextMessageId = useRef(1);

  useEffect(() => {
    const list = listRef.current;
    if (list) list.scrollTop = list.scrollHeight;
  }, [messages, sending, expanded]);

  const sendQuestion = async (rawQuestion: string) => {
    const trimmed = rawQuestion.trim();
    if (!trimmed || sending || unavailable) return;

    if (trimmed.length > ASSISTANT_QUESTION_MAX_LENGTH) {
      setNotice(`La pregunta no puede superar los ${ASSISTANT_QUESTION_MAX_LENGTH} caracteres.`);
      return;
    }

    setNotice(null);
    setQuestion("");
    setMessages((previous) => [
      ...previous,
      { id: nextMessageId.current++, role: "user", text: trimmed },
    ]);
    setSending(true);

    try {
      const result = await askComparisonAssistant(productIds, trimmed);
      setMessages((previous) => [
        ...previous,
        { id: nextMessageId.current++, role: "assistant", text: result.answer },
      ]);
    } catch (error) {
      if (error instanceof ApiError && (error.status === 404 || error.status === 503)) {
        setUnavailable(true);
        return;
      }

      const text =
        error instanceof ApiError && error.status === 429
          ? "Hiciste muchas preguntas seguidas. Espera un momento y vuelve a intentarlo."
          : "No pude responder en este momento. Intenta de nuevo en unos segundos.";
      setMessages((previous) => [
        ...previous,
        { id: nextMessageId.current++, role: "assistant", text, tone: "error" },
      ]);
    } finally {
      setSending(false);
    }
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void sendQuestion(question);
  };

  return (
    <section
      aria-labelledby="comparison-assistant-title"
      className="mt-6 overflow-hidden rounded-[28px] border border-bd bg-surface text-text shadow-[0_24px_70px_rgba(0,0,0,0.4)]"
    >
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((open) => !open)}
        className="flex w-full items-center justify-between gap-3 bg-surface-2 px-4 py-3 text-left transition-colors hover:bg-surface-3 sm:px-5"
      >
        <span className="flex min-w-0 items-center gap-3">
          <span
            aria-hidden="true"
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-prime text-lg text-[#181714]"
          >
            ✦
          </span>
          <span className="min-w-0">
            <span
              id="comparison-assistant-title"
              className="block text-sm font-extrabold text-text"
            >
              Asistente de comparación
            </span>
            <span className="block truncate text-[11px] text-muted">
              Pregunta sobre estos {productIds.length} productos
            </span>
          </span>
        </span>
        <span aria-hidden="true" className="text-lg font-bold text-prime">
          {expanded ? "−" : "+"}
        </span>
      </button>

      {expanded && (
        <div className="space-y-4 p-4 sm:p-5">
          <div
            ref={listRef}
            role="log"
            aria-live="polite"
            aria-label="Conversación con el asistente"
            className="max-h-72 space-y-3 overflow-y-auto pr-1"
          >
            {messages.length === 0 && (
              <p className="text-sm text-muted">
                Te ayudo a decidir entre los productos de esta comparación. Solo uso los datos que
                ves en la tabla: no invento precios ni características.
              </p>
            )}

            {messages.map((message) => (
              <div
                key={message.id}
                className={message.role === "user" ? "flex justify-end" : "flex justify-start"}
              >
                <p
                  className={`max-w-[85%] whitespace-pre-line rounded-2xl px-3 py-2 text-sm leading-relaxed ${
                    message.role === "user"
                      ? "bg-prime text-[#181714]"
                      : message.tone === "error"
                        ? "border border-danger/40 bg-danger/10 text-danger"
                        : "border border-bd bg-surface-2 text-text"
                  }`}
                >
                  {message.text}
                </p>
              </div>
            ))}

            {sending && (
              <div className="flex justify-start">
                <p
                  role="status"
                  className="rounded-2xl border border-bd bg-surface-2 px-3 py-2 text-sm text-muted"
                >
                  El asistente está escribiendo…
                </p>
              </div>
            )}
          </div>

          <div className="flex flex-wrap gap-2">
            {suggestedQuestions.map((suggestion) => (
              <button
                key={suggestion}
                type="button"
                disabled={sending || unavailable}
                onClick={() => void sendQuestion(suggestion)}
                className="rounded-full border border-bd bg-surface-2 px-3 py-1.5 text-[11px] font-semibold text-text transition hover:border-prime hover:text-prime disabled:cursor-not-allowed disabled:opacity-40"
              >
                {suggestion}
              </button>
            ))}
          </div>

          {unavailable && (
            <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-bd bg-surface-2 px-3 py-2">
              <p className="text-[11px] text-muted">{unavailableText}</p>
              <button
                type="button"
                onClick={() => setUnavailable(false)}
                className="rounded-lg border border-bd px-2.5 py-1 text-[11px] font-semibold text-text transition hover:border-prime hover:text-prime"
              >
                Reintentar
              </button>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-1.5">
            <label htmlFor="comparison-assistant-question" className="sr-only">
              Tu pregunta sobre los productos comparados
            </label>
            <div className="flex gap-2">
              <input
                id="comparison-assistant-question"
                type="text"
                value={question}
                onChange={(event) => {
                  setQuestion(event.target.value);
                  if (notice) setNotice(null);
                }}
                maxLength={ASSISTANT_QUESTION_MAX_LENGTH}
                disabled={sending || unavailable}
                placeholder="Ej: ¿Cuál tiene más capacidad?"
                className="min-w-0 flex-1 rounded-xl border border-bd bg-surface-2 px-3 py-2 text-sm text-text placeholder:text-muted focus:border-prime focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
              />
              <button
                type="submit"
                disabled={sending || unavailable || question.trim().length === 0}
                className="rounded-xl bg-prime px-4 py-2 text-sm font-extrabold text-[#181714] shadow-md shadow-black/20 transition hover:bg-prime-dark disabled:cursor-not-allowed disabled:opacity-40"
              >
                Enviar
              </button>
            </div>
            <div className="flex items-center justify-between gap-2">
              {notice ? (
                <p role="alert" className="text-[10px] text-danger">
                  {notice}
                </p>
              ) : (
                <span />
              )}
              <p className="text-right text-[10px] text-muted">
                {question.length}/{ASSISTANT_QUESTION_MAX_LENGTH}
              </p>
            </div>
          </form>

          <p className="text-[10px] leading-relaxed text-muted">
            Respuesta generada a partir de los datos de esta comparación. Verifica en la ficha del
            producto. No se envían tus datos personales.
          </p>
        </div>
      )}
    </section>
  );
}
