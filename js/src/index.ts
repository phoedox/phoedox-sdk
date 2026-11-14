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
  /** Lucide icon names, e.g. "battery-charging". */
  icons: { render(name: string, size?: number): unknown };
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

export interface BlockDef {
  id: string;
  element: string;
  icon?: string;
}

export interface PluginDef {
  elements?: Record<string, CustomElementConstructor>;
  blocks?: BlockDef[];
  /**
   * Element shown on entry pages of a kind: kind -> element tag. Gets the entry slug as attribute `entity`.
   * Works for the plugin's own kinds and for core kinds (e.g. device); the element renders nothing when the entry is not its business.
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
