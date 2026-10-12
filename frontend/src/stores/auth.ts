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
      this.user = user;
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
    // easily reset state using `$reset`
    clearUser() {
      this.$reset();
    },
    setLogoutTimer(logoutTimer: number | null) {
      this.logoutTimer = logoutTimer;
    },
  },
});
