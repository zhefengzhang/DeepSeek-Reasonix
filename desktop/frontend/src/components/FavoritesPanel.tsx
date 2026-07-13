import { useCallback, useEffect, useRef, useState, type DragEvent as ReactDragEvent } from "react";
import { Bookmark, GripVertical, Pencil, Pin, Plus, Search, Trash2, X } from "lucide-react";
import { useFavoritesStore } from "../lib/favoritesStore";
import { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";
import { CopyButton } from "./CopyButton";
import type { FavoriteItem } from "../lib/types";

interface FavoritesPanelProps {
  workspaceRoot: string;
}

export function FavoritesPanel({ workspaceRoot }: FavoritesPanelProps) {
  const t = useT();
  const { showToast } = useToast();
  const store = useFavoritesStore();
  const {
    items,
    loaded,
    currentPage,
    composerText,
    lastError,
    load,
    add,
    remove,
    update,
    reorder,
    setPage,
    clearError,
  } = store;

  // Show toast on flush errors.
  useEffect(() => {
    if (lastError) {
      showToast(lastError, "error");
      clearError();
    }
  }, [lastError, showToast, clearError]);

  // Load favorites on mount and when workspaceRoot changes.
  useEffect(() => {
    void load(workspaceRoot);
  }, [workspaceRoot, load]);

  // ── pagination ──
  const [searchQuery, setSearchQuery] = useState("");
  const [showSearch, setShowSearch] = useState(false);
  const filteredItems = searchQuery.trim()
    ? items.filter((item) => item.text.toLowerCase().includes(searchQuery.toLowerCase()))
    : items;
  const totalPages = Math.max(1, Math.ceil(filteredItems.length / 20));
  const pageItems = filteredItems.slice((currentPage - 1) * 20, currentPage * 20);

  // ── editing state ──
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editText, setEditText] = useState("");

  const startEdit = (item: FavoriteItem) => {
    setEditingId(item.id);
    setEditText(item.text);
  };

  const cancelEdit = () => {
    setEditingId(null);
    setEditText("");
  };

  const saveEdit = (id: string) => {
    const trimmed = editText.trim();
    if (trimmed) {
      update(id, trimmed);
    }
    setEditingId(null);
  };

  const onEditKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>, id: string) => {
    if (e.key === "Escape") {
      cancelEdit();
    } else if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      saveEdit(id);
    }
  };

  // ── delete confirmation ──
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const confirmDelete = (id: string) => {
    remove(id);
    setDeleteId(null);
  };

  // ── drag-and-drop ──
  const dragItem = useRef<number>(-1);
  const dragOverItem = useRef<number>(-1);

  const onDragStart = (_e: ReactDragEvent, index: number) => {
    dragItem.current = index;
  };

  const onDragOver = (e: ReactDragEvent, index: number) => {
    e.preventDefault();
    dragOverItem.current = index;
  };

  const onDragEnd = () => {
    const from = dragItem.current;
    const to = dragOverItem.current;
    if (from >= 0 && to >= 0 && from !== to) {
      // Convert page-local indices to global indices.
      const globalFrom = (currentPage - 1) * 20 + from;
      const globalTo = (currentPage - 1) * 20 + to;
      reorder(globalFrom, globalTo);
    }
    dragItem.current = -1;
    dragOverItem.current = -1;
  };

  // ── add current input ──
  const addCurrentInput = useCallback(async () => {
    const text = composerText.trim();
    if (!text) return;
    await add(text, "user", "");
    // Jump to the last page so the user sees the newly added item.
    const tp = Math.max(1, Math.ceil((items.length + 1) / 20));
    setPage(tp);
  }, [composerText, add, items.length, setPage]);


  // ── empty state ──
  if (!loaded) {
    return (
      <div className="favorites-panel">
        <div className="favorites-panel__empty">{t("caps.loading")}</div>
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <div className="favorites-panel">
        <div className="favorites-panel__empty">
          <Bookmark size={32} />
          <p>{t("favorites.empty")}</p>
        </div>
        {composerText.trim() && (
          <div className="favorites-panel__footer">
            <button
              className="favorites-panel__add-btn"
              type="button"
              onClick={addCurrentInput}
            >
              <Plus size={14} />
              <span>{t("favorites.addCurrent")}</span>
            </button>
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="favorites-panel">
      {items.length > 0 && (
        <div className="favorites-panel__toolbar">
          <button
            className="msg-meta__btn"
            type="button"
            aria-label={t("favorites.search")}
            title={t("favorites.search")}
            onClick={() => { setShowSearch((v) => !v); if (showSearch) setSearchQuery(""); }}
          >
            <Search size={14} />
          </button>
        </div>
      )}
      {items.length > 0 && showSearch && (
        <div className="favorites-panel__search">
          <Search size={14} />
          <input
            type="text"
            placeholder={t("favorites.search")}
            value={searchQuery}
            onChange={(e) => { setSearchQuery(e.target.value); setPage(1); }}
          />
          {searchQuery.trim() && (
            <button type="button" onClick={() => setSearchQuery("")} aria-label={t("common.close")}>
              <X size={14} />
            </button>
          )}
          {searchQuery.trim() && (
            <span className="favorites-panel__search-count">
              {t("favorites.searchResult", { n: filteredItems.length })}
            </span>
          )}
        </div>
      )}
      <div className="favorites-panel__list">
        {pageItems.map((item, index) => {
          const isEditing = editingId === item.id;
          const isConfirmingDelete = deleteId === item.id;
          const sourceLabel = item.source === "user" ? t("favorites.sourceUser") : t("favorites.sourceAssistant");

          return (
            <div
              key={item.id}
              className={`favorites-panel__item${isEditing ? " favorites-panel__item--editing" : ""}`}
              draggable={!isEditing && !searchQuery.trim()}
              onDragStart={(e) => onDragStart(e, index)}
              onDragOver={(e) => onDragOver(e, index)}
              onDragEnd={onDragEnd}
            >
              {/* Pin to top */}
              {!isEditing && (
                <button
                  className="msg-meta__btn favorites-panel__pin"
                  type="button"
                  aria-label={t("favorites.pin")}
                  title={t("favorites.pin")}
                  onClick={() => {
                    const globalIdx = items.indexOf(item);
                    if (globalIdx >= 0) {
                      reorder(globalIdx, 0);
                      setPage(1);
                    }
                  }}
                >
                  <Pin size={13} />
                </button>
              )}
              {/* Drag handle */}
              {!isEditing && !searchQuery.trim() && (
                <span className="favorites-panel__grip" aria-label={t("favorites.dragHint")} title={t("favorites.dragHint")}>
                  <GripVertical size={14} />
                </span>
              )}

              {/* Source badge */}
              <span className="favorites-panel__source" title={sourceLabel}>
                {item.source === "user" ? "👤" : "🤖"}
              </span>

              {/* Text content or edit textarea */}
              {isEditing ? (
                <textarea
                  ref={(el) => { if (el) el.focus(); }}
                  className="favorites-panel__textarea"
                  value={editText}
                  onChange={(e) => setEditText(e.target.value)}
                  onKeyDown={(e) => onEditKeyDown(e, item.id)}
                  rows={Math.max(2, Math.min(6, editText.split(/\r?\n/).length))}
                />
              ) : (
                <span className="favorites-panel__text" title={item.text}>
                  {item.text}
                </span>
              )}

              {/* Actions */}
              <div className="favorites-panel__actions">
                {isEditing ? (
                  <>
                    <button
                      className="msg-meta__btn"
                      type="button"
                      aria-label={t("common.save")}
                      title={t("common.save")}
                      onClick={() => saveEdit(item.id)}
                    >
                      <Pencil size={14} />
                    </button>
                    <button
                      className="msg-meta__btn"
                      type="button"
                      aria-label={t("common.cancel")}
                      title={t("common.cancel")}
                      onClick={cancelEdit}
                    >
                      <X size={14} />
                    </button>
                  </>
                ) : isConfirmingDelete ? (
                  <>
                    <span className="favorites-panel__confirm-text">{t("favorites.confirmDelete")}</span>
                    <button
                      className="msg-meta__btn"
                      type="button"
                      aria-label={t("common.delete")}
                      title={t("common.delete")}
                      onClick={() => confirmDelete(item.id)}
                    >
                      <Trash2 size={14} />
                    </button>
                    <button
                      className="msg-meta__btn"
                      type="button"
                      aria-label={t("common.cancel")}
                      title={t("common.cancel")}
                      onClick={() => setDeleteId(null)}
                    >
                      <X size={14} />
                    </button>
                  </>
                ) : (
                  <>
                    <CopyButton text={item.text} label={t("favorites.copy")} />
                    <button
                      className="msg-meta__btn"
                      type="button"
                      aria-label={t("favorites.edit")}
                      title={t("favorites.edit")}
                      onClick={() => startEdit(item)}
                    >
                      <Pencil size={14} />
                    </button>
                    <button
                      className="msg-meta__btn"
                      type="button"
                      aria-label={t("favorites.delete")}
                      title={t("favorites.delete")}
                      onClick={() => setDeleteId(item.id)}
                    >
                      <Trash2 size={14} />
                    </button>
                  </>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="favorites-panel__pagination">
          <button
            type="button"
            disabled={currentPage <= 1}
            onClick={() => setPage(currentPage - 1)}
          >
            ← {t("favorites.prevPage")}
          </button>
          <span>{t("favorites.pageOf", { current: currentPage, total: totalPages })}</span>
          <button
            type="button"
            disabled={currentPage >= totalPages}
            onClick={() => setPage(currentPage + 1)}
          >
            {t("favorites.nextPage")} →
          </button>
        </div>
      )}

      {/* Footer: add current input */}
      {composerText.trim() && (
        <div className="favorites-panel__footer">
          <button
            className="favorites-panel__add-btn"
            type="button"
            onClick={addCurrentInput}
          >
            <Plus size={14} />
            <span>{t("favorites.addCurrent")}</span>
          </button>
        </div>
      )}
    </div>
  );
}
