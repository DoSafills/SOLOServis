import { useEffect, useState } from "react";
import type { Product } from "../../types";
import type { CategoryAttribute, ProductCategoryDetail } from "../../services/api/categories";
import { getCategoryBySlug, getDynamicCategories } from "../../services/api/categories";
import {
  createProduct,
  updateProduct,
  type CreateProductInput,
  type UpdateProductInput,
} from "../../services/api/product-write";
import {
  validateDynamicAttributes,
  type DynamicFieldErrors,
  type DynamicFieldValue,
  type DynamicFormValues,
} from "./dynamicProductValidation";

interface Props {
  onCreated?: (product: unknown) => void;
  onUpdated?: (product: unknown) => void;
  initialProduct?: Product;
}

export default function DynamicProductForm({ onCreated, onUpdated, initialProduct }: Props) {
  const [categories, setCategories] = useState<ProductCategoryDetail[]>([]);
  const [category, setCategory] = useState<ProductCategoryDetail | null>(null);
  const [values, setValues] = useState<DynamicFormValues>(() =>
    initialProduct
      ? {
          name: initialProduct.name,
          model: initialProduct.model,
          description: initialProduct.description,
          ...initialProduct.attributes,
        }
      : {},
  );
  const [errors, setErrors] = useState<DynamicFieldErrors>({});
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const editing = Boolean(initialProduct);

  useEffect(() => {
    getDynamicCategories()
      .then((items) => Promise.all(items.map((item) => getCategoryBySlug(item.slug))))
      .then(setCategories)
      .catch(() => setMessage("No se pudieron cargar las categorías."));
  }, []);

  const activeCategory =
    category ?? categories.find((item) => item.slug === initialProduct?.categorySlug) ?? null;
  const updateValue = (key: string, value: DynamicFieldValue) => {
    setValues((previous) => ({ ...previous, [key]: value }));
    setErrors((previous) => ({ ...previous, [key]: "" }));
  };
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!activeCategory) return;
    const nextErrors = validateDynamicAttributes(activeCategory.attributes, values);
    if (!values.name || String(values.name).trim() === "")
      nextErrors.name = "El nombre es obligatorio.";
    if (!editing && (!values.imageUrl || !/^https?:\/\//.test(String(values.imageUrl))))
      nextErrors.imageUrl = "Ingresa una URL http(s) válida.";
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;
    setLoading(true);
    setMessage(null);
    try {
      const input = {
        categorySlug: activeCategory.slug,
        name: String(values.name),
        model: String(values.model ?? ""),
        sku: String(values.sku ?? ""),
        description: String(values.description ?? ""),
        attributes: Object.fromEntries(
          activeCategory.attributes
            .filter(
              (attribute) => values[attribute.key] !== undefined && values[attribute.key] !== "",
            )
            .map((attribute) => [attribute.key, values[attribute.key]]),
        ) as CreateProductInput["attributes"],
      };
      const product = editing
        ? await updateProduct(initialProduct!.id, input as UpdateProductInput)
        : await createProduct({ ...input, imageUrl: String(values.imageUrl) });
      setMessage(
        editing ? "Producto actualizado correctamente." : "Producto creado correctamente.",
      );
      if (editing) onUpdated?.(product);
      else onCreated?.(product);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "No se pudo crear el producto.");
    } finally {
      setLoading(false);
    }
  };
  return (
    <form
      onSubmit={submit}
      className="mx-auto max-w-3xl space-y-6 rounded-2xl border border-slate-700 bg-slate-950/80 p-6 text-slate-100"
    >
      <div>
        <h1 className="text-2xl font-bold">{editing ? "Editar producto" : "Nuevo producto"}</h1>
        <p className="mt-1 text-sm text-slate-400">
          Los campos cambian según la categoría seleccionada.
        </p>
      </div>
      <label className="block text-sm font-medium">
        Categoría
        <select
          className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-900 p-3"
          value={activeCategory?.slug ?? ""}
          onChange={(event) => {
            const selected = categories.find((item) => item.slug === event.target.value) ?? null;
            setCategory(selected);
            setValues({});
            setErrors({});
          }}
        >
          <option value="">Selecciona una categoría</option>
          {categories.map((item) => (
            <option key={item.slug} value={item.slug}>
              {item.name}
            </option>
          ))}
        </select>
      </label>
      {activeCategory && (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            {(["name", "model", "sku", "imageUrl"] as const).map((key) => (
              <label key={key} className="block text-sm font-medium">
                {{ name: "Nombre", model: "Modelo", sku: "SKU", imageUrl: "URL de imagen" }[key]}
                <input
                  className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-900 p-3"
                  value={String(
                    values[key] ?? (key === "imageUrl" && editing ? initialProduct?.image : ""),
                  )}
                  onChange={(event) => updateValue(key, event.target.value)}
                />
                {errors[key] && (
                  <span className="mt-1 block text-xs text-rose-300">{errors[key]}</span>
                )}
              </label>
            ))}
          </div>
          <label className="block text-sm font-medium">
            Descripción
            <textarea
              className="mt-2 min-h-24 w-full rounded-lg border border-slate-700 bg-slate-900 p-3"
              value={String(values.description ?? "")}
              onChange={(event) => updateValue("description", event.target.value)}
            />
          </label>
          <div className="grid gap-4 sm:grid-cols-2">
            {activeCategory.attributes.map((attribute) => (
              <DynamicField
                key={attribute.key}
                attribute={attribute}
                value={values[attribute.key] ?? ""}
                error={errors[attribute.key]}
                onChange={(value) => updateValue(attribute.key, value)}
              />
            ))}
          </div>
          {message && (
            <p role="alert" className="rounded-lg border border-slate-700 p-3 text-sm">
              {message}
            </p>
          )}
          <button
            disabled={loading}
            className="rounded-lg bg-cyan-500 px-5 py-3 font-semibold text-slate-950 disabled:opacity-50"
          >
            {loading ? "Guardando..." : "Crear producto"}
          </button>
        </>
      )}
    </form>
  );
}

function DynamicField({
  attribute,
  value,
  error,
  onChange,
}: {
  attribute: CategoryAttribute;
  value: DynamicFieldValue;
  error?: string;
  onChange: (value: DynamicFieldValue) => void;
}) {
  const label = `${attribute.label}${attribute.unit ? ` (${attribute.unit})` : ""}${attribute.required ? " *" : ""}`;
  if (attribute.dataType === "boolean")
    return (
      <label className="flex items-center gap-3 text-sm font-medium">
        <input
          type="checkbox"
          checked={value === true}
          onChange={(event) => onChange(event.target.checked)}
        />
        {label}
      </label>
    );
  if (attribute.dataType === "enum")
    return (
      <label className="block text-sm font-medium">
        {label}
        <select
          className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-900 p-3"
          value={String(value ?? "")}
          onChange={(event) => onChange(event.target.value)}
        >
          <option value="">Selecciona una opción</option>
          {attribute.allowedValues.map((option) => (
            <option key={option}>{option}</option>
          ))}
        </select>
        {error && <span className="mt-1 block text-xs text-rose-300">{error}</span>}
      </label>
    );
  const numeric = attribute.dataType === "integer" || attribute.dataType === "decimal";
  return (
    <label className="block text-sm font-medium">
      {label}
      <input
        type={numeric ? "number" : "text"}
        step={attribute.dataType === "decimal" ? "any" : "1"}
        min={attribute.min ?? undefined}
        max={attribute.max ?? undefined}
        maxLength={attribute.maxLength ?? undefined}
        className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-900 p-3"
        value={String(value ?? "")}
        onChange={(event) =>
          onChange(
            numeric
              ? event.target.value === ""
                ? undefined
                : Number(event.target.value)
              : event.target.value,
          )
        }
      />
      {error && <span className="mt-1 block text-xs text-rose-300">{error}</span>}
    </label>
  );
}
