// crdt.js — a browser/Node port of the Go causal-tree (RGA) sequence CRDT in
// internal/crdt. Both sides run the same algorithm so the server only has to
// relay operations; each client maps ops to positions locally.
//
// An atom is a character with a unique id {seq, site} and a parent (the id it
// was inserted after). Atoms form a tree; the visible string is a pre-order DFS
// with each node's children ordered by DESCENDING id. Deletes are tombstones.
// Operations are idempotent + commutative, and out-of-order inserts are buffered
// until their parent arrives.
(function (root) {
  "use strict";

  const ROOT = { seq: 0, site: 0 };
  const key = (id) => id.seq + ":" + id.site;

  // less: ascending order over ids.
  function less(a, b) {
    if (a.seq !== b.seq) return a.seq < b.seq;
    return a.site < b.site;
  }

  class Doc {
    constructor(site) {
      this.site = site >>> 0;
      this.clock = 0;
      this.atoms = new Map();
      this.pending = new Map(); // parentKey -> [op,...]
      this.atoms.set(key(ROOT), { id: ROOT, parent: null, char: "", deleted: false, children: [] });
    }

    nextID() {
      this.clock += 1;
      return { seq: this.clock, site: this.site };
    }

    localInsert(index, ch) {
      const parent = this.parentForIndex(index);
      // Store char as an integer code point to match Go's rune JSON encoding.
      const code = typeof ch === "string" ? ch.codePointAt(0) : ch;
      const op = { type: "insert", id: this.nextID(), parent, char: code };
      this.apply(op);
      return op;
    }

    localDelete(index) {
      const vis = this.visibleIDs();
      if (index < 0 || index >= vis.length) return null;
      const op = { type: "delete", id: vis[index] };
      this.apply(op);
      return op;
    }

    apply(op) {
      if (op.type === "insert") this._applyInsert(op);
      else if (op.type === "delete") {
        const a = this.atoms.get(key(op.id));
        if (a) a.deleted = true;
      }
    }

    _applyInsert(op) {
      const k = key(op.id);
      if (this.atoms.has(k)) return; // idempotent
      const pk = key(op.parent);
      const parent = this.atoms.get(pk);
      if (!parent) {
        const buf = this.pending.get(pk) || [];
        buf.push(op);
        this.pending.set(pk, buf);
        return;
      }
      if (op.id.seq > this.clock) this.clock = op.id.seq;
      this.atoms.set(k, { id: op.id, parent: op.parent, char: op.char, deleted: false, children: [] });
      this._insertDesc(parent.children, op.id);

      const waiting = this.pending.get(k);
      if (waiting) {
        this.pending.delete(k);
        for (const w of waiting) this._applyInsert(w);
      }
    }

    // insert id into children array keeping DESCENDING order.
    _insertDesc(children, id) {
      let i = 0;
      while (i < children.length && !less(children[i], id)) i++;
      children.splice(i, 0, id);
    }

    value() {
      let out = "";
      this._walk(ROOT, (a) => {
        if (!(a.id.seq === 0 && a.id.site === 0) && !a.deleted) {
          out += String.fromCodePoint(a.char);
        }
      });
      return out;
    }

    visibleIDs() {
      const ids = [];
      this._walk(ROOT, (a) => {
        if (!(a.id.seq === 0 && a.id.site === 0) && !a.deleted) ids.push(a.id);
      });
      return ids;
    }

    _walk(id, visit) {
      const a = this.atoms.get(key(id));
      if (!a) return;
      visit(a);
      for (const c of a.children) this._walk(c, visit);
    }

    parentForIndex(index) {
      if (index <= 0) return ROOT;
      const vis = this.visibleIDs();
      if (index - 1 < vis.length) return vis[index - 1];
      if (vis.length > 0) return vis[vis.length - 1];
      return ROOT;
    }
  }

  const api = { Doc, ROOT };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  else root.CRDT = api;
})(typeof self !== "undefined" ? self : this);
