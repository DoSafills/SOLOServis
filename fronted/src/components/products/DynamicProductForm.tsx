import { useEffect, useState } from "react";
import type { Product } from "../../types";
import type { CategoryAttribute, ProductCategoryDetail } from "../../services/api/categories";
import { getCategoryBySlug, getDynamicCategories } from "../../services/api/categories";
import {
  checkProductDuplicates,
  createProduct,
  updateProduct,
  type CreateProductInput,
  type DuplicateCandidate,
  type DuplicateCandidateInput,
  type UpdateProductInput,
} from "../../services/api/product-write";
import { ApiError } from "../../services/api/personalization";
import { getSearchBrands, type SearchBrand } from "../../services/api/search";
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

type PendingWrite =
  | { kind: "create"; input: CreateProductInput }
  | { kind: "update"; id: string; input: UpdateProductInput };

export default function DynamicProductForm({ onCreated, onUpdated, initialProduct }: Props) {
  const [categories, setCategories] = useState<ProductCategoryDetail[]>([]);
  const [brands, setBrands] = useState<SearchBrand[]>([]);
  const [brandId, setBrandId] = useState(String(initialProduct?.brandId ?? ""));
  const [category, setCategory] = useState<ProductCategoryDetail | null>(null);
  const [categoriesLoading, setCategoriesLoading] = useState(true);
  const [categoriesError, setCategoriesError] = useState<string | null>(null);
  const [brandsLoading, setBrandsLoading] = useState(true);
  const [brandsError, setBrandsError] = useState<string | null>(null);
  const [categoryLoadAttempt, setCategoryLoadAttempt] = useState(0);
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
  const [duplicateCandidates, setDuplicateCandidates] = useState<DuplicateCandidate[]>([]);
  const [pendingWrite, setPendingWrite] = useState<PendingWrite | null>(null);
  const editing = Boolean(initialProduct);

  useEffect(() => {
    let cancelled = false;
    setCategoriesLoading(true);
    setCategoriesError(null);
    getDynamicCategories()
      .then((items) => Promise.all(items.map((item) => getCategoryBySlug(item.slug))))
      .then((items) => {
        if (!cancelled) setCategories(items);
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setCategoriesError(
            error instanceof Error ? error.message : "No se pudieron cargar las categorías.",
          );
        }
      })
      .finally(() => {
        if (!cancelled) setCategoriesLoading(false);
      });
    setBrandsLoading(true);
    setBrandsError(null);
    getSearchBrands()
      .then((items) => {
        if (!cancelled) setBrands(items);
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setBrandsError(
            error instanceof Error ? error.message : "No se pudieron cargar las marcas.",
          );
        }
      })
      .finally(() => {
        if (!cancelled) setBrandsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [categoryLoadAttempt]);

  const activeCategory =
    category ?? categories.find((item) => item.slug === initialProduct?.categorySlug) ?? null;
  const updateValue = (key: string, value: DynamicFieldValue) => {
    setValues((previous) => ({ ...previous, [key]: value }));
    setErrors((previous) => {
      const next = { ...previous };
      delete next[key];
      return next;
    });
    setDuplicateCandidates([]);
    setPendingWrite(null);
  };

  const savePendingWrite = async (pending: PendingWrite, checkForDuplicates: boolean) => {
    setLoading(true);
    setMessage(null);
    try {
      if (checkForDuplicates) {
        const input = pending.input;
        const checkInput: DuplicateCandidateInput = {
          categorySlug: input.categorySlug,
          ...(input.brandId ? { brandId: input.brandId } : {}),
          ...(input.sku ? { sku: input.sku } : {}),
          ...(input.model ? { model: input.model } : {}),
          ...(pending.kind === "update" ? { excludeProductId: pending.id } : {}),
        };
        const candidates = await checkProductDuplicates(checkInput);
        if (candidates.length > 0) {
          setDuplicateCandidates(candidates);
          setPendingWrite(pending);
          setMessage(
            candidates.some((candidate) => candidate.matchType === "sku")
              ? "Este SKU ya pertenece a un producto. Agrega la oferta a ese producto en lugar de crear otro."
              : "Encontramos posibles coincidencias. Revísalas antes de continuar; no se fusionará ningún producto.",
          );
          return;
        }
        if (!input.sku && !input.brandId && input.model) {
          setMessage(
            "No se pudo revisar duplicados sin SKU: selecciona una marca para comparar categoría, marca y modelo.",
          );
        }
      }

      const product =
        pending.kind === "create"
          ? await createProduct(pending.input)
          : await updateProduct(pending.id, pending.input);
      setDuplicateCandidates([]);
      setPendingWrite(null);
      setMessage(
        pending.kind === "update"
          ? "Producto actualizado correctamente."
          : "Producto creado correctamente.",
      );
      if (pending.kind === "update") onUpdated?.(product);
      else onCreated?.(product);
    } catch (error) {
      if (error instanceof ApiError) {
        setErrors((previous) => ({
          ...previous,
          ...Object.fromEntries(error.fieldErrors.map(({ field, message }) => [field, message])),
        }));
      }
      setMessage(error instanceof Error ? error.message : "No se pudo guardar el producto.");
    } finally {
      setLoading(false);
    }
  };

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!activeCategory) {
      setMessage("Selecciona una categoría antes de guardar el producto.");
      return;
    }
    const nextErrors = validateDynamicAttributes(activeCategory.attributes, values);
    const name = String(values.name ?? "").trim();
    const model = String(values.model ?? "").trim();
    const sku = String(values.sku ?? "").trim();
    const description = String(values.description ?? "").trim();
    const imageUrl = String(values.imageUrl ?? "").trim();
    const parsedBrandId = brandId === "" ? undefined : Number(brandId);
    if (parsedBrandId !== undefined && (!Number.isSafeInteger(parsedBrandId) || parsedBrandId <= 0))
      nextErrors.brandId = "Selecciona una marca válida.";
    if (!name) nextErrors.name = "El nombre es obligatorio.";
    else if ([...name].length > 255) nextErrors.name = "Máximo 255 caracteres.";
    if ([...model].length > 150) nextErrors.model = "Máximo 150 caracteres.";
    if ([...sku].length > 100) nextErrors.sku = "Máximo 100 caracteres.";
    if ([...description].length > 5000) nextErrors.description = "Máximo 5000 caracteres.";
    if (!editing && !isHttpURL(imageUrl)) nextErrors.imageUrl = "Ingresa una URL http(s) válida.";
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;
    const input: UpdateProductInput = {
      categorySlug: activeCategory.slug,
      ...(parsedBrandId ? { brandId: parsedBrandId } : {}),
      name,
      model,
      sku,
      description,
      attributes: Object.fromEntries(
        activeCategory.attributes
          .filter((attribute) => {
            const value = values[attribute.key];
            return (
              value !== undefined &&
              value !== "" &&
              !(typeof value === "string" && value.trim() === "")
            );
          })
          .map((attribute) => [attribute.key, values[attribute.key]]),
      ) as CreateProductInput["attributes"],
    };
    const pending: PendingWrite = editing
      ? { kind: "update", id: initialProduct!.id, input }
      : { kind: "create", input: { ...input, imageUrl } };
    await savePendingWrite(pending, true);
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
      {categoriesLoading && <p role="status">Cargando categorías...</p>}
      {categoriesError && (
        <div role="alert" className="rounded-lg border border-rose-700 p-3 text-sm text-rose-200">
          <p>{categoriesError}</p>
          <button
            type="button"
            className="mt-2 underline"
            onClick={() => setCategoryLoadAttempt((attempt) => attempt + 1)}
          >
            Reintentar
          </button>
        </div>
      )}
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
            Marca
            <select
              value={brandId}
              onChange={(event) => {
                setBrandId(event.target.value);
                setDuplicateCandidates([]);
                setPendingWrite(null);
                setErrors((previous) => {
                  const next = { ...previous };
                  delete next.brandId;
                  return next;
                });
              }}
              className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-900 p-3"
            >
              <option value="">Sin marca</option>
              {initialProduct?.brandId &&
                !brands.some((item) => item.id === initialProduct.brandId) && (
                  <option value={initialProduct.brandId}>{initialProduct.brand}</option>
                )}
              {brands.map((brand) => (
                <option key={brand.id} value={brand.id}>
                  {brand.name}
                </option>
              ))}
            </select>
            {brandsLoading && (
              <span className="mt-1 block text-xs text-slate-400">Cargando marcas...</span>
            )}
            {brandsError && (
              <span role="alert" className="mt-1 block text-xs text-rose-300">
                {brandsError}
              </span>
            )}
            {errors.brandId && (
              <span className="mt-1 block text-xs text-rose-300">{errors.brandId}</span>
            )}
          </label>
          <label className="block text-sm font-medium">
            Descripción
            <textarea
              className="mt-2 min-h-24 w-full rounded-lg border border-slate-700 bg-slate-900 p-3"
              value={String(values.description ?? "")}
              onChange={(event) => updateValue("description", event.target.value)}
            />
            {errors.description && (
              <span className="mt-1 block text-xs text-rose-300">{errors.description}</span>
            )}
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
          {duplicateCandidates.length > 0 && (
            <div
              role="alert"
              className="space-y-3 rounded-lg border border-amber-500/60 p-4 text-sm"
            >
              <ul className="list-inside list-disc space-y-1">
                {duplicateCandidates.map((candidate) => (
                  <li key={candidate.id}>
                    <strong>{candidate.name}</strong> — {candidate.brand} {candidate.model} ·{" "}
                    {candidate.offerCount} ofertas en tiendas
                  </li>
                ))}
              </ul>
              <p>
                Un producto puede tener ofertas distintas en varias tiendas; esta acción no las
                fusiona.
              </p>
              <div className="flex flex-wrap gap-2">
                {pendingWrite &&
                  duplicateCandidates.every((candidate) => candidate.matchType !== "sku") && (
                    <button
                      type="button"
                      disabled={loading}
                      onClick={() => void savePendingWrite(pendingWrite, false)}
                      className="rounded-lg bg-amber-300 px-4 py-2 font-semibold text-slate-950 disabled:opacity-50"
                    >
                      {loading ? "Guardando..." : "Continuar sin fusionar"}
                    </button>
                  )}
                <button
                  type="button"
                  disabled={loading}
                  onClick={() => {
                    setDuplicateCandidates([]);
                    setPendingWrite(null);
                    setMessage(null);
                  }}
                  className="rounded-lg border border-slate-600 px-4 py-2"
                >
                  Cancelar
                </button>
              </div>
            </div>
          )}
          <button
            disabled={loading || categoriesLoading || brandsLoading}
            className="rounded-lg bg-cyan-500 px-5 py-3 font-semibold text-slate-950 disabled:opacity-50"
          >
            {loading ? "Guardando..." : editing ? "Guardar cambios" : "Crear producto"}
          </button>
        </>
      )}
    </form>
  );
}

function isHttpURL(value: string): boolean {
  try {
    const url = new URL(value);
    return (url.protocol === "http:" || url.protocol === "https:") && Boolean(url.hostname);
  } catch {
    return false;
  }
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
      <div>
        <label className="flex items-center gap-3 text-sm font-medium">
          <input
            type="checkbox"
            checked={value === true}
            onChange={(event) => onChange(event.target.checked)}
          />
          {label}
        </label>
        {error && <span className="mt-1 block text-xs text-rose-300">{error}</span>}
      </div>
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
