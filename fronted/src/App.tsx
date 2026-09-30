import { useState, useEffect } from "react";
import type { CartItem, Page, Product } from "./types";
import Header from "./components/common/Header";
import Footer from "./components/common/Footer";
import HomePage from "./pages/home/HomePage";
import SearchResultsPage from "./pages/search/SearchResultsPage";
import ProductDetailPage from "./pages/products/ProductDetailPage";
import ProductComparisonPage from "./pages/products/ProductComparisonPage";
import ServicesPage from "./pages/services/ServicesPage";
import ServiceDetailPage from "./pages/services/ServiceDetailPage";
import ServiceComparisonPage from "./pages/services/ServiceComparisonPage";
import StoresPage from "./pages/stores/StoresPage";
import FavoritesPage from "./pages/user/FavoritesPage";
import UserPage from "./pages/user/UserPage";
import CartPage from "./pages/cart/CartPage";

export default function App() {
  const [page, setPage] = useState<Page>({ id: "home" });
  const [favorites, setFavorites] = useState<Set<string>>(new Set());
  const [compareList, setCompareList] = useState<Set<string>>(new Set());
  const [cart, setCart] = useState<CartItem[]>(() => {
    try {
      const savedCart = localStorage.getItem("soloservis-cart");
      return savedCart ? (JSON.parse(savedCart) as CartItem[]) : [];
    } catch {
      return [];
    }
  });

  useEffect(() => {
    localStorage.setItem("soloservis-cart", JSON.stringify(cart));
  }, [cart]);

  const addToCart = (product: Product) => {
    setCart((prev) => {
      const existing = prev.find((item) => item.product.id === product.id);

      if (existing) {
        return prev.map((item) =>
          item.product.id === product.id
            ? { ...item, quantity: item.quantity + 1 }
            : item,
        );
      }

      return [...prev, { product, quantity: 1 }];
    });
  };

  const updateCartQuantity = (productId: string, quantity: number) => {
    if (quantity <= 0) {
      setCart((prev) => prev.filter((item) => item.product.id !== productId));
      return;
    }

    setCart((prev) =>
      prev.map((item) =>
        item.product.id === productId ? { ...item, quantity } : item,
      ),
    );
  };

  const removeFromCart = (productId: string) => {
    setCart((prev) => prev.filter((item) => item.product.id !== productId));
  };

  // Scroll to top on navigation
  useEffect(() => {
    window.scrollTo({ top: 0, behavior: "smooth" });
  }, [page]);

  const navigate = (next: Page) => setPage(next);

  const toggleFavorite = (id: string) => {
    setFavorites((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const toggleCompare = (id: string) => {
    setCompareList((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else if (next.size < 3) {
        next.add(id);
      }
      return next;
    });
  };

  const sharedProps = {
    navigate,
    favorites,
    compareList,
    onToggleFavorite: toggleFavorite,
    onToggleCompare: toggleCompare,
  };

  const renderPage = () => {
    switch (page.id) {
      case "home":
        return <HomePage {...sharedProps} />;
      case "search-products":
        return <SearchResultsPage {...sharedProps} query={page.query} />;
      case "product-detail":
        return (
          <ProductDetailPage
            productId={page.productId}
            navigate={navigate}
            isFavorite={favorites.has(page.productId)}
            isComparing={compareList.has(page.productId)}
            onToggleFavorite={toggleFavorite}
            onToggleCompare={toggleCompare}
            onAddToCart={addToCart}
          />
        );
      case "product-comparison":
        return <ProductComparisonPage productIds={page.productIds} navigate={navigate} />;
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
            onToggleFavorite={toggleFavorite}
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
      case "user":
        return <UserPage navigate={navigate} favorites={favorites} />;
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

      {/* Compare bar */}
      {compareList.size > 0 && (
        <div
          style={{
            background: "#111111",
            borderBottom: "1px solid rgba(232,0,27,0.3)",
            zIndex: 40,
          }}
          className="sticky top-14 px-4 py-2"
        >
          <div className="max-w-7xl mx-auto flex items-center justify-between gap-3 flex-wrap">
            <div className="flex items-center gap-2">
              <svg
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="#E8001B"
                strokeWidth="2"
              >
                <path d="M9 3H5a2 2 0 0 0-2 2v4m6-6h10a2 2 0 0 1 2 2v4M9 3v18m0 0h10a2 2 0 0 0 2-2V9M9 21H5a2 2 0 0 1-2-2V9m0 0h18" />
              </svg>
              <span className="text-xs text-prime font-semibold">
                {compareList.size} producto{compareList.size > 1 ? "s" : ""} en comparador
              </span>
            </div>
            <div className="flex items-center gap-2">
              {compareList.size >= 2 && (
                <button
                  onClick={() =>
                    navigate({
                      id: "product-comparison",
                      productIds: [...compareList],
                    })
                  }
                  style={{ background: "#E8001B", color: "#0A0A0A" }}
                  className="px-3 py-1 rounded-lg text-xs font-semibold hover:opacity-90 transition-opacity"
                >
                  Comparar ahora
                </button>
              )}
              <button
                onClick={() => setCompareList(new Set())}
                className="text-xs text-muted hover:text-danger transition-colors"
              >
                Limpiar
              </button>
            </div>
          </div>
        </div>
      )}

      <main style={{ flex: 1 }}>{renderPage()}</main>

      <Footer navigate={navigate} />
    </div>
  );
}



