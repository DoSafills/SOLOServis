export interface ApiOffer {
  id: number;
  productId: number;
  productPublicId: string;
  productName: string;
  storeId: number;
  storeName: string;
  price: string;
  listPrice: string;
  currency: string;
  shippingCost: string;
  shippingFree: boolean;
  available: boolean;
  stock: number | null;
  condition: string;
  productUrl: string;
}

const API_URL = import.meta.env.VITE_API_URL;

export async function getOffers(): Promise<ApiOffer[]> {
  const response = await fetch(`${API_URL}/product-offers`);

  if (!response.ok) {
    throw new Error(`Failed to fetch offers: ${response.status}`);
  }

  return response.json();
}
