import { LitElement } from "lit";

export { html, css } from "lit";

/** Services the host passes to a plugin. Implemented by the Phoedox web app. */
export interface PhoedoxSdk {
  graph: {
    get(id: string): Promise<Entity>;
    /** Calls cb now and whenever the entry changes (polling while the page is visible). */
    watch(id: string, cb: (e: Entity) => void): () => void;
  };
  docs: {
    /**
     * Creates a document in the current project with the user's rights. `content` adds the
     * fields of the documents API, e.g. `{ markdown }` or, for decisions (type "adr"),
     * `{ adr: { status: "proposed", context, options: [{ title, description }] } }`.
     */
    create(type: string, title?: string, content?: Record<string, unknown>): Promise<{ id: string; slug: string }>;
  };
  /** Runs a tool of this plugin's backend in the current project with the user's rights. */
  tool<T = unknown>(name: string, input: unknown): Promise<T>;
  ui: {
    /** A short message; "error" shows it as a failure. */
    toast(message: string, kind?: "info" | "error"): void;
    /** The language of the interface ("de", "en", …), for the plugin's own texts. */
    locale(): string;
    /** Opens a page of the current project in the app, e.g. "d/<slug>" or "e/<entry>". */
    open(path: string): void;
    /**
     * Copies whiteboard elements (the format of save_whiteboard: shapes, texts, arrows …)
     * so that the user can paste them onto a whiteboard.
     */
    copyToWhiteboard(elements: unknown[]): Promise<void>;
  };
  commands: { add(cmd: Command): void };
  /** Lucide icon names, e.g. "battery-charging"; returns an SVG element that renders in any Lit version. */
  icons: { render(name: string, size?: number): unknown };
  /** Cards of a board of the current project, with the reader's rights. */
  tasks: { list(board: string): Promise<Task[]> };
  /** Draws a block of this project's plugins on the server (block_render, cleaned SVG). */
  blocks: {
    render(kind: string, source: string, width?: number): Promise<{ svg: string; problems: { line: number; message: string; severity: string }[] }>;
    /**
     * The reader's state of a block on a page or whiteboard (a JSON object up to 16 KB; null removes it).
     * Signed-in readers keep it on the server; elsewhere (no doc) it stays in the browser.
     */
    state<T = unknown>(ref: { doc: string; key: string }): { get(): Promise<BlockState<T>>; put(data: T | null): Promise<BlockState<T>> };
  };
}

/** A card of a board. */
export interface Task {
  id: string;
  title: string;
  column: string;
  due?: string;
  createdAt: string;
  doneAt?: string | null;
  boardTitle?: string;
  labels: string[];
}

export interface Entity {
  id: string;
  kind: string;
  slug: string;
  name: string;
  attrs: Record<string, unknown>;
}

export interface Command {
  id: string;
  label: string;
  run(): void | Promise<void>;
}

/**
 * A content block's interactive element (ADR-0086). The manifest declares the block
 * (`contributes.blocks`); the backend export `block_render` draws it as SVG everywhere.
 * Where the frontend runs (web app, and the mobile app in a WebView), the host creates
 * `element` instead and sets `source` (the block's text), `svg` (the backend's picture,
 * a good first paint) and `project` before connecting it.
 */
export interface BlockDef {
  id: string;
  element: string;
  icon?: string;
}

/** The properties the host sets on a block element. */
export interface BlockElement {
  source: string;
  svg: string;
  project: string;
  /** The page or whiteboard the block is on ("" in previews). */
  doc: string;
  /**
   * The key of the block's state: the whiteboard element's id, else a hash of the
   * trimmed source. A plugin may use its own id from the source instead (1–40 of A–Z a–z 0–9 -).
   */
  stateKey: string;
}

/** A reader's saved state of a block and, for polls (state `{choices: [...]}`), the votes of all readers. */
export interface BlockState<T = unknown> {
  mine: T | null;
  /** choice → number of readers who chose it (never who). */
  votes?: Record<string, number>;
  voters?: number;
}

export interface PluginDef {
  elements?: Record<string, CustomElementConstructor>;
  blocks?: BlockDef[];
  /**
   * Element shown on entry pages of a kind: kind -> element tag. Gets the entry slug as attribute `entity`.
   * Works for the plugin's own kinds and for core kinds (e.g. device); when an entry is not its business the
   * element sets its `hidden` attribute and the host hides the frame around it.
   */
  views?: Record<string, string>;
  onReady?(sdk: PhoedoxSdk): void | Promise<void>;
}

/** Base class for plugin elements. The host injects `sdk` before connecting. */
export class PhxElement extends LitElement {
  sdk!: PhoedoxSdk;
}

const tagRe = /^phx-[a-z0-9]+(-[a-z0-9]+)*$/;

/**
 * Declares a plugin. Element names must start with "phx-" and every block
 * must reference an element the plugin defines.
 */
export function definePlugin(def: PluginDef): PluginDef {
  const elements = def.elements ?? {};
  for (const tag of Object.keys(elements)) {
    if (!tagRe.test(tag)) throw new Error(`definePlugin: element "${tag}" must match phx-<name>`);
  }
  for (const [kind, tag] of Object.entries(def.views ?? {})) {
    if (!(tag in elements)) throw new Error(`definePlugin: view for "${kind}" uses unknown element "${tag}"`);
  }
  for (const b of def.blocks ?? []) {
    if (!(b.element in elements)) throw new Error(`definePlugin: block "${b.id}" uses unknown element "${b.element}"`);
  }
  return def;
}

/** Item types of the store (manifest `type`). */
export type ItemType = "plugin" | "theme" | "app";

/**
 * phoedox-plugin.json (see schema/phoedox-plugin.schema.json and go/pdk). Themes carry theme.json and an
 * optional theme.css, app templates app.json and files/…; neither has a backend, frontend or permissions.
 */
export interface Manifest {
  type?: ItemType;
  id: string;
  name: string;
  version: string;
  /** Required for plugins. */
  hostApi?: string;
  engines: { phoedox: string };
  description?: string;
  /** Lucide icon name; with category it makes the plugin's tile. */
  icon?: string;
  /** Store section and tile colour (default other). */
  category?: Category;
  /** SVG at the package root shown instead of the tile (the product's own logo where its brand allows it). */
  logo?: string;
  /** Variant of logo for dark backgrounds. */
  logoDark?: string;
  screenshots?: { file: string; caption?: string }[];
  backend?: { wasm: string; memoryMb?: number };
  frontend?: { module: string };
  permissions?: string[];
  criticalPath?: boolean;
  contributes?: Contributes;
}

/** Store categories (manifest `category`); each has a tile colour. */
export const categories = ["home", "network", "servers", "cloud", "monitoring", "development", "records", "blocks", "other"] as const;
export type Category = (typeof categories)[number];

/** What a plugin adds to Phoedox (manifest `contributes`). */
export interface Contributes {
  kinds?: { id: string; schema?: string; icon?: string }[];
  relations?: { id: string; from: string[]; to: string[]; card?: string }[];
  mcpTools?: { name: string; description?: string; input?: string; control?: boolean; timeoutSec?: number; target?: { arg: string; attr?: string } }[];
  events?: string[];
  /** Content blocks for pages; the backend must export block_render. */
  blocks?: ManifestBlock[];
}

export interface ManifestBlock {
  /** The fenced block's info string: 2–30 lowercase letters and digits. */
  id: string;
  label: string;
  /** Lucide icon name. */
  icon?: string;
  /** phx-<name> element of the frontend that shows the block interactively. */
  element?: string;
  /** May be placed on whiteboards. */
  board?: boolean;
  /** On whiteboards the element runs interactively (e.g. a poll); needs board and element. */
  live?: boolean;
  /** Size on a whiteboard (80–2000 each). */
  size?: { w: number; h: number };
  /** A source to start from. */
  example?: string;
}
