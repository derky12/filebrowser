import { describe, expect, it } from "vitest";
import { favoriteName, favoriteToUrl, urlToFavorite } from "../favorites";

describe("favorites", () => {
  it("converts listing urls to decoded favorite paths", () => {
    expect(urlToFavorite("/files/Photos/2024/")).toBe("/Photos/2024");
    expect(urlToFavorite("/files/My%20Docs/%E6%96%87%E4%BB%B6/")).toBe(
      "/My Docs/文件"
    );
    expect(urlToFavorite("/files/")).toBe("/");
  });

  it("converts favorites back to encoded listing urls", () => {
    expect(favoriteToUrl("/My Docs/文件")).toBe(
      "/files/My%20Docs/%E6%96%87%E4%BB%B6/"
    );
    expect(favoriteToUrl("/")).toBe("/files/");
  });

  it("round-trips", () => {
    const url = "/files/a%20b/c%23d/";
    expect(favoriteToUrl(urlToFavorite(url))).toBe(url);
  });

  it("derives a display name", () => {
    expect(favoriteName("/Photos/2024")).toBe("2024");
    expect(favoriteName("/")).toBe("/");
  });
});
