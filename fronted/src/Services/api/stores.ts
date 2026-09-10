export interface ApiStore {
  id: number;
  name: string;
  websiteUrl: string;
  logoUrl: string;
  rating: number;
  reputation: string;
  shippingInformation: string;
  generalConditions: string;
  active: boolean;
}

const API_URL = import.meta.env.VITE_API_URL;

export async function getStores(): Promise<ApiStore[]> {
  const response = await fetch(`${API_URL}/stores`);

  if (!response.ok) {
    throw new Error(`Failed to fetch stores: ${response.status}`);
  }

  return response.json();
}
