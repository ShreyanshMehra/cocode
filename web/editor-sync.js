// editor-sync.js — pure text-diff helpers shared by the frontend, with NO DOM or
// CodeMirror dependency so they can be unit-tested under Node.
//
// Two directions:
//   - editRange(old, new): the minimal contiguous edit (a delete run + an insert
//     run) that turns `old` into `new`. Used to translate a local editor change
//     into CRDT insert/delete ops.
//   - reconcile(old, new): the same edit expressed as a CodeMirror-style
//     {from, to, insert} change. Used to apply the CRDT's converged value back
//     into the editor with a single dispatch.
(function (root) {
  "use strict";

  // commonAffix returns the lengths of the shared prefix (p) and suffix (s) of
  // a and b, with prefix and suffix guaranteed not to overlap.
  function commonAffix(a, b) {
    let p = 0;
    while (p < a.length && p < b.length && a[p] === b[p]) p++;
    let s = 0;
    while (
      s < a.length - p &&
      s < b.length - p &&
      a[a.length - 1 - s] === b[b.length - 1 - s]
    ) {
      s++;
    }
    return { p, s };
  }

  // editRange: minimal edit turning oldText into newText.
  //   pos      - index where the change starts
  //   removed  - number of chars removed at pos
  //   inserted - string inserted at pos
  function editRange(oldText, newText) {
    const { p, s } = commonAffix(oldText, newText);
    return {
      pos: p,
      removed: oldText.length - p - s,
      inserted: newText.slice(p, newText.length - s),
    };
  }

  // reconcile: the same edit as a CodeMirror change spec.
  function reconcile(oldText, newText) {
    const e = editRange(oldText, newText);
    return { from: e.pos, to: e.pos + e.removed, insert: e.inserted };
  }

  // applyToDoc translates old->new into CRDT ops on a Doc-like object exposing
  // localDelete(index) and localInsert(index, ch). Returns the ops produced (in
  // send order) so the caller can broadcast them. Deletes happen first, each at
  // `pos` (since deleting shifts subsequent chars left); then inserts at pos++.
  function applyToDoc(doc, oldText, newText) {
    const e = editRange(oldText, newText);
    const ops = [];
    for (let i = 0; i < e.removed; i++) {
      const op = doc.localDelete(e.pos);
      if (op) ops.push(op);
    }
    for (let i = 0; i < e.inserted.length; i++) {
      ops.push(doc.localInsert(e.pos + i, e.inserted[i]));
    }
    return ops;
  }

  const api = { commonAffix, editRange, reconcile, applyToDoc };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  else root.EditorSync = api;
})(typeof self !== "undefined" ? self : this);
