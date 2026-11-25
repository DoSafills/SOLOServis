import { useState } from "react";
import type { CategoryIconName } from "./CategoryIcon";
import { getProductCategoryIcon, getServiceCategoryIcon } from "../../constants/productCatalog";
import { API_URLS } from "../../services/api/config";

const emojiByIcon: Record<CategoryIconName, string> = {
  refrigerator: "🧊",
  bolt: "⚡",
  laptop: "💻",
  phone: "📱",
  gamepad: "🎮",
  house: "🏠",
  wifi: "📶",
  shield: "🔒",
  wrench: "🔧",
  book: "📖",
  grid: "◻️",
  washer: "🫧",
  television: "📺",
  headphones: "🎧",
  camera: "📷",
  furniture: "🪑",
  airConditioning: "❄️",
  vacuum: "🧹",
  coffee: "☕",
  watch: "⌚",
  blender: "🥛",
  airFryer: "🍟",
  microwave: "♨️",
  kitchen: "🍳",
};

const artworkByIcon: Record<CategoryIconName, string> = {
  refrigerator: "refrigerator",
  bolt: "technology",
  laptop: "laptop",
  phone: "phone",
  gamepad: "gamepad",
  house: "home",
  wifi: "router",
  shield: "security",
  wrench: "tools",
  book: "book",
  grid: "generic",
  washer: "washer",
  television: "television",
  headphones: "headphones",
  camera: "camera",
  furniture: "furniture",
  airConditioning: "air-conditioning",
  vacuum: "vacuum",
  coffee: "coffee",
  watch: "watch",
  blender: "blender",
  airFryer: "air-fryer",
  microwave: "microwave",
  kitchen: "kitchen",
};

interface Props {
  name: string;
  type: "product" | "service";
  /** Ícono explícito (el del árbol NV); si falta se infiere del nombre. */
  icon?: CategoryIconName;
}

/**
 * Ilustración de una categoría con emoji de respaldo si la imagen falla.
 * @param props.name Nombre de la categoría.
 * @param props.type Si es categoría de producto o de servicio.
 * @param props.icon Ícono del árbol NV; por defecto se infiere del nombre.
 * @returns La imagen de la categoría o su emoji equivalente.
 */
export default function CategoryArtwork({ name, type, icon: iconProp }: Props) {
  const icon =
    iconProp ?? (type === "product" ? getProductCategoryIcon(name) : getServiceCategoryIcon(name));
  const artworkUrl = `${API_URLS.products}/categories/artworks/${artworkByIcon[icon]}`;
  const [failedArtworkUrl, setFailedArtworkUrl] = useState<string | null>(null);

  if (failedArtworkUrl === artworkUrl) {
    return (
      <span
        aria-hidden="true"
        className="flex h-full w-full items-center justify-center bg-surface-2 text-3xl grayscale opacity-60 transition-transform duration-500 group-hover:scale-[1.03]"
      >
        {emojiByIcon[icon]}
      </span>
    );
  }

  return (
    <img
      src={artworkUrl}
      alt=""
      aria-hidden="true"
      loading="lazy"
      decoding="async"
      onError={() => setFailedArtworkUrl(artworkUrl)}
      className="h-full w-full bg-surface-2 object-contain p-2 transition-transform duration-500 group-hover:scale-[1.03]"
    />
  );
}
