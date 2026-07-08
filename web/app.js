"use strict";

// cocode frontend: a CodeMirror 6 editor wired to the CRDT and a WebSocket.
// Local edits become CRDT insert/delete ops (via editor-sync); remote ops are
// applied to the CRDT and reconciled back into the editor with a single change.
// Remote participants' cursors are rendered as colored carets.

(function () {
  const room = (location.hash || "#default").slice(1) || "default";
  const site = (Math.random() * 0xffffffff) >>> 0;
  const doc = new CRDT.Doc(site);
  const Sync = window.EditorSync;

  const statusEl = document.getElementById("status");
  const participantsEl = document.getElementById("participants");
  const langSelect = document.getElementById("lang");
  document.getElementById("room").textContent = "room: " + room;

  const roomQuery = "room=" + encodeURIComponent(room);
  let selfId = null;
  let participants = [];
  let langManuallySet = false;

  // --- Editor (CodeMirror via the vendored bundle) ---
  const editor = window.CoCode.createEditor({
    parent: document.getElementById("editor"),
    doc: "",
    onLocalChange: (text) => {
      const ops = Sync.applyToDoc(doc, doc.value(), text);
      ops.forEach(send);
      maybeAutoDetectLanguage(text);
      sendCursor(editor.getCaret());
    },
    onCaret: (pos) => sendCursor(pos),
  });

  // Populate the language selector from the editor's supported languages.
  ["plaintext", ...(window.CoCode.languages || [])].sort().forEach((id) => {
    const opt = document.createElement("option");
    opt.value = id;
    opt.textContent = id;
    langSelect.appendChild(opt);
  });
  langSelect.value = "plaintext";
  langSelect.addEventListener("change", () => {
    langManuallySet = true;
    editor.setLanguage(langSelect.value);
  });

  // --- WebSocket ---
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const ws = new WebSocket(`${proto}://${location.host}/ws?${roomQuery}`);

  ws.onopen = () => setStatus("online", "online");
  ws.onclose = () => setStatus("offline", "offline");
  ws.onerror = () => setStatus("offline", "error");

  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data);
    switch (msg.type) {
      case "init":
        (msg.ops || []).forEach((op) => doc.apply(op));
        applyRemoteToEditor();
        break;
      case "welcome":
        selfId = msg.self ? msg.self.id : null;
        break;
      case "op":
        doc.apply(msg.op);
        applyRemoteToEditor();
        break;
      case "presence":
        participants = msg.participants || [];
        renderParticipants();
        renderRemoteCursors();
        break;
    }
  };

  function setStatus(cls, text) {
    statusEl.textContent = text;
    statusEl.className = "pill status-" + cls;
  }

  function send(op) {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "op", op }));
    }
  }

  // Reconcile the editor's text with the CRDT's converged value in one change.
  function applyRemoteToEditor() {
    const change = Sync.reconcile(editor.getValue(), doc.value());
    if (change.from !== change.to || change.insert !== "") {
      editor.dispatchChange(change);
    }
    renderRemoteCursors();
  }

  // --- Cursor sharing ---
  let cursorTimer = null;
  function sendCursor(pos) {
    if (cursorTimer || ws.readyState !== WebSocket.OPEN) return;
    cursorTimer = setTimeout(() => {
      cursorTimer = null;
      ws.send(JSON.stringify({ type: "cursor", cursor: pos }));
    }, 100);
  }

  function renderRemoteCursors() {
    const others = participants
      .filter((p) => p.id !== selfId)
      .map((p) => ({ name: p.name, color: p.color, pos: p.cursor || 0 }));
    editor.setCursors(others);
  }

  function renderParticipants() {
    participantsEl.innerHTML = "";
    participants.forEach((p) => {
      const chip = document.createElement("span");
      chip.className = "chip";
      const dot = document.createElement("span");
      dot.className = "dot";
      dot.style.background = p.color;
      chip.appendChild(dot);
      const label = p.id === selfId ? p.name + " (you)" : p.name;
      chip.appendChild(document.createTextNode(label));
      participantsEl.appendChild(chip);
    });
  }

  // --- Language auto-detection (server-side heuristic) ---
  let detectTimer = null;
  function maybeAutoDetectLanguage(text) {
    if (langManuallySet || detectTimer || text.length < 10) return;
    detectTimer = setTimeout(async () => {
      detectTimer = null;
      try {
        const res = await fetch(
          "/api/lang?content=" + encodeURIComponent(text.slice(0, 200))
        );
        const l = await res.json();
        if (!langManuallySet && l && l.id && l.id !== "plaintext") {
          langSelect.value = window.CoCode.languages.includes(l.id)
            ? l.id
            : "plaintext";
          editor.setLanguage(langSelect.value);
        }
      } catch (_) {
        /* ignore network errors */
      }
    }, 800);
  }

  // --- Version history + diff panel (uses the HTTP API) ---
  const saveBtn = document.getElementById("save-btn");
  const versionsEl = document.getElementById("versions");
  const diffEl = document.getElementById("diff");
  let versions = [];
  let selected = [];

  saveBtn.addEventListener("click", async () => {
    const msg = prompt("Version label:", "checkpoint");
    if (msg === null) return;
    saveBtn.disabled = true;
    try {
      await fetch(`/api/snapshot?${roomQuery}&message=${encodeURIComponent(msg)}`, {
        method: "POST",
      });
      await loadVersions();
    } finally {
      saveBtn.disabled = false;
    }
  });

  async function loadVersions() {
    const res = await fetch(`/api/versions?${roomQuery}`);
    const data = await res.json();
    versions = data.versions || [];
    renderVersions();
  }

  function renderVersions() {
    versionsEl.innerHTML = "";
    if (versions.length === 0) {
      const li = document.createElement("li");
      li.className = "muted";
      li.textContent = "No versions yet — click Save version.";
      versionsEl.appendChild(li);
      return;
    }
    versions.forEach((v) => {
      const li = document.createElement("li");
      if (selected.includes(v.id)) li.classList.add("selected");
      const msg = document.createElement("span");
      msg.className = "vmsg";
      msg.textContent = v.message || "(no label)";
      const time = document.createElement("span");
      time.className = "vtime";
      time.textContent = new Date(v.unixTime * 1000).toLocaleTimeString();
      li.appendChild(msg);
      li.appendChild(time);
      li.addEventListener("click", () => toggleSelect(v.id));
      versionsEl.appendChild(li);
    });
  }

  function toggleSelect(id) {
    const at = selected.indexOf(id);
    if (at >= 0) selected.splice(at, 1);
    else {
      selected.push(id);
      if (selected.length > 2) selected.shift();
    }
    renderVersions();
    if (selected.length === 2) loadDiff();
    else diffEl.innerHTML = '<span class="muted">Select two versions to compare.</span>';
  }

  async function loadDiff() {
    const order = versions.map((v) => v.id);
    const pair = [...selected].sort((a, b) => order.indexOf(a) - order.indexOf(b));
    const res = await fetch(`/api/diff?${roomQuery}&a=${pair[0]}&b=${pair[1]}`);
    const data = await res.json();
    renderDiff(data.diff || "");
  }

  function renderDiff(text) {
    diffEl.innerHTML = "";
    if (!text) {
      diffEl.innerHTML = '<span class="muted">No differences.</span>';
      return;
    }
    text.split("\n").forEach((line) => {
      if (line === "") return;
      const span = document.createElement("span");
      const mark = line[0];
      span.className = mark === "+" ? "add" : mark === "-" ? "del" : "ctx";
      span.textContent = line + "\n";
      diffEl.appendChild(span);
    });
  }

  loadVersions();
})();
