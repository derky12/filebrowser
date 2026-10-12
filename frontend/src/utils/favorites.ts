import { encodePath } from "@/utils/url";

// Favorites are stored server side as decoded, scope-relative paths without
// a trailing slash ("/Photos/2024"); the root is "/".

// Converts a listing url ("/files/Photos/2024/") to a favorite path.
export function urlToFavorite(url: string): string {
  const path = url
    .replace(/^\/files/, "")
    .split("/")
    .map((segment) => {
      try {
        return decodeURIComponent(segment);
      } catch {
        return segment;
      }
    })
    .join("/");

  return path.length > 1 ? path.replace(/\/+$/, "") || "/" : "/";
}

// Converts a favorite path to a listing url with a trailing slash.
export function favoriteToUrl(favorite: string): string {
  const encoded = encodePath(favorite).replace(/\/+$/, "");
  return `/files${encoded}/`;
}

export function favoriteName(favorite: string): string {
  const parts = favorite.split("/").filter(Boolean);
  return parts.length > 0 ? parts[parts.length - 1] : "/";
}
