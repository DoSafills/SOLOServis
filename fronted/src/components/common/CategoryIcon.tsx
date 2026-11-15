const categoryIconPaths = {
  refrigerator: "M6 3h12v18H6z M6 10h12 M9 7v1 M9 13v2",
  bolt: "M13 2 5 13h6l-1 9 9-12h-6l1-8Z",
  laptop: "M5 4h14v11H5z M2 20h20l-2-5H4z",
  phone: "M7 2.5h10v19H7z M11 18.5h2",
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
