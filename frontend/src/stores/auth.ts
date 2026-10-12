import { defineStore } from "pinia";
import { detectLocale, setLocale } from "@/i18n";
import { cloneDeep } from "lodash-es";
import { users } from "@/api";

export const useAuthStore = defineStore("auth", {
  // convert to a function
  state: (): {
    user: IUser | null;
    jwt: string;
    logoutTimer: number | null;
  } => ({
    user: null,
    jwt: "",
    logoutTimer: null,
  }),
  getters: {
    // user and jwt getter removed, no longer needed
    isLoggedIn: (state) => state.user !== null,
  },
  actions: {
    // no context as first argument, use `this` instead
    setUser(user: IUser) {
      if (user === null) {
        this.user = null;
        return;
      }

      setLocale(user.locale || detectLocale());
      // The user decoded from the JWT does not carry the favorites and folder
      // colors, so keep the ones already loaded when the token is renewed.
      const previous = this.user?.id === user.id ? this.user : null;
      this.user = {
        ...user,
        favorites: user.favorites ?? previous?.favorites,
        folderColors: user.folderColors ?? previous?.folderColors,
      } as IUser;
    },
    // Fetches the favorites and folder colors once per session; the JWT does
    // not include them.
    async loadUserData() {
      if (this.user === null || this.user.favorites !== undefined) return;

      const id = this.user.id;
      const { favorites, folderColors } = await users.get(id);
      if (this.user?.id === id && this.user.favorites === undefined) {
        this.user = {
          ...this.user,
          favorites: favorites ?? [],
          folderColors: folderColors ?? {},
        };
      }
    },
    updateUser(user: Partial<IUser>) {
      if (user.locale) {
        setLocale(user.locale);
      }

      this.user = { ...this.user, ...cloneDeep(user) } as IUser;
    },
    isFavorite(path: string): boolean {
      return this.user?.favorites?.includes(path) ?? false;
    },
    // Adds or removes a favorite folder, persisting it on the server and
    // rolling back the local state if saving fails.
    async toggleFavorite(path: string) {
      if (this.user === null) return;

      const previous = this.user.favorites ?? [];
      const favorites = previous.includes(path)
        ? previous.filter((f) => f !== path)
        : [...previous, path];

      this.user = { ...this.user, favorites };
      try {
        await users.update({ id: this.user.id, favorites }, ["favorites"]);
      } catch (e) {
        this.user = { ...this.user, favorites: previous } as IUser;
        throw e;
      }
    },
    folderColor(path: string): string | undefined {
      return this.user?.folderColors?.[path];
    },
    // Sets (or, with null, clears) the color of a folder, persisting it on the
    // server and rolling back the local state if saving fails.
    async setFolderColor(path: string, color: string | null) {
      if (this.user === null) return;

      const previous = this.user.folderColors ?? {};
      const folderColors = { ...previous };
      if (color === null) {
        delete folderColors[path];
      } else {
        folderColors[path] = color;
      }

      this.user = { ...this.user, folderColors };
      try {
        await users.update({ id: this.user.id, folderColors }, [
          "folderColors",
        ]);
      } catch (e) {
        this.user = { ...this.user, folderColors: previous } as IUser;
        throw e;
      }
    },
    // easily reset state using `$reset`
    clearUser() {
      this.$reset();
    },
    setLogoutTimer(logoutTimer: number | null) {
      this.logoutTimer = logoutTimer;
    },
  },
});
