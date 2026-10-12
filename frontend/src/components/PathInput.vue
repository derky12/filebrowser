<template>
  <div class="path-input" @focusout="onFocusOut">
    <input
      :id="inputId"
      ref="input"
      class="input input--block"
      type="text"
      role="combobox"
      autocomplete="off"
      spellcheck="false"
      :value="modelValue"
      :placeholder="placeholder"
      :aria-expanded="open && suggestions.length > 0"
      aria-autocomplete="list"
      @input="onInput"
      @focus="refresh"
      @keydown="onKeydown"
    />
    <ul
      v-if="open && suggestions.length > 0"
      class="path-suggestions"
      role="listbox"
    >
      <li
        v-for="(name, i) in suggestions"
        :key="name"
        role="option"
        :class="{ active: i === active }"
        :aria-selected="i === active"
        @mousedown.prevent="choose(name)"
        @mousemove="active = i"
      >
        <i class="material-icons">folder</i>{{ name }}
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from "vue";
import { files as api } from "@/api";
import { encodePath } from "@/utils/url";
import { chooseFolder, splitTyped, suggestFolders } from "@/utils/pathSuggest";
import type { SuggestEntry } from "@/utils/pathSuggest";

const props = defineProps<{
  modelValue: string;
  placeholder?: string;
  inputId?: string;
}>();
const emit = defineEmits<{ (e: "update:modelValue", value: string): void }>();

const input = ref<HTMLInputElement | null>(null);
const suggestions = ref<string[]>([]);
const active = ref(0);
const open = ref(false);

// Folder listings are reused for a few seconds while typing.
const cache = new Map<string, { entries: SuggestEntry[]; at: number }>();
const CACHE_MS = 10000;
let controller: AbortController | null = null;
let timer: number | undefined;

const listFolder = async (parent: string): Promise<SuggestEntry[]> => {
  const hit = cache.get(parent);
  if (hit && Date.now() - hit.at < CACHE_MS) return hit.entries;

  controller?.abort();
  controller = new AbortController();
  const url = `/files${encodePath(parent === "/" ? "" : parent)}/`;
  const resource = await api.fetch(url, controller.signal);
  const entries = (resource.items ?? []).map((i) => ({
    name: i.name,
    isDir: i.isDir,
  }));
  cache.set(parent, { entries, at: Date.now() });

  return entries;
};

const refresh = async () => {
  const { parent, prefix } = splitTyped(props.modelValue);
  try {
    const entries = await listFolder(parent);
    // Ignore an answer to what is no longer typed.
    if (splitTyped(props.modelValue).parent !== parent) return;

    suggestions.value = suggestFolders(
      entries,
      splitTyped(props.modelValue).prefix || prefix
    );
    active.value = 0;
    open.value = true;
  } catch {
    // The folder does not exist (yet) or cannot be read: nothing to suggest.
    suggestions.value = [];
  }
};

const onInput = (event: Event) => {
  emit("update:modelValue", (event.target as HTMLInputElement).value);
  window.clearTimeout(timer);
  // The value reaches the parent on the next tick.
  timer = window.setTimeout(refresh, 120);
};

const choose = (name: string) => {
  const { parent } = splitTyped(props.modelValue);
  emit("update:modelValue", chooseFolder(parent, name));
  input.value?.focus();
  window.clearTimeout(timer);
  timer = window.setTimeout(refresh, 0);
};

const onKeydown = (event: KeyboardEvent) => {
  if (!open.value || suggestions.value.length === 0) return;

  switch (event.key) {
    case "ArrowDown":
      event.preventDefault();
      active.value = (active.value + 1) % suggestions.value.length;
      break;
    case "ArrowUp":
      event.preventDefault();
      active.value =
        (active.value - 1 + suggestions.value.length) %
        suggestions.value.length;
      break;
    case "Tab":
    case "Enter":
      event.preventDefault();
      choose(suggestions.value[active.value]);
      break;
    case "Escape":
      event.stopPropagation();
      open.value = false;
      break;
  }
};

const onFocusOut = (event: FocusEvent) => {
  const root = event.currentTarget as HTMLElement;
  if (!root.contains(event.relatedTarget as Node | null)) open.value = false;
};

onBeforeUnmount(() => {
  window.clearTimeout(timer);
  controller?.abort();
});
</script>

<style scoped>
.path-input {
  position: relative;
}

.path-suggestions {
  position: absolute;
  left: 0;
  right: 0;
  top: 100%;
  z-index: 20;
  max-height: 14em;
  margin: 2px 0 0;
  padding: 0;
  overflow-y: auto;
  list-style: none;
  background: var(--surfacePrimary);
  border: 1px solid var(--divider);
  border-radius: 4px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.path-suggestions li {
  display: flex;
  align-items: center;
  gap: 0.4em;
  padding: 0.4em 0.7em;
  cursor: pointer;
}

.path-suggestions li.active {
  background: var(--hover);
}

.path-suggestions i {
  font-size: 1.2em;
  color: var(--blue);
}
</style>
