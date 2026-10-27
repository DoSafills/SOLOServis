import type { CategoryAttribute } from "../../services/api/categories";

export type DynamicFieldValue = string | number | boolean | undefined;
export type DynamicFormValues = Record<string, DynamicFieldValue>;
export type DynamicFieldErrors = Record<string, string>;

function validateAttribute(attribute: CategoryAttribute, value: DynamicFieldValue): string | null {
  if (value === undefined || value === "")
    return attribute.required ? "Este campo es obligatorio." : null;
  if (attribute.dataType === "boolean" && typeof value !== "boolean") return "Selecciona Sí o No.";
  if (attribute.dataType === "string" && typeof value !== "string")
    return "Ingresa un texto válido.";
  if (attribute.dataType === "enum" && !attribute.allowedValues.includes(String(value)))
    return `Selecciona una opción válida: ${attribute.allowedValues.join(", ")}.`;
  if (attribute.dataType === "integer" && (!Number.isInteger(value) || typeof value !== "number"))
    return "Ingresa un número entero.";
  if (attribute.dataType === "decimal" && (typeof value !== "number" || !Number.isFinite(value)))
    return "Ingresa un número válido.";
  if (
    (attribute.dataType === "integer" || attribute.dataType === "decimal") &&
    typeof value === "number"
  ) {
    if (attribute.min !== null && value < attribute.min)
      return `Debe ser mayor o igual a ${attribute.min}.`;
    if (attribute.max !== null && value > attribute.max)
      return `Debe ser menor o igual a ${attribute.max}.`;
  }
  if (
    attribute.maxLength !== null &&
    typeof value === "string" &&
    value.length > attribute.maxLength
  )
    return `Máximo ${attribute.maxLength} caracteres.`;
  return null;
}

export function validateDynamicAttributes(
  attributes: CategoryAttribute[],
  values: DynamicFormValues,
): DynamicFieldErrors {
  return Object.fromEntries(
    attributes
      .map(
        (attribute) =>
          [attribute.key, validateAttribute(attribute, values[attribute.key])] as const,
      )
      .filter(([, error]) => error !== null),
  ) as DynamicFieldErrors;
}
