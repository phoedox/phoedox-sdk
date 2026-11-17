import { LitElement } from "lit";

export { html, css } from "lit";

/** Services the host passes to a plugin. Implemented by the Phoedox web app. */
export interface PhoedoxSdk {
  graph: {
    get(id: string): Promise<Entity>;
    /** Calls cb now and whenever the entry changes (polling while the page is visible). */
    watch(id: string, cb: (e: Entity) => void): () => void;
  };
  docs: { create(type: string, title?: string): Promise<{ id: string }> };
  /** Runs a tool of this plugin's backend in the current project with the user's rights. */
  tool<T = unknown>(name: string, input: unknown): Promise<T>;
  ui: { toast(message: string): void };
  commands: { add(cmd: Command): void };
  /** Lucide icon names, e.g. "battery-charging"; returns an SVG element that renders in any Lit version. */
  icons: { render(name: string, size?: number): unknown };
  /** Cards of a board of the current project, with the reader's rights. */
  tasks: { list(board: string): Promise<Task[]> };
  /** Draws a block of this project's plugins on the server (block_render, cleaned SVG). */
  blocks: { render(kind: string, source: string, width?: number): Promise<{ svg: string; problems: { line: number; message: string; severity: string }[] }> };
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
  icon?: string;
  screenshots?: { file: string; caption?: string }[];
  backend?: { wasm: string; memoryMb?: number };
  frontend?: { module: string };
  permissions?: string[];
  criticalPath?: boolean;
  contributes?: Contributes;
}

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
  /** A source to start from. */
  example?: string;
}
