import { useEffect, useState } from "react";
import type { Product } from "../types";
import { getProducts, type ApiProduct } from "../Services/api/products";

export type SortOption = "relevance" | "price-asc" | "price-desc" | "rating";

const PER_PAGE = 6;

function toProduct(product: ApiProduct): Product {
  return {
    id: product.id,
    name: product.name,
    brand: product.brand,
    model: product.model,
    category: product.category,
    subcategory: "",
    image: "",
    images: [],
    description: product.description,
    rating: product.rating,
    reviewCount: product.reviewCount,
    specs: product.model ? { Modelo: product.model } : { Modelo: "" },
    offers: [],
    priceHistory: [],
    offerPriceHistory: [],
    tags: [],
  };
}

export function useSearchResults(query: string) {
  const [apiProducts, setApiProducts] = useState<ApiProduct[]>([]);
  const [sort, setSort] = useState<SortOption>("relevance");
  const [priceMin, setPriceMin] = useState("");
  const [priceMax, setPriceMax] = useState("");
  const [selectedBrands, setSelectedBrands] = useState<Set<string>>(new Set());
  const [availableOnly, setAvailableOnly] = useState(false);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [page, setPage] = useState(1);

  useEffect(() => {
    let cancelled = false;

    getProducts()
      .then((products) => {
        if (!cancelled) setApiProducts(products);
      })
      .catch(console.error);

    return () => {
      cancelled = true;
    };
  }, []);

  const products = apiProducts.map(toProduct);
  const brands = [...new Set(products.map((product) => product.brand))];
  const normalizedQuery = query.toLowerCase();

  const filtered = products
    .filter((product) => {
      if (
        normalizedQuery &&
        !product.name.toLowerCase().includes(normalizedQuery) &&
        !product.brand.toLowerCase().includes(normalizedQuery) &&
        !product.category.toLowerCase().includes(normalizedQuery)
      ) {
        return false;
      }

      return selectedBrands.size === 0 || selectedBrands.has(product.brand);
    })
    .sort((a, b) => (sort === "rating" ? b.rating - a.rating : 0));

  const paginated = filtered.slice((page - 1) * PER_PAGE, page * PER_PAGE);

  const toggleBrand = (brand: string) => {
    setSelectedBrands((previous) => {
      const next = new Set(previous);
      if (next.has(brand)) next.delete(brand);
      else next.add(brand);
      return next;
    });
    setPage(1);
  };

  const clearFilters = () => {
    setSelectedBrands(new Set());
    setPriceMin("");
    setPriceMax("");
    setAvailableOnly(false);
  };

  return {
    brands,
    filtered,
    paginated,
    perPage: PER_PAGE,
    page,
    setPage,
    sort,
    setSort,
    priceMin,
    setPriceMin,
    priceMax,
    setPriceMax,
    selectedBrands,
    availableOnly,
    setAvailableOnly,
    filtersOpen,
    setFiltersOpen,
    toggleBrand,
    clearFilters,
  };
}