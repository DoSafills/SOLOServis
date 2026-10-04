import { useEffect, useRef, useState } from "react";
import type { CartItem, Page, Product, StoreOffer } from "./types";
import Header from "./components/common/Header";
import Footer from "./components/common/Footer";
import HomePage from "./pages/home/HomePage";
import ProductCategoriesPage from "./pages/products/ProductCategoriesPage";
import SearchResultsPage from "./pages/search/SearchResultsPage";
import ProductDetailPage from "./pages/products/ProductDetailPage";
import ProductComparisonPage from "./pages/products/ProductComparisonPage";
import ServicesPage from "./pages/services/ServicesPage";
import ServiceDetailPage from "./pages/services/ServiceDetailPage";
import ServiceComparisonPage from "./pages/services/ServiceComparisonPage";
import StoresPage from "./pages/stores/StoresPage";
import FavoritesPage from "./pages/user/FavoritesPage";
import UserPage from "./pages/user/UserPage";
import CartPage from "./pages/user/CartPage";
import { areProductCategoriesCompatible } from "./pages/products/productComparisonUtils";
import ComparisonDock from "./components/products/ComparisonDock";
import { getProductById } from "./services/api/api";
import {
  addCartItem,
  addSearchHistory,
  getCartItems,
  getFavoriteProductIds,
  getFavoriteServiceIds,
  getSearchHistory,
  loginAccount,
  logoutAccount,
  registerAccount,
  removeCartItem,
  restoreAuthSession,
  setFavoriteProduct,
  setFavoriteService,
  updateCartItem,
} from "./services/api/personalization";
import type { AuthSession, AuthUser, SearchHistoryEntry } from "./services/api/personalization";

const SERVICE_COMPARISON_KEY = "soloservice.service-comparison";
const PRODUCT_COMPARISON_PRODUCTS_KEY = "soloservice.product-comparison-products";
const GUEST_CART_KEY = "soloservice.guest-cart";

interface StoredCartEntry {
  productId: string;
  offerId: number;
  quantity: number;
}

function readStoredIds(key: string): Set<string> {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(key) ?? "[]");
    return Array.isArray(value) && value.every((id) => typeof id === "string")
      ? new Set(value)
      : new Set();
  } catch {
    return new Set();
  }
}

function readGuestCart(): StoredCartEntry[] {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(GUEST_CART_KEY) ?? "[]");
    if (!Array.isArray(value)) return [];
    return value.filter(
      (item): item is StoredCartEntry =>
        typeof item === "object" &&
        item !== null &&
        "productId" in item &&
        typeof item.productId === "string" &&
        "offerId" in item &&
        typeof item.offerId === "number" &&
        "quantity" in item &&
        typeof item.quantity === "number" &&
        Number.isInteger(item.quantity) &&
        item.quantity > 0,
    );
  } catch {
    return [];
  }
}

function storeGuestCart(cart: CartItem[]): void {
  const entries = cart.map(({ product, offer, quantity }) => ({
    productId: product.id,
    offerId: offer.offerId,
    quantity,
  }));
  localStorage.setItem(GUEST_CART_KEY, JSON.stringify(entries));
}

function makeCartItem(product: Product, offerId: number, quantity: number): CartItem | null {
  const offer = product.offers.find((candidate) => candidate.offerId === offerId);
  if (!offer) return null;
  return { product, offer, quantity, cartItemId: offerId };
}

export default function App() {
  const [page, setPage] = useState<Page>({ id: "home" });
  const [user, setUser] = useState<AuthUser | null>(null);
  const [favorites, setFavorites] = useState<Set<string>>(new Set());
  const [searchHistory, setSearchHistory] = useState<SearchHistoryEntry[]>([]);
  const [compareList, setCompareList] = useState<Set<string>>(() =>
    readStoredIds(SERVICE_COMPARISON_KEY),
  );
  const [productCompareList, setProductCompareList] = useState<Set<string>>(() =>
    readStoredIds(PRODUCT_COMPARISON_PRODUCTS_KEY),
  );
  const [productCompareCategories, setProductCompareCategories] = useState<Record<string, string>>(
    {},
  );
  const [productCompareError, setProductCompareError] = useState<string | null>(null);
  const [cart, setCart] = useState<CartItem[]>([]);
  const [persistenceError, setPersistenceError] = useState<string | null>(null);
  const userId = user?.userId ?? null;
  const activeUserId = useRef<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    localStorage.removeItem("soloservice.guest-key");
    localStorage.removeItem("soloservice.service-favorites");

    void restoreAuthSession()
      .then((session) => {
        if (!cancelled && session) {
          activeUserId.current = session.userId;
          setUser(session);
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setPersistenceError(
            `No se pudo verificar la sesión: ${error instanceof Error ? error.message : "error desconocido"}`,
          );
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    activeUserId.current = userId;
  }, [userId]);

  useEffect(() => {
    let cancelled = false;

    if (!userId) {
      void Promise.all(
        readGuestCart().map(async (entry) => {
          const product = await getProductById(entry.productId);
          return product ? makeCartItem(product, entry.offerId, entry.quantity) : null;
        }),
      )
        .then((items) => {
          if (!cancelled) setCart(items.filter((item): item is CartItem => item !== null));
        })
        .catch((error: unknown) => {
          if (!cancelled) {
            setPersistenceError(
              `No se pudo recuperar la cesta local: ${error instanceof Error ? error.message : "error desconocido"}`,
            );
          }
        });
      return () => {
        cancelled = true;
      };
    }

    const loadAccountData = async () => {
      const [productFavorites, serviceFavorites, cartResult, historyResult] =
        await Promise.allSettled([
          getFavoriteProductIds(userId),
          getFavoriteServiceIds(userId),
          getCartItems(userId),
          getSearchHistory(userId),
        ]);

      if (cancelled) return;
      const failed: string[] = [];
      const loadedFavorites = new Set<string>();
      if (productFavorites.status === "fulfilled") {
        productFavorites.value.forEach((id) => loadedFavorites.add(id));
      } else failed.push("favoritos de productos");
      if (serviceFavorites.status === "fulfilled") {
        serviceFavorites.value.forEach((id) => loadedFavorites.add(id));
      } else failed.push("favoritos de servicios");
      setFavorites(loadedFavorites);

      if (cartResult.status === "fulfilled") setCart(cartResult.value);
      else failed.push("carrito");
      if (historyResult.status === "fulfilled") setSearchHistory(historyResult.value);
      else failed.push("historial");
      setPersistenceError(
        failed.length > 0
          ? `No se pudieron cargar: ${failed.join(", ")}. Inténtalo nuevamente.`
          : null,
      );
    };

    void loadAccountData();
    return () => {
      cancelled = true;
    };
  }, [userId]);

  useEffect(() => {
    localStorage.setItem(SERVICE_COMPARISON_KEY, JSON.stringify([...compareList]));
  }, [compareList]);

  useEffect(() => {
    localStorage.setItem(PRODUCT_COMPARISON_PRODUCTS_KEY, JSON.stringify([...productCompareList]));
  }, [productCompareList]);

  useEffect(() => {
    let cancelled = false;
    void Promise.all([...productCompareList].map((id) => getProductById(id)))
      .then((products) => {
        if (cancelled) return;
        setProductCompareCategories(
          Object.fromEntries(
            products
              .filter((product): product is Product => product !== null)
              .map((product) => [product.id, product.category]),
          ),
        );
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setProductCompareError(
            `No se pudieron cargar los productos comparados: ${error instanceof Error ? error.message : "error desconocido"}`,
          );
        }
      });
    return () => {
      cancelled = true;
    };
  }, [productCompareList]);

  const navigate = (next: Page) => {
    setPage(next);
    if (
      userId &&
      (next.id === "search-products" || next.id === "search-services") &&
      next.query.trim()
    ) {
      const type = next.id === "search-services" ? "service" : "product";
      void addSearchHistory(userId, next.query.trim(), type)
        .then(() => getSearchHistory(userId))
        .then((history) => {
          if (activeUserId.current === userId) setSearchHistory(history);
        })
        .catch((error: unknown) => {
          if (activeUserId.current === userId) {
            setPersistenceError(
              `No se pudo guardar la búsqueda en el historial: ${error instanceof Error ? error.message : "error desconocido"}`,
            );
          }
        });
    }
  };

  const saveAuthenticatedUser = async (session: AuthSession) => {
    const guestCart = readGuestCart();
    let cartSyncFailed = false;
    if (guestCart.length > 0) {
      try {
        const accountCart = await getCartItems(session.userId);
        const offerIds = new Set(accountCart.map((item) => item.offer.offerId));
        for (const item of guestCart) {
          if (offerIds.has(item.offerId)) continue;
          await addCartItem(session.userId, item.productId, item.offerId);
          offerIds.add(item.offerId);
        }
        localStorage.removeItem(GUEST_CART_KEY);
      } catch (error) {
        cartSyncFailed = true;
        setPersistenceError(
          `Iniciaste sesión, pero no se pudo sincronizar la cesta local: ${error instanceof Error ? error.message : "error desconocido"}`,
        );
      }
    }
    localStorage.removeItem("soloservice.service-favorites");
    activeUserId.current = session.userId;
    setUser(session);
    setPage({ id: "user" });
    if (!cartSyncFailed) setPersistenceError(null);
  };

  const handleLogin = async (email: string, password: string) => {
    const session = await loginAccount(email, password);
    await saveAuthenticatedUser(session);
  };

  const handleRegister = async (name: string, email: string, password: string) => {
    const session = await registerAccount(name, email, password);
    await saveAuthenticatedUser(session);
  };

  const handleLogout = async () => {
    activeUserId.current = null;
    try {
      await logoutAccount();
    } catch (error) {
      setPersistenceError(
        `No se pudo cerrar la sesión en el servidor: ${error instanceof Error ? error.message : "error desconocido"}`,
      );
    } finally {
      setUser(null);
      setFavorites(new Set());
      setSearchHistory([]);
      setCart([]);
      setPage({ id: "home" });
    }
  };

  const addToCart = async (product: Product, offer: StoreOffer) => {
    if (offer.offerId <= 0) {
      setPersistenceError(
        "Esta oferta no tiene un identificador válido para guardarla en la cesta.",
      );
      return;
    }

    if (!userId) {
      setCart((previous) => {
        const existing = previous.find((item) => item.offer.offerId === offer.offerId);
        const updated = existing
          ? previous.map((item) =>
              item.offer.offerId === offer.offerId
                ? { ...item, quantity: item.quantity + 1 }
                : item,
            )
          : [...previous, { product, offer, quantity: 1, cartItemId: offer.offerId }];
        storeGuestCart(updated);
        return updated;
      });
      setPersistenceError(null);
      return;
    }

    try {
      await addCartItem(userId, product.id, offer.offerId);
      setCart(await getCartItems(userId));
      setPersistenceError(null);
    } catch (error) {
      setPersistenceError(
        `No se pudo guardar el producto en la cesta: ${error instanceof Error ? error.message : "error desconocido"}`,
      );
    }
  };

  const updateCartQuantity = async (cartItemId: number, quantity: number) => {
    if (!userId) {
      setCart((previous) => {
        const updated =
          quantity <= 0
            ? previous.filter((item) => item.cartItemId !== cartItemId)
            : previous.map((item) =>
                item.cartItemId === cartItemId ? { ...item, quantity } : item,
              );
        storeGuestCart(updated);
        return updated;
      });
      return;
    }
    try {
      if (quantity <= 0) await removeCartItem(userId, cartItemId);
      else await updateCartItem(userId, cartItemId, quantity);
      setCart(await getCartItems(userId));
      setPersistenceError(null);
    } catch (error) {
      setPersistenceError(
        `No se pudo actualizar la cesta: ${error instanceof Error ? error.message : "error desconocido"}`,
      );
    }
  };

  const removeFromCart = async (cartItemId: number) => {
    if (!userId) {
      setCart((previous) => {
        const updated = previous.filter((item) => item.cartItemId !== cartItemId);
        storeGuestCart(updated);
        return updated;
      });
      return;
    }
    try {
      await removeCartItem(userId, cartItemId);
      setCart(await getCartItems(userId));
      setPersistenceError(null);
    } catch (error) {
      setPersistenceError(
        `No se pudo quitar el producto de la cesta: ${error instanceof Error ? error.message : "error desconocido"}`,
      );
    }
  };

  useEffect(() => {
    window.scrollTo({ top: 0, behavior: "smooth" });
  }, [page]);

  const toggleFavorite = async (id: string, kind: "product" | "service" = "product") => {
    if (!userId) {
      setPersistenceError("Inicia sesión o crea una cuenta para guardar tus favoritos.");
      setPage({ id: "user" });
      return;
    }

    const isFavorite = favorites.has(id);
    try {
      if (kind === "service") await setFavoriteService(userId, id, !isFavorite);
      else await setFavoriteProduct(userId, id, !isFavorite);
      setFavorites((previous) => {
        const next = new Set(previous);
        if (isFavorite) next.delete(id);
        else next.add(id);
        return next;
      });
      setPersistenceError(null);
    } catch (error) {
      setPersistenceError(
        `No se pudo actualizar el favorito: ${error instanceof Error ? error.message : "error desconocido"}`,
      );
    }
  };

  const toggleCompare = async (id: string) => {
    const isComparing = compareList.has(id);
    if (!isComparing && compareList.size >= 3) {
      setPersistenceError("Solo puedes comparar hasta 3 servicios a la vez.");
      return;
    }
    setCompareList((previous) => {
      const next = new Set(previous);
      if (isComparing) next.delete(id);
      else next.add(id);
      return next;
    });
    setPersistenceError(null);
  };

  const toggleProductCompare = async (id: string, category: string) => {
    if (productCompareList.has(id)) {
      setProductCompareList((previous) => {
        const next = new Set(previous);
        next.delete(id);
        return next;
      });
      setProductCompareCategories((previous) => {
        const next = { ...previous };
        delete next[id];
        return next;
      });
      setProductCompareError(null);
      return;
    }
    if (productCompareList.size >= 3) {
      setProductCompareError("Solo puedes comparar hasta 3 productos a la vez.");
      return;
    }
    const incompatibleCategory = [...productCompareList]
      .map((productId) => productCompareCategories[productId])
      .find(
        (selectedCategory) => !areProductCategoriesCompatible(selectedCategory ?? "", category),
      );
    if (incompatibleCategory) {
      setProductCompareError(
        `No se agregó el producto: “${category}” es una categoría diferente de “${incompatibleCategory}”. Selecciona productos de la misma categoría o compatibles.`,
      );
      return;
    }
    setProductCompareList((previous) => new Set(previous).add(id));
    setProductCompareCategories((previous) => ({ ...previous, [id]: category }));
    setProductCompareError(null);
  };

  const clearProductComparison = async () => {
    setProductCompareList(new Set());
    setProductCompareCategories({});
    setProductCompareError(null);
  };

  const sharedProps = {
    navigate,
    favorites,
    compareList,
    productCompareList,
    onAddToCart: addToCart,
    onToggleFavorite: toggleFavorite,
    onToggleCompare: toggleCompare,
    onToggleProductCompare: toggleProductCompare,
  };

  const renderPage = () => {
    switch (page.id) {
      case "home":
        return <HomePage {...sharedProps} />;
      case "product-categories":
        return <ProductCategoriesPage navigate={navigate} />;
      case "search-products":
        return (
          <SearchResultsPage
            key={`${page.category ?? ""}:${page.productGroup ?? ""}:${page.query}`}
            {...sharedProps}
            query={page.query}
            category={page.category}
            productGroup={page.productGroup}
          />
        );
      case "product-detail":
        return (
          <ProductDetailPage
            productId={page.productId}
            navigate={navigate}
            isFavorite={favorites.has(page.productId)}
            isComparing={productCompareList.has(page.productId)}
            onToggleFavorite={toggleFavorite}
            onToggleCompare={toggleProductCompare}
            onAddToCart={addToCart}
          />
        );
      case "product-comparison":
        return (
          <ProductComparisonPage
            productIds={page.productIds}
            navigate={navigate}
            onAddToCart={addToCart}
          />
        );
      case "search-services":
        return <ServicesPage {...sharedProps} query={page.query} />;
      case "service-detail":
        return (
          <ServiceDetailPage
            serviceId={page.serviceId}
            navigate={navigate}
            isFavorite={favorites.has(page.serviceId)}
            isComparing={compareList.has(page.serviceId)}
            onToggleFavorite={toggleFavorite}
            onToggleCompare={toggleCompare}
          />
        );
      case "service-comparison":
        return <ServiceComparisonPage serviceIds={page.serviceIds} navigate={navigate} />;
      case "stores":
        return (
          <StoresPage
            navigate={navigate}
            favorites={favorites}
            compareList={compareList}
            onToggleFavorite={toggleFavorite}
            onToggleCompare={toggleCompare}
          />
        );
      case "store-detail":
        return (
          <StoresPage
            navigate={navigate}
            storeId={page.storeId}
            favorites={favorites}
            compareList={compareList}
            onToggleFavorite={toggleFavorite}
            onToggleCompare={toggleCompare}
          />
        );
      case "favorites":
        return (
          <FavoritesPage
            navigate={navigate}
            favorites={favorites}
            authenticated={Boolean(user)}
            onToggleFavorite={toggleFavorite}
          />
        );
      case "user":
        return (
          <UserPage
            navigate={navigate}
            user={user}
            favoritesCount={favorites.size}
            history={searchHistory}
            onLogin={handleLogin}
            onRegister={handleRegister}
            onLogout={handleLogout}
          />
        );
      case "cart":
        return (
          <CartPage
            cart={cart}
            navigate={navigate}
            onUpdateQuantity={updateCartQuantity}
            onRemove={removeFromCart}
          />
        );
      default:
        return <HomePage {...sharedProps} />;
    }
  };

  return (
    <div style={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <Header
        navigate={navigate}
        currentPage={page}
        favCount={favorites.size}
        cartCount={cart.reduce((sum, item) => sum + item.quantity, 0)}
      />

      <main style={{ flex: 1 }}>{renderPage()}</main>

      {productCompareList.size > 0 && (
        <div className="fixed bottom-4 left-4 z-50">
          <ComparisonDock
            productIds={[...productCompareList]}
            onClear={clearProductComparison}
            onRemove={(product) => toggleProductCompare(product.id, product.category)}
            onNavigate={navigate}
          />
          {productCompareError && (
            <p
              role="alert"
              className="mt-2 max-w-[min(22rem,calc(100vw-2rem))] rounded-xl border border-amber-300/25 bg-slate-950/95 px-3 py-2 text-[11px] font-medium text-amber-200 shadow-lg"
            >
              {productCompareError}
            </p>
          )}
        </div>
      )}

      {persistenceError && (
        <div
          role="alert"
          className="fixed right-4 top-20 z-[60] max-w-md rounded-xl border border-rose-300/30 bg-slate-950/95 px-4 py-3 text-sm text-rose-100 shadow-xl"
        >
          <div className="flex items-start gap-3">
            <p className="flex-1">{persistenceError}</p>
            <button
              type="button"
              onClick={() => setPersistenceError(null)}
              aria-label="Cerrar aviso"
              className="font-bold text-rose-200 hover:text-white"
            >
              ×
            </button>
          </div>
        </div>
      )}

      <Footer navigate={navigate} />
    </div>
  );
}
