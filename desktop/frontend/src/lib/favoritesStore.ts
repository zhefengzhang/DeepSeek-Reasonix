// favoritesStore — Zustand store for session content bookmarks.
// Data is persisted to .reasonix/favorites.json via the Go backend, keyed by
// workspace root. All tabs sharing a workspace see the same favorites.
//
// Each mutation (add / remove / update / reorder) auto-flushes after a 500ms
// debounce, so rapid operations coalesce into a single file write.
//
// ID generation is delegated to the Go backend (app.GenerateFavoriteID) so
// the ID namespace is consistent regardless of which frontend process creates
// the item.

import { create } from "zustand";
import { app } from "./bridge";
import type { FavoriteItem } from "./types";

const PAGE_SIZE = 20;

// ── module-level add lock ─────────────────────────────────────────────
// Prevents concurrent add() calls from creating duplicate items for the
// same originalMessageId (e.g. user double-clicks the bookmark button
// before the async Go ID round-trip completes).
let addPending: Promise<void> | null = null;

// ── module-level debounce ──────────────────────────────────────────────

let flushTimer: ReturnType<typeof setTimeout> | null = null;

function debounceFlush(fn: () => void) {
  if (flushTimer) clearTimeout(flushTimer);
  flushTimer = setTimeout(() => {
    flushTimer = null;
    fn();
  }, 500);
}

// ── store shape ────────────────────────────────────────────────────────

export interface FavoritesState {
  items: FavoriteItem[];
  loaded: boolean;
  dirty: boolean;
  currentPage: number;
  workspaceRoot: string;
  composerText: string;
  lastError: string | null;
  requestGuidanceFill: string | null;
}

interface FavoritesActions {
  load(workspaceRoot: string): Promise<void>;
  add(text: string, source: "user" | "assistant", originalMessageId: string): Promise<void>;
  remove(id: string): void;
  update(id: string, text: string): void;
  reorder(fromIndex: number, toIndex: number): void;
  flush(): Promise<void>;
  setPage(page: number): void;
  setComposerText(text: string): void;
  clearError(): void;
  totalPages(): number;
  pageItems(): FavoriteItem[];
  fillGuidancePrompt(text: string): void;
  clearRequestGuidanceFill(): void;
}

// ── store ──────────────────────────────────────────────────────────────

export const useFavoritesStore = create<FavoritesState & FavoritesActions>((set, get) => {
  function doFlush() {
    const state = get();
    if (!state.dirty || !state.workspaceRoot) return;
    app
      .SaveFavorites(state.workspaceRoot, { items: state.items })
      .then(() => set({ dirty: false }))
      .catch((err: unknown) => {
        const msg = err instanceof Error ? err.message : String(err);
        console.error("SaveFavorites failed", msg);
        set({ lastError: msg, dirty: false });
      });
  }

  function scheduleFlush() {
    debounceFlush(doFlush);
  }

  return {
    items: [],
    loaded: false,
    dirty: false,
    currentPage: 1,
    workspaceRoot: "",
    composerText: "",
    lastError: null,
    requestGuidanceFill: null,

    async load(workspaceRoot: string) {
      const state = get();
      if (state.loaded && state.workspaceRoot === workspaceRoot) return;

      let items: FavoriteItem[] = [];
      try {
        const view = await app.GetFavorites(workspaceRoot);
        items = view.items ?? [];
      } catch {
        // Keep empty list on error.
      }
      set({
        items,
        loaded: true,
        dirty: false,
        currentPage: 1,
        workspaceRoot,
      });
    },

    async add(text: string, source: "user" | "assistant", originalMessageId: string) {
      // Dedup: if this message is already bookmarked, skip.
      const existing = get().items;
      if (originalMessageId && existing.some((it) => it.originalMessageId === originalMessageId)) {
        return;
      }

      // Serialize concurrent add() calls to prevent duplicate items.
      while (addPending) {
        await addPending;
      }
      let resolveLock: () => void;
      addPending = new Promise<void>((r) => { resolveLock = r; });

      try {
        const state = get();
        let id: string;
        try {
          id = await app.GenerateFavoriteID();
        } catch {
          // Fallback: generate a local ID if the Go side is unavailable.
          id = `fav_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
        }
        const item: FavoriteItem = {
          id,
          text,
          source,
          originalMessageId,
          createdAt: Date.now(),
          order: state.items.length,
        };
        set({ items: [...state.items, item], dirty: true });
        scheduleFlush();
      } finally {
        addPending = null;
        resolveLock!();
      }
    },

    remove(id: string) {
      const state = get();
      const next = state.items
        .filter((item) => item.id !== id)
        .map((item, i) => ({ ...item, order: i }));
      const totalPages = Math.max(1, Math.ceil(next.length / PAGE_SIZE));
      let page = state.currentPage;
      if (page > totalPages) page = totalPages;
      set({ items: next, dirty: true, currentPage: page });
      scheduleFlush();
    },

    update(id: string, text: string) {
      const next = get().items.map((item) =>
        item.id === id ? { ...item, text } : item,
      );
      set({ items: next, dirty: true });
      scheduleFlush();
    },

    reorder(fromIndex: number, toIndex: number) {
      const next = [...get().items];
      const [moved] = next.splice(fromIndex, 1);
      next.splice(toIndex, 0, moved);
      const reordered = next.map((item, index) => ({ ...item, order: index }));
      set({ items: reordered, dirty: true });
      scheduleFlush();
    },

    async flush() {
      doFlush();
    },

    setPage(page: number) {
      const state = get();
      const totalPages = Math.max(1, Math.ceil(state.items.length / PAGE_SIZE));
      const clamped = Math.max(1, Math.min(page, totalPages));
      set({ currentPage: clamped });
    },

    setComposerText(text: string) {
      set({ composerText: text });
    },

    clearError() {
      set({ lastError: null });
    },

    totalPages() {
      return Math.max(1, Math.ceil(get().items.length / PAGE_SIZE));
    },

    pageItems() {
      const state = get();
      const start = (state.currentPage - 1) * PAGE_SIZE;
      return state.items.slice(start, start + PAGE_SIZE);
    },

    fillGuidancePrompt(text: string) {
      set({ requestGuidanceFill: text });
    },

    clearRequestGuidanceFill() {
      set({ requestGuidanceFill: null });
    },
  };
});
