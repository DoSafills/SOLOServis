import type { Product, Service, Store } from "../types";

const genHistory = (
  basePrice: number,
  days: number,
): Array<{
  date: string;
  price: number;
}> => {
  const points = [];
  const now = new Date();
  for (let i = days; i >= 0; i -= Math.ceil(days / 30)) {
    const d = new Date(now);
    d.setDate(d.getDate() - i);
    const variation = basePrice * (0.85 + Math.random() * 0.3);
    points.push({
      date: d.toISOString().split("T")[0],
      price: Math.round(variation / 1000) * 1000,
    });
  }
  return points;
};

/** Generate a sparse offer-price history (promotions happen only a few times per year) */
const genOfferHistory = (
  basePrice: number,
): Array<{
  date: string;
  price: number;
}> => {
  const now = new Date();
  const promos = [
    { daysAgo: 300, discount: 0.18 },
    { daysAgo: 200, discount: 0.12 },
    { daysAgo: 90, discount: 0.15 },
    { daysAgo: 15, discount: 0.1 },
  ];
  return promos.map(({ daysAgo, discount }) => {
    const d = new Date(now);
    d.setDate(d.getDate() - daysAgo);
    return {
      date: d.toISOString().split("T")[0],
      price: Math.round((basePrice * (1 - discount)) / 1000) * 1000,
    };
  });
};

export const stores: Store[] = [
  {
    id: "techzone",
    name: "TechZone",
    logo: "TZ",
    rating: 4.7,
    reviewCount: 12840,
    productCount: 8420,
    reputation: "Excelente",
    dispatchTime: "24–48 hrs",
    conditions: "Despacho gratis sobre $50.000. Devoluciones en 30 días.",
    website: "techzone.cl",
  },
  {
    id: "megapc",
    name: "MegaPC",
    logo: "MP",
    rating: 4.5,
    reviewCount: 9310,
    productCount: 6150,
    reputation: "Muy buena",
    dispatchTime: "48–72 hrs",
    conditions: "Despacho gratis sobre $70.000. Devoluciones en 15 días.",
    website: "megapc.cl",
  },
  {
    id: "infoplex",
    name: "InfoPlex",
    logo: "IP",
    rating: 4.3,
    reviewCount: 5670,
    productCount: 4200,
    reputation: "Buena",
    dispatchTime: "3–5 días hábiles",
    conditions: "Despacho a cargo del cliente. Devoluciones en 10 días.",
    website: "infoplex.cl",
  },
  {
    id: "digitalstore",
    name: "DigitalStore",
    logo: "DS",
    rating: 4.6,
    reviewCount: 7890,
    productCount: 5800,
    reputation: "Muy buena",
    dispatchTime: "24 hrs",
    conditions: "Despacho gratis sobre $40.000. Devoluciones en 30 días.",
    website: "digitalstore.cl",
  },
];

export const products: Product[] = [
  {
    id: "rtx-4060",
    name: "ASUS RTX 4060 Dual 8GB",
    brand: "ASUS",
    model: "DUAL-RTX4060-O8G",
    category: "Tecnología",
    subcategory: "Gaming",
    image:
      "https://images.unsplash.com/photo-1591489378430-ef2f4c626b35?w=600&h=400&fit=crop&auto=format",
    images: [
      "https://images.unsplash.com/photo-1591489378430-ef2f4c626b35?w=600&h=400&fit=crop&auto=format",
      "https://images.unsplash.com/photo-1587202372616-b43abea06c2a?w=600&h=400&fit=crop&auto=format",
    ],
    description:
      "La RTX 4060 de ASUS con diseño dual fan ofrece rendimiento excepcional en gaming 1080p con tecnología DLSS 3.",
    rating: 4.6,
    reviewCount: 342,
    specs: {
      VRAM: "8GB GDDR6",
      Arquitectura: "Ada Lovelace",
      "Núcleos CUDA": "3072",
      "Bus de memoria": "128-bit",
      TDP: "115W",
      Dimensiones: "240 × 122 × 44 mm",
      Garantía: "3 años",
      Conectores: "HDMI 2.1, 3× DP 1.4a",
    },
    offers: [
      {
        storeId: "techzone",
        storeName: "TechZone",
        price: 299990,
        available: true,
        shipping: 0,
      },
      {
        storeId: "megapc",
        storeName: "MegaPC",
        price: 309990,
        available: true,
        shipping: 5990,
      },
      {
        storeId: "infoplex",
        storeName: "InfoPlex",
        price: 319990,
        available: false,
        shipping: 7990,
      },
      {
        storeId: "digitalstore",
        storeName: "DigitalStore",
        price: 304990,
        available: true,
        shipping: 0,
      },
    ],
    priceHistory: genHistory(299990, 365),
    offerPrice: 269990,
    offerPriceHistory: genOfferHistory(299990),
    tags: ["Nvidia", "GPU", "Gaming", "RTX 40"],
  },
  {
    id: "rtx-4060-ti",
    name: "MSI RTX 4060 Ti Gaming X 16GB",
    brand: "MSI",
    model: "RTX 4060 Ti GAMING X 16G",
    category: "Tecnología",
    subcategory: "Gaming",
    image:
      "https://images.unsplash.com/photo-1587202372616-b43abea06c2a?w=600&h=400&fit=crop&auto=format",
    images: [
      "https://images.unsplash.com/photo-1587202372616-b43abea06c2a?w=600&h=400&fit=crop&auto=format",
    ],
    description:
      "La RTX 4060 Ti de MSI con 16GB de VRAM, ideal para contenido creativo y gaming 1440p con máximos detalles.",
    rating: 4.8,
    reviewCount: 218,
    specs: {
      VRAM: "16GB GDDR6",
      Arquitectura: "Ada Lovelace",
      "Núcleos CUDA": "4352",
      "Bus de memoria": "128-bit",
      TDP: "165W",
      Dimensiones: "323 × 140 × 57 mm",
      Garantía: "3 años",
      Conectores: "HDMI 2.1, 3× DP 1.4a",
    },
    offers: [
      {
        storeId: "techzone",
        storeName: "TechZone",
        price: 449990,
        available: true,
        shipping: 0,
      },
      {
        storeId: "megapc",
        storeName: "MegaPC",
        price: 459990,
        available: true,
        shipping: 0,
      },
      {
        storeId: "digitalstore",
        storeName: "DigitalStore",
        price: 444990,
        available: true,
        shipping: 0,
      },
    ],
    priceHistory: genHistory(449990, 365),
    offerPriceHistory: genOfferHistory(449990),
    tags: ["Nvidia", "GPU", "Gaming", "RTX 40"],
  },
  {
    id: "rx-7600",
    name: "Sapphire Pulse RX 7600 8GB",
    brand: "Sapphire",
    model: "PULSE RX 7600 8GB",
    category: "Tecnología",
    subcategory: "Gaming",
    image:
      "https://images.unsplash.com/photo-1624705002806-5d72df19c3ad?w=600&h=400&fit=crop&auto=format",
    images: [
      "https://images.unsplash.com/photo-1624705002806-5d72df19c3ad?w=600&h=400&fit=crop&auto=format",
    ],
    description:
      "La alternativa AMD de alto valor con 8GB GDDR6. Excelente para gaming 1080p y edición de video ligera.",
    rating: 4.4,
    reviewCount: 187,
    specs: {
      VRAM: "8GB GDDR6",
      Arquitectura: "RDNA 3",
      "Stream Processors": "2048",
      "Bus de memoria": "128-bit",
      TDP: "165W",
      Dimensiones: "235 × 125 × 48 mm",
      Garantía: "2 años",
      Conectores: "HDMI 2.1, 3× DP 2.1",
    },
    offers: [
      {
        storeId: "techzone",
        storeName: "TechZone",
        price: 259990,
        available: true,
        shipping: 0,
      },
      {
        storeId: "infoplex",
        storeName: "InfoPlex",
        price: 269990,
        available: true,
        shipping: 7990,
      },
      {
        storeId: "megapc",
        storeName: "MegaPC",
        price: 264990,
        available: false,
        shipping: 5990,
      },
    ],
    priceHistory: genHistory(259990, 365),
    offerPriceHistory: [],
    tags: ["AMD", "GPU", "Gaming", "RX 7000"],
  },
  {
    id: "lenovo-loq",
    name: "Lenovo LOQ 15IRX9 Gaming",
    brand: "Lenovo",
    model: "LOQ 15IRX9",
    category: "Computación",
    subcategory: "Notebooks",
    image:
      "https://images.unsplash.com/photo-1525547719571-a2d4ac8945e2?w=600&h=400&fit=crop&auto=format",
    images: [
      "https://images.unsplash.com/photo-1525547719571-a2d4ac8945e2?w=600&h=400&fit=crop&auto=format",
    ],
    description:
      "Notebook gaming con i7 de 13ª gen y RTX 4060. Pantalla 144Hz IPS y 16GB RAM DDR5. Relación precio-rendimiento excepcional.",
    rating: 4.5,
    reviewCount: 456,
    specs: {
      Procesador: "Intel Core i7-13620H",
      GPU: "NVIDIA RTX 4060 8GB",
      RAM: "16GB DDR5 4800MHz",
      Almacenamiento: "512GB SSD NVMe",
      Pantalla: '15.6" IPS 144Hz FHD',
      Batería: "60Wh",
      "Sistema operativo": "Windows 11 Home",
      Garantía: "1 año",
    },
    offers: [
      {
        storeId: "techzone",
        storeName: "TechZone",
        price: 849990,
        available: true,
        shipping: 0,
      },
      {
        storeId: "digitalstore",
        storeName: "DigitalStore",
        price: 839990,
        available: true,
        shipping: 0,
      },
      {
        storeId: "megapc",
        storeName: "MegaPC",
        price: 869990,
        available: true,
        shipping: 0,
      },
      {
        storeId: "infoplex",
        storeName: "InfoPlex",
        price: 879990,
        available: false,
        shipping: null,
      },
    ],
    priceHistory: genHistory(849990, 365),
    offerPrice: 799990,
    offerPriceHistory: genOfferHistory(849990),
    tags: ["Lenovo", "Notebook", "Gaming", "Intel"],
  },
  {
    id: "iphone-15",
    name: "Apple iPhone 15 128GB",
    brand: "Apple",
    model: "iPhone 15",
    category: "Celulares",
    subcategory: "Smartphones",
    image:
      "https://images.unsplash.com/photo-1697051897520-8d7d0f77ab8f?w=600&h=400&fit=crop&auto=format",
    images: [
      "https://images.unsplash.com/photo-1697051897520-8d7d0f77ab8f?w=600&h=400&fit=crop&auto=format",
    ],
    description:
      "iPhone 15 con chip A16 Bionic, Dynamic Island, cámara principal de 48 MP y conector USB-C.",
    rating: 4.7,
    reviewCount: 1204,
    specs: {
      Procesador: "Apple A16 Bionic",
      Almacenamiento: "128GB",
      RAM: "6GB",
      Pantalla: '6.1" OLED 60Hz Super Retina XDR',
      "Cámara principal": "48 MP f/1.6",
      Batería: "3877 mAh",
      "Sistema operativo": "iOS 17",
      Garantía: "1 año",
    },
    offers: [
      {
        storeId: "techzone",
        storeName: "TechZone",
        price: 749990,
        available: true,
        shipping: 0,
      },
      {
        storeId: "digitalstore",
        storeName: "DigitalStore",
        price: 739990,
        available: true,
        shipping: 0,
      },
      {
        storeId: "megapc",
        storeName: "MegaPC",
        price: 759990,
        available: true,
        shipping: 5990,
      },
    ],
    priceHistory: genHistory(749990, 365),
    offerPriceHistory: genOfferHistory(749990),
    tags: ["Apple", "iPhone", "Smartphone", "5G"],
  },
  {
    id: "galaxy-s25",
    name: "Samsung Galaxy S25 256GB",
    brand: "Samsung",
    model: "Galaxy S25",
    category: "Celulares",
    subcategory: "Smartphones",
    image:
      "https://images.unsplash.com/photo-1610945415295-d9bbf067e59c?w=600&h=400&fit=crop&auto=format",
    images: [
      "https://images.unsplash.com/photo-1610945415295-d9bbf067e59c?w=600&h=400&fit=crop&auto=format",
    ],
    description:
      "Galaxy S25 con Snapdragon 8 Elite, cámara de 50 MP y Android 15. Integración avanzada con Galaxy AI.",
    rating: 4.6,
    reviewCount: 876,
    specs: {
      Procesador: "Snapdragon 8 Elite",
      Almacenamiento: "256GB",
      RAM: "12GB",
      Pantalla: '6.2" Dynamic AMOLED 120Hz',
      "Cámara principal": "50 MP f/1.8",
      Batería: "4000 mAh",
      "Sistema operativo": "Android 15",
      Garantía: "1 año",
    },
    offers: [
      {
        storeId: "techzone",
        storeName: "TechZone",
        price: 829990,
        available: true,
        shipping: 0,
      },
      {
        storeId: "infoplex",
        storeName: "InfoPlex",
        price: 819990,
        available: true,
        shipping: 0,
      },
      {
        storeId: "digitalstore",
        storeName: "DigitalStore",
        price: 839990,
        available: false,
        shipping: null,
      },
    ],
    priceHistory: genHistory(829990, 365),
    offerPriceHistory: [],
    tags: ["Samsung", "Android", "Smartphone", "5G"],
  },
];

export const services: Service[] = [
  {
    id: "entel-500",
    name: "Internet Hogar 500 Mbps",
    provider: "Entel",
    category: "Internet",
    subcategory: "Fibra óptica",
    description: "Fibra óptica simétrica de 500 Mbps para toda la familia. Sin límite de descarga.",
    monthlyPrice: 19990,
    installationCost: 0,
    contractMonths: 12,
    rating: 4.3,
    reviewCount: 2340,
    specs: {
      "Velocidad bajada": "500 Mbps",
      "Velocidad subida": "500 Mbps",
      Tecnología: "Fibra óptica",
      "Límite de datos": "Sin límite",
      "Router incluido": "Sí",
      Soporte: "24/7",
    },
    benefits: ["Wi-Fi 6 incluido", "Sin límite de datos", "TV básica incluida"],
    coverage: "Región Metropolitana, Valparaíso, Biobío",
    image:
      "https://images.unsplash.com/photo-1558494949-ef010cbdcc31?w=400&h=300&fit=crop&auto=format",
    priceHistory: genHistory(19990, 365),
  },
  {
    id: "movistar-600",
    name: "Internet Hogar 600 Mbps",
    provider: "Movistar",
    category: "Internet",
    subcategory: "Fibra óptica",
    description:
      "Plan fibra óptica 600 Mbps con router Wi-Fi 6 y hasta 5 dispositivos conectados simultáneamente.",
    monthlyPrice: 22990,
    installationCost: 10000,
    contractMonths: null,
    rating: 4.1,
    reviewCount: 1890,
    specs: {
      "Velocidad bajada": "600 Mbps",
      "Velocidad subida": "600 Mbps",
      Tecnología: "Fibra óptica",
      "Límite de datos": "Sin límite",
      "Router incluido": "Sí",
      Soporte: "Lun–Sáb 8–20 hrs",
    },
    benefits: ["Sin permanencia", "Router Wi-Fi 6", "App de control parental"],
    coverage: "Cobertura nacional en 15 regiones",
    image:
      "https://images.unsplash.com/photo-1544197150-b99a580bb7a8?w=400&h=300&fit=crop&auto=format",
    priceHistory: genHistory(22990, 365),
  },
  {
    id: "wom-100gb",
    name: "Plan Móvil 100 GB",
    provider: "WOM",
    category: "Telefonía",
    subcategory: "Plan móvil",
    description:
      "Plan móvil con 100 GB de datos, llamadas ilimitadas y roaming en Latinoamérica incluido.",
    monthlyPrice: 14990,
    installationCost: null,
    contractMonths: null,
    rating: 4.0,
    reviewCount: 3210,
    specs: {
      Datos: "100 GB 5G/4G LTE",
      Llamadas: "Ilimitadas",
      SMS: "Ilimitados",
      Roaming: "LATAM incluido",
      Red: "5G/4G LTE",
      Portabilidad: "Gratis",
    },
    benefits: ["Sin costo de portabilidad", "Datos adicionales a $1.990/GB", "App WOM incluida"],
    coverage: "Cobertura 5G en principales ciudades",
    image:
      "https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?w=400&h=300&fit=crop&auto=format",
    priceHistory: genHistory(14990, 365),
  },
  {
    id: "netflix-premium",
    name: "Netflix Premium 4K",
    provider: "Netflix",
    category: "Streaming",
    subcategory: "Video",
    description:
      "Acceso ilimitado a series, películas y documentales en calidad 4K Ultra HD. Hasta 4 pantallas simultáneas.",
    monthlyPrice: 17990,
    installationCost: null,
    contractMonths: null,
    rating: 4.5,
    reviewCount: 8940,
    specs: {
      Calidad: "4K Ultra HD + HDR",
      "Pantallas simultáneas": "4",
      Descargas: "Sí (hasta 4 dispositivos)",
      Idiomas: "+30 idiomas",
      Audio: "Dolby Atmos",
      Perfiles: "Hasta 6",
    },
    benefits: ["Sin anuncios", "Contenido exclusivo", "Modo offline", "Calidad 4K HDR"],
    coverage: "Disponible en todo Chile",
    image:
      "https://images.unsplash.com/photo-1574375927938-d5a98e8ffe85?w=400&h=300&fit=crop&auto=format",
    priceHistory: genHistory(17990, 365),
  },
];

export const formatPrice = (price: number): string => `$${price.toLocaleString("es-CL")}`;

export const getMinPrice = (product: Product): number =>
  Math.min(...product.offers.filter((o) => o.available).map((o) => o.price));

export const getMinOffer = (product: Product) =>
  product.offers.filter((o) => o.available).sort((a, b) => a.price - b.price)[0];

export const getAvailableStoreCount = (product: Product): number =>
  product.offers.filter((o) => o.available).length;
