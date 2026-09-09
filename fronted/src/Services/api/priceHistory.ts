export interface ApiPricePoint {
  id: number;
  productOfferId: number;
  price: string;
  isPromotional: boolean;
  recordedAt: string;
}

const API_URL = import.meta.env.VITE_API_URL;

export async function getPriceHistory(offerId: number): Promise<ApiPricePoint[]> {
  const response = await fetch(`${API_URL}/price-history/${offerId}`);

  if (!response.ok) {
    throw new Error(`Failed to fetch price history: ${response.status}`);
  }

  return response.json();
}
