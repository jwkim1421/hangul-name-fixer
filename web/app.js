// 자/모음 분리 복구기 화면. 규칙은 fix.js 하나 — 여기서는 파일을 모으고, 미리 보여 주고, 누르면 고친다.
import { asSeenOnWindows, fixName, planNames } from "./fix.js";

const $ = (id) => document.getElementById(id);
let job = null; // { kind: "folder" | "zip", items: [...], run: async () => {} }

function show(items, summary) {
  $("rows").innerHTML = "";
  for (const it of items) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td class="from"></td><td></td><td class="st"></td>`;
    tr.children[0].textContent = asSeenOnWindows(it.from); // 맥에서는 NFD·NFC가 똑같아 보인다
    tr.children[1].textContent = it.to;
    tr.children[2].textContent = it.status || "고칠 예정";
    it.row = tr;
    $("rows").append(tr);
  }
  $("summary").textContent = summary;
  $("result").style.display = "block";
  $("fix-all").disabled = items.length === 0;
  $("result").scrollIntoView({ behavior: "smooth", block: "start" });
}

function setStatus(it, text, cls) {
  it.status = text;
  const td = it.row.children[2];
  td.textContent = text;
  td.className = "st " + (cls || "");
}

// ── ① 폴더 바로 고치기 (File System Access API) ─────────────────────────
async function collect(dir, path, depth, out) {
  for await (const [name, handle] of dir.entries()) {
    const p = path ? `${path}/${name}` : name;
    if (handle.kind === "directory") await collect(handle, p, depth + 1, out);
    const { fixed, changed } = fixName(name);
    if (changed) out.push({ handle, parent: dir, name, to: fixed, from: p, depth,
      toPath: p.split("/").slice(0, -1).map((s) => fixName(s).fixed).concat(fixed).join("/") });
  }
}

async function existsAsOther(parent, handle, name) {
  for (const get of [(n) => parent.getFileHandle(n), (n) => parent.getDirectoryHandle(n)]) {
    try {
      const other = await get(name);
      return !(await other.isSameEntry(handle));
    } catch { /* 없음 */ }
  }
  return false;
}

async function pickFolder() {
  let root;
  try {
    root = await window.showDirectoryPicker({ mode: "readwrite" });
  } catch (e) {
    if (e.name !== "AbortError") alert("폴더를 열지 못했습니다: " + e.message);
    return;
  }
  const found = [];
  await collect(root, "", 0, found);
  found.sort((a, b) => b.depth - a.depth); // 깊은 곳부터 — 부모 이름이 바뀌어도 자식 핸들은 그대로
  const items = found.map((f) => ({ ...f, to: f.toPath }));
  job = {
    kind: "folder", items,
    run: async () => {
      for (const it of items) {
        if (typeof it.handle.move !== "function") {
          setStatus(it, "이 브라우저는 이름 바꾸기를 지원하지 않습니다 — ZIP 방식을 쓰세요", "st-err");
          continue;
        }
        const newName = fixName(it.name).fixed;
        if (await existsAsOther(it.parent, it.handle, newName)) {
          setStatus(it, "같은 이름이 있어 건너뜀", "st-skip");
          continue;
        }
        try {
          await it.handle.move(newName);
          setStatus(it, "고침", "st-done");
        } catch (e) {
          setStatus(it, `실패: ${e.name}${e.name === "NotAllowedError" ? " — 권한이 없습니다. ZIP 방식을 쓰세요" : ""}`, "st-err");
        }
      }
    },
  };
  show(items, items.length ? `고칠 이름 ${items.length}개 (폴더: ${root.name})` : "고칠 이름이 없습니다. 모두 정상입니다.");
}

// ── ② 고친 ZIP 받기 ────────────────────────────────────────────────────
// 맥에서 만든 ZIP은 이름에 UTF-8 표시가 없을 때가 있다 — 그래도 UTF-8로 읽는다
const decodeFileName = (bytes) => new TextDecoder("utf-8").decode(bytes);

async function fromFiles(files) {
  files = [...files];
  if (!files.length) return;
  const zips = files.filter((f) => /\.zip$/i.test(f.name));
  const entries = []; // { path, getData, date, dir }
  for (const f of files) {
    if (zips.includes(f)) {
      const z = await JSZip.loadAsync(f, { decodeFileName });
      z.forEach((path, e) => entries.push({ path, dir: e.dir, date: e.date, getData: () => e.async("uint8array") }));
    } else {
      const path = f.webkitRelativePath || f.name;
      entries.push({ path, dir: false, date: new Date(f.lastModified),
        getData: async () => new Uint8Array(await f.arrayBuffer()) });
    }
  }
  const plan = new Map(planNames(entries.map((e) => e.path)).map((r) => [r.from, r.to]));
  const items = entries.filter((e) => plan.has(e.path)).map((e) => ({ from: e.path, to: plan.get(e.path) }));
  job = {
    kind: "zip", items,
    run: async () => {
      const out = new JSZip();
      const used = new Set();
      const put = async (e, path) => {
        used.add(path);
        if (e.dir) out.folder(path);
        else out.file(path, await e.getData(), { date: e.date });
      };
      // 이미 정상인 이름이 먼저 자리를 잡는다 — 고친 이름이 겹치면 덮어쓰지 않고 원래 이름으로 둔다
      for (const e of entries) if (!plan.has(e.path)) await put(e, e.path);
      for (const e of entries) {
        if (!plan.has(e.path)) continue;
        const it = items.find((i) => i.from === e.path);
        const to = plan.get(e.path);
        if (used.has(to)) {
          await put(e, e.path);
          setStatus(it, "같은 이름이 있어 건너뜀", "st-skip");
        } else {
          await put(e, to);
          setStatus(it, "고침", "st-done");
        }
      }
      const blob = await out.generateAsync({ type: "blob" });
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = "고친_파일.zip";
      a.click();
      setTimeout(() => URL.revokeObjectURL(a.href), 10_000);
    },
  };
  show(items, items.length ? `고칠 이름 ${items.length}개 — "모두 고치기"를 누르면 고친 ZIP을 내려받습니다`
    : "고칠 이름이 없습니다. 모두 정상입니다.");
}

// ── ③ 윈도우 프로그램(Wails) 안: 폴더를 고르면 Go(core)가 이름을 바꾼다 ──────────
const inWails = typeof window.go !== "undefined";
const STATUS = { done: ["고침", "st-done"], "skipped-exists": ["같은 이름이 있어 건너뜀", "st-skip"] };

async function wailsFolder() {
  const app = window.go.main.App;
  let root;
  try {
    root = await app.ChooseFolder();
  } catch (e) {
    alert("폴더를 열지 못했습니다: " + e);
    return;
  }
  if (!root) return; // 취소
  const sep = root.includes("\\") ? "\\" : "/";
  const rel = (dir) => (dir.length > root.length ? dir.slice(root.length + 1) + sep : "");
  const fixPath = (p) => p.split(sep).map((s) => fixName(s).fixed).join(sep);
  const plan = (await app.Preview(root)) || [];
  const items = plan.map((r) => ({ key: r.Dir + sep + r.From, from: rel(r.Dir) + r.From, to: fixPath(rel(r.Dir)) + r.To }));
  job = {
    kind: "wails", items,
    run: async () => {
      const res = (await app.FixAll(root)) || [];
      const byKey = new Map(res.map((r) => [r.Dir + sep + r.From, r.Status]));
      for (const it of items) {
        const st = byKey.get(it.key) || "error: 다시 확인 필요";
        const [text, cls] = STATUS[st] || [`실패: ${st.replace(/^error: /, "")}`, "st-err"];
        setStatus(it, text, cls);
      }
      $("summary").textContent = "다 고쳤습니다. 원래 이름은 그 폴더의 되돌리기_날짜.csv에 남겼습니다.";
    },
  };
  show(items, items.length ? `고칠 이름 ${items.length}개 (폴더: ${root})` : "고칠 이름이 없습니다. 모두 정상입니다.");
}

// ── 연결 ───────────────────────────────────────────────────────────────
if (inWails) {
  $("mode-folder").hidden = false;
  $("mode-zip").hidden = true;
  $("mode-folder").querySelector("p").textContent =
    "폴더를 고르면 안의 파일·폴더 이름을 고치고, 원래 이름을 되돌리기 목록(CSV)으로 그 폴더에 남깁니다.";
  $("pick-folder").addEventListener("click", wailsFolder);
  // 프로그램 안에서 바깥 링크를 누르면 기본 브라우저로 연다(창이 그 페이지로 바뀌지 않게)
  document.addEventListener("click", (e) => {
    const a = e.target.closest("a[href^='http']");
    if (a && window.runtime) { e.preventDefault(); window.runtime.BrowserOpenURL(a.href); }
  });
} else if ("showDirectoryPicker" in window) {
  $("mode-folder").hidden = false;
  $("modes").classList.add("two");
  $("pick-folder").addEventListener("click", pickFolder);
}
$("pick-files").addEventListener("change", (e) => fromFiles(e.target.files));
$("pick-dir").addEventListener("change", (e) => fromFiles(e.target.files));
const drop = $("drop");
drop.addEventListener("dragover", (e) => { e.preventDefault(); drop.classList.add("over"); });
drop.addEventListener("dragleave", () => drop.classList.remove("over"));
drop.addEventListener("drop", (e) => { e.preventDefault(); drop.classList.remove("over"); fromFiles(e.dataTransfer.files); });
$("fix-all").addEventListener("click", async () => {
  if (!job) return;
  $("fix-all").disabled = true;
  await job.run();
});
$("reset").addEventListener("click", () => { job = null; $("result").style.display = "none"; });
