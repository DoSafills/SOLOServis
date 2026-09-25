import { useState, useEffect } from "react";
import type { Page } from "./types";
import Header from "./components/Header";
import Footer from "./components/Footer";
import HomePage from "./pages/Navigation/HomePage";
import SearchResultsPage from "./pages/Navigation/SearchResultsPage";
import ProductDetailPage from "./pages/Navigation/ProductDetailPage";
import ProductComparisonPage from "./pages/Navigation/ProductComparisonPage";
import ServicesPage from "./pages/Services/ServicesPage";
import ServiceDetailPage from "./pages/Services/ServiceDetailPage";
import ServiceComparisonPage from "./pages/Services/ServiceComparisonPage";
import StoresPage from "./pages/Navigation/StoresPage";
import FavoritesPage from "./pages/User/FavoritesPage";
import CartPage from "./pages/User/CartPage";
import UserPage from "./pages/User/UserPage";
import type { CartItem, Product } from "./types";

export default function App() {
  const [page, setPage] = useState<Page>({ id: "home" });
  const [favorites, setFavorites] = useState<Set<string>>(new Set());
  const [compareList, setCompareList] = useState<Set<string>>(new Set());
  const [cart, setCart] = useState<CartItem[]>(() => {
    const stored = localStorage.getItem("soloservis-cart");
    if (!stored) return [];

    try {
      return JSON.parse(stored) as CartItem[];
    } catch {
      return [];
    }
  });

  // Scroll to top on navigation
  useEffect(() => {
    window.scrollTo({ top: 0, behavior: "smooth" });
  }, [page]);

  useEffect(() => {
    localStorage.setItem("soloservis-cart", JSON.stringify(cart));
  }, [cart]);

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

  const addToCart = (product: Product) => {
    const offerPrice =
      product.offers.find((offer) => offer.available)?.price ??
      product.offers[0]?.price ??
      product.offerPrice ??
      0;

    setCart((prev) => {
      const existing = prev.find((item) => item.id === product.id);
      if (existing) {
        return prev.map((item) =>
          item.id === product.id ? { ...item, quantity: item.quantity + 1 } : item,
        );
      }

      return [
        ...prev,
        {
          id: product.id,
          name: product.name,
          brand: product.brand,
          image: product.image,
          price: offerPrice,
          quantity: 1,
        },
      ];
    });
  };

  const updateCartItem = (id: string, quantity: number) => {
    setCart((prev) =>
      prev
        .map((item) => (item.id === id ? { ...item, quantity: Math.max(quantity, 0) } : item))
        .filter((item) => item.quantity > 0),
    );
  };

  const removeFromCart = (id: string) => {
    setCart((prev) => prev.filter((item) => item.id !== id));
  };

  const cartCount = cart.reduce((sum, item) => sum + item.quantity, 0);

  const sharedProps = {
    navigate,
    favorites,
    compareList,
    onToggleFavorite: toggleFavorite,
    onToggleCompare: toggleCompare,
    onAddToCart: addToCart,
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
            onAddToCart={addToCart}
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
            onAddToCart={addToCart}
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
            navigate={navigate}
            cart={cart}
            onRemoveFromCart={removeFromCart}
            onUpdateCartItem={updateCartItem}
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
      <Header navigate={navigate} currentPage={page} favCount={favorites.size} cartCount={cartCount} />

      {/* Compare bar */}
      {compareList.size > 0 && (
        <div
          style={{
            background: "#F1F5F9",
            borderBottom: "1px solid rgba(3,105,161,0.3)",
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
                stroke="#0369A1"
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
                  style={{ background: "#0369A1", color: "#FFFFFF" }}
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
