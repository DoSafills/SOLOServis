const categoryIconPaths = {
  refrigerator: "M6 3h12v18H6z M6 10h12 M9 7v1 M9 13v2",
  bolt: "M13 2 5 13h6l-1 9 9-12h-6l1-8Z",
  laptop: "M5 4h14v11H5z M2 20h20l-2-5H4z",
  phone: "M7 2.5h10v19H7z M11 18.5h2",
  washer:
    "M5 3h14v18H5z M5 7h14 M8 5h.01 M11 5h.01 M12 17a4 4 0 1 0 0-8 4 4 0 0 0 0 8Z M12 11v2l1.5 1",
  television: "M3 5h18v12H3z M8 21h8 M12 17v4 M8 2l4 3 4-3",
  headphones:
    "M3 14v-3a9 9 0 0 1 18 0v3 M3 14h4v7H5a2 2 0 0 1-2-2v-5Z M17 14h4v5a2 2 0 0 1-2 2h-2z",
  camera: "M3 8h4l2-3h6l2 3h4v12H3z M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z",
  furniture: "M5 5h14v7H5z M7 12v9 M17 12v9 M5 16h14",
  airConditioning: "M3 4h18v10H3z M6 8h12 M7 18h10 M9 14v4 M15 14v4",
  vacuum: "M9 3h6v11H9z M12 14l-5 5 M7 19h10 M12 3l3-1",
  coffee:
    "M5 8h12v8a4 4 0 0 1-4 4H9a4 4 0 0 1-4-4z M17 10h2a2 2 0 0 1 0 4h-2 M8 3c2 1 2 2 0 3 M13 3c2 1 2 2 0 3",
  watch: "M8 3h8v4H8z M7 7h10v10H7z M8 17h8v4H8z M12 10v3l2 1",
  blender: "M8 3h8v3l2 3v9H6V9l2-3z M6 9h12 M9 12h6",
  airFryer: "M5 5h14v14H5z M8 8h8v6H8z M10 16h4 M8 3h8",
  microwave: "M3 5h18v14H3z M6 8h10v8H6z M18 8v.01 M18 11v.01 M18 14v.01",
  kitchen: "M5 9h8v11H5z M6 5h6v4H6z M17 4v16 M15 8h4 M15 12h4",
  gamepad:
    "M6.5 8h11a4.5 4.5 0 0 1 4.4 5.5l-.8 3.6a2 2 0 0 1-3.3 1l-2.6-2.1H8.8l-2.6 2.1a2 2 0 0 1-3.3-1l-.8-3.6A4.5 4.5 0 0 1 6.5 8Z M8 10.5v4 M6 12.5h4 M16 11h.01 M18 14h.01",
  house: "M3 10.5 12 3l9 7.5 M5 9v12h14V9 M10 21v-7h4v7",
  wifi: "M2 8.5a15 15 0 0 1 20 0 M5 12a10 10 0 0 1 14 0 M8.5 15.5a5 5 0 0 1 7 0",
  shield: "M12 22s8-4 8-11V5l-8-3-8 3v6c0 7 8 11 8 11Z",
  wrench: "M14.7 6.3a5 5 0 0 0-6.4 6.4L3 18l3 3 5.3-5.3a5 5 0 0 0 6.4-6.4L15 12l-3-3 2.7-2.7Z",
  book: "M4 19.5A2.5 2.5 0 0 1 6.5 17H20 M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2Z M8 6h8",
  grid: "M4 4h7v7H4z M13 4h7v7h-7z M4 13h7v7H4z M13 13h7v7h-7z",
} as const;

export type CategoryIconName = keyof typeof categoryIconPaths;

interface Props {
  name: CategoryIconName;
  className?: string;
}

export default function CategoryIcon({ name, className = "h-5 w-5" }: Props) {
  return (
    <svg
      aria-hidden="true"
      className={`shrink-0 ${className}`}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.7"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d={categoryIconPaths[name]} />
      {name === "wifi" && <circle cx="12" cy="19" r="0.8" fill="currentColor" stroke="none" />}
    </svg>
  );
}
