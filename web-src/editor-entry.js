// editor-entry.js — bundled with esbuild into web/vendor/cocode-editor.js.
// Exposes a small window.CoCode API so the (unbundled) app.js can drive a
// CodeMirror 6 editor without importing the CM modules directly.

import { EditorView, basicSetup } from "codemirror";
import { EditorState, StateField, StateEffect, Compartment, Annotation } from "@codemirror/state";
import { Decoration, WidgetType } from "@codemirror/view";
import { oneDark } from "@codemirror/theme-one-dark";

import { javascript } from "@codemirror/lang-javascript";
import { python } from "@codemirror/lang-python";
import { html } from "@codemirror/lang-html";
import { css } from "@codemirror/lang-css";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import { cpp } from "@codemirror/lang-cpp";
import { rust } from "@codemirror/lang-rust";
import { java } from "@codemirror/lang-java";

// Map our language-detector ids to CodeMirror language extensions.
const LANGS = {
  javascript: () => javascript(),
  typescript: () => javascript({ typescript: true }),
  python: () => python(),
  html: () => html(),
  css: () => css(),
  json: () => json(),
  markdown: () => markdown(),
  cpp: () => cpp(),
  c: () => cpp(),
  rust: () => rust(),
  java: () => java(),
};

// Annotation marking a transaction as a remote/programmatic change so the
// update listener does not echo it back as a local edit.
const Remote = Annotation.define();

// --- Remote cursor decorations ---
const setCursorsEffect = StateEffect.define();

class CursorWidget extends WidgetType {
  constructor(name, color) {
    super();
    this.name = name;
    this.color = color;
  }
  eq(other) {
    return other.name === this.name && other.color === this.color;
  }
  toDOM() {
    const wrap = document.createElement("span");
    wrap.className = "cm-remote-cursor";
    wrap.style.borderLeftColor = this.color;
    const label = document.createElement("span");
    label.className = "cm-remote-label";
    label.textContent = this.name;
    label.style.backgroundColor = this.color;
    wrap.appendChild(label);
    return wrap;
  }
  ignoreEvent() {
    return true;
  }
}

const cursorsField = StateField.define({
  create() {
    return Decoration.none;
  },
  update(deco, tr) {
    for (const e of tr.effects) {
      if (e.is(setCursorsEffect)) {
        const docLen = tr.state.doc.length;
        const ranges = e.value
          .map((c) => ({ ...c, pos: Math.max(0, Math.min(c.pos, docLen)) }))
          .sort((a, b) => a.pos - b.pos)
          .map((c) =>
            Decoration.widget({
              widget: new CursorWidget(c.name, c.color),
              side: 1,
            }).range(c.pos)
          );
        return Decoration.set(ranges, true);
      }
    }
    return deco.map(tr.changes);
  },
  provide: (f) => EditorView.decorations.from(f),
});

// createEditor builds a CodeMirror editor and returns an imperative handle.
function createEditor({ parent, doc = "", onLocalChange, onCaret }) {
  const langCompartment = new Compartment();

  const listener = EditorView.updateListener.of((u) => {
    const isRemote = u.transactions.some((tr) => tr.annotation(Remote));
    if (isRemote) return;
    if (u.docChanged && onLocalChange) {
      onLocalChange(u.state.doc.toString(), u.state.selection.main.head);
    } else if (u.selectionSet && onCaret) {
      onCaret(u.state.selection.main.head);
    }
  });

  const view = new EditorView({
    parent,
    state: EditorState.create({
      doc,
      extensions: [
        basicSetup,
        oneDark,
        EditorView.lineWrapping,
        langCompartment.of([]),
        cursorsField,
        listener,
      ],
    }),
  });

  return {
    getValue: () => view.state.doc.toString(),
    getCaret: () => view.state.selection.main.head,
    // Apply a remote/programmatic change without triggering onLocalChange.
    dispatchChange: ({ from, to, insert }) => {
      view.dispatch({
        changes: { from, to: to ?? from, insert: insert ?? "" },
        annotations: [Remote.of(true)],
      });
    },
    setCursors: (list) => {
      view.dispatch({ effects: setCursorsEffect.of(list || []) });
    },
    setLanguage: (id) => {
      const factory = LANGS[id];
      view.dispatch({
        effects: langCompartment.reconfigure(factory ? factory() : []),
      });
    },
    focus: () => view.focus(),
  };
}

window.CoCode = { createEditor, languages: Object.keys(LANGS) };
