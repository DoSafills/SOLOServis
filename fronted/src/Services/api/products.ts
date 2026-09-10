export interface ApiProduct {
  id: string;
  name: string;
  brand: string;
  model: string;
  category: string;
  description: string;
  rating: number;
  reviewCount: number;
}

export interface ApiProductImage {
  url: string;
  altText: string;
  sortOrder: number;
}

export interface ApiProductDetailOffer {
  id: number;
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

export interface ApiProductDetail extends ApiProduct {
  images: ApiProductImage[];
  offers: ApiProductDetailOffer[];
}

const API_URL = import.meta.env.VITE_API_URL;

export async function getProducts(): Promise<ApiProduct[]> {
  const response = await fetch(`${API_URL}/products`);

  if (!response.ok) {
    throw new Error(`Failed to fetch products: ${response.status}`);
  }

  return response.json();
}

export async function getProductById(publicId: string): Promise<ApiProductDetail> {
  const response = await fetch(`${API_URL}/products/${publicId}`);

  if (!response.ok) {
    throw new Error(`Failed to fetch product: ${response.status}`);
  }

  return response.json();
}