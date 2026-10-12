import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { users } from "@/api";
import { useAuthStore } from "../auth";

vi.mock("@/api", () => ({
  users: { get: vi.fn(), update: vi.fn() },
}));
vi.mock("@/i18n", () => ({
  detectLocale: () => "en",
  setLocale: vi.fn(),
}));

// What parseToken passes to setUser: the JWT payload, without favorites.
const jwtUser = (id = 1) => ({ id, username: "u", locale: "en" }) as IUser;

describe("auth store favorites", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.resetAllMocks();
  });

  it("loads favorites once after login", async () => {
    vi.mocked(users.get).mockResolvedValue({ favorites: ["/A"] } as IUser);
    const store = useAuthStore();

    store.setUser(jwtUser());
    await store.loadUserData();

    expect(store.user?.favorites).toEqual(["/A"]);
  });

  it("keeps favorites when the token is renewed", async () => {
    vi.mocked(users.get).mockResolvedValue({ favorites: ["/A"] } as IUser);
    const store = useAuthStore();
    store.setUser(jwtUser());
    await store.loadUserData();

    // Navigating into a folder renews the token, replacing the user.
    store.setUser(jwtUser());
    await store.loadUserData();

    expect(store.user?.favorites).toEqual(["/A"]);
    expect(users.get).toHaveBeenCalledTimes(1);
  });

  it("does not leak favorites to a different user", async () => {
    vi.mocked(users.get).mockResolvedValue({ favorites: ["/A"] } as IUser);
    const store = useAuthStore();
    store.setUser(jwtUser(1));
    await store.loadUserData();

    store.setUser(jwtUser(2));

    expect(store.user?.favorites).toBeUndefined();
  });

  it("loads and keeps folder colors across renewals", async () => {
    vi.mocked(users.get).mockResolvedValue({
      favorites: [],
      folderColors: { "/A": "#e53935" },
    } as unknown as IUser);
    const store = useAuthStore();
    store.setUser(jwtUser());
    await store.loadUserData();

    store.setUser(jwtUser());

    expect(store.folderColor("/A")).toBe("#e53935");
  });

  it("sets, clears and rolls back folder colors", async () => {
    vi.mocked(users.update).mockResolvedValue(undefined);
    const store = useAuthStore();
    store.setUser({ ...jwtUser(), favorites: [], folderColors: {} });

    await store.setFolderColor("/A", "#e53935");
    expect(store.folderColor("/A")).toBe("#e53935");
    expect(users.update).toHaveBeenCalledWith(
      { id: 1, folderColors: { "/A": "#e53935" } },
      ["folderColors"]
    );

    await store.setFolderColor("/A", null);
    expect(store.folderColor("/A")).toBeUndefined();

    vi.mocked(users.update).mockRejectedValue(new Error("boom"));
    await expect(store.setFolderColor("/B", "#1e88e5")).rejects.toThrow("boom");
    expect(store.folderColor("/B")).toBeUndefined();
  });

  it("toggles, persists and rolls back on failure", async () => {
    vi.mocked(users.update).mockResolvedValue(undefined);
    const store = useAuthStore();
    store.setUser({ ...jwtUser(), favorites: [] });

    await store.toggleFavorite("/A");
    expect(store.user?.favorites).toEqual(["/A"]);
    expect(users.update).toHaveBeenCalledWith({ id: 1, favorites: ["/A"] }, [
      "favorites",
    ]);

    vi.mocked(users.update).mockRejectedValue(new Error("boom"));
    await expect(store.toggleFavorite("/B")).rejects.toThrow("boom");
    expect(store.user?.favorites).toEqual(["/A"]);
  });
});
