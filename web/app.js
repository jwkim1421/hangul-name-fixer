// 자/모음 분리 복구기 화면. 규칙은 fix.js 하나 — 여기서는 파일을 모으고, 미리 보여 주고, 누르면 고친다.
import { asSeenOnWindows, fixName, planNames, uniqueName } from "./fix.js";
import { FEEDBACK_LINK, formFromPrefilledLink, sendFeedback } from "./feedback.js";

const $ = (id) => document.getElementById(id);
let job = null; // { kind: "folder" | "zip", items: [...], run: async () => {} }

function show(items, summary) {
  $("rows").innerHTML = "";
  for (const it of items) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td class="from"></td><td></td><td class="st"></td>`;
    tr.children[0].textContent = asSeenOnWindows(it.from); // 맥에서는 NFD·NFC가 똑같아 보인다
    tr.children[1].textContent = it.to;
    tr.children[2].textContent = it.status || "대기";
    it.row = tr;
    $("rows").append(tr);
  }
  $("summary").textContent = summary;
  $("csv-line").hidden = true;
  $("result").style.display = "block";
  $("fix-all").disabled = items.length === 0;
  if (inWails) growWindow();
  $("result").scrollIntoView({ behavior: "smooth", block: "start" });
}

// 윈도우 프로그램: 시작 화면은 작게 두고, 결과 표가 나오면 창을 키운다(이미 크면 그대로).
// "처음으로"를 누르면 키우기 전 크기로 돌린다 — 그사이 사용자가 크기를 바꿨으면 그대로 둔다.
const TALL = 620;
let grownFrom = null;
async function growWindow() {
  const rt = window.runtime;
  if (!rt || !rt.WindowGetSize) return;
  const s = await rt.WindowGetSize();
  if (s.h < TALL) { grownFrom = s; rt.WindowSetSize(s.w, TALL); }
}
async function shrinkWindow() {
  const rt = window.runtime;
  if (!grownFrom || !rt || !rt.WindowGetSize) return;
  const s = await rt.WindowGetSize();
  if (s.h === TALL) rt.WindowSetSize(s.w, grownFrom.h);
  grownFrom = null;
}

const EMPTY = "수정할 파일이 없습니다.";
const DONE = ["완료", "st-done"];
const DONE_SUFFIX = ["완료 (번호 붙임)", "st-done"]; // 같은 이름이 있어 끝에 (1), (2)…를 붙였다

// 같은 이름이 있어 번호를 붙였으면 "수정된 파일" 칸도 실제 이름으로
function setTo(it, to) {
  it.to = to;
  it.row.children[1].textContent = to;
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
        let newName = fixName(it.name).fixed;
        let status = DONE;
        if (await existsAsOther(it.parent, it.handle, newName)) {
          const names = new Set();
          for await (const n of it.parent.keys()) names.add(n);
          newName = uniqueName(newName, it.handle.kind === "directory", (n) => names.has(n));
          setTo(it, it.to.slice(0, it.to.lastIndexOf("/") + 1) + newName);
          status = DONE_SUFFIX;
        }
        try {
          await it.handle.move(newName);
          setStatus(it, ...status);
        } catch (e) {
          setStatus(it, `실패: ${e.name}${e.name === "NotAllowedError" ? " — 권한이 없습니다. ZIP 방식을 쓰세요" : ""}`, "st-err");
        }
      }
    },
  };
  show(items, items.length ? `수정할 파일 ${items.length}개 (폴더: ${root.name})` : EMPTY);
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
      // 이미 정상인 이름이 먼저 자리를 잡는다 — 고친 이름이 겹치면 덮어쓰지 않고 끝에 (1), (2)…를 붙인다.
      // 폴더가 겹치면 같은 폴더로 합친다(안의 파일은 각자 위 규칙을 따른다).
      for (const e of entries) if (!plan.has(e.path)) await put(e, e.path);
      for (const e of entries) {
        if (!plan.has(e.path)) continue;
        const it = items.find((i) => i.from === e.path);
        const to = plan.get(e.path);
        if (!used.has(to) || e.dir) {
          if (!used.has(to)) await put(e, to);
          setStatus(it, ...DONE);
        } else {
          const cut = to.lastIndexOf("/") + 1;
          const dir = to.slice(0, cut);
          const path = dir + uniqueName(to.slice(cut), false, (n) => used.has(dir + n));
          await put(e, path);
          setTo(it, path);
          setStatus(it, ...DONE_SUFFIX);
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
  show(items, items.length ? `수정할 파일 ${items.length}개 — "수정"을 누르면 고친 ZIP을 내려받습니다` : EMPTY);
}

// ── ③ 윈도우 프로그램(Wails) 안: 폴더·파일을 고르면 Go(core)가 이름을 바꾼다 ──────────
const inWails = typeof window.go !== "undefined";
const goApp = () => window.go.main.App;
let makeCSV = false; // "수정 목록(*.csv) 파일 생성" — 프로그램이 설정 파일에 기억한다

// kind: "folder"(폴더 선택) | "files"(파일 선택) | "drop"(창에 끌어다 놓기 — dropped에 경로 목록)
async function wailsPick(kind, dropped) {
  let picked = dropped;
  if (!picked) {
    try {
      picked = kind === "folder" ? await goApp().ChooseFolder() : await goApp().ChooseFiles();
    } catch (e) {
      alert(`${kind === "folder" ? "폴더" : "파일"}를 열지 못했습니다: ` + e);
      return;
    }
  }
  if (!picked || picked.length === 0) return; // 취소
  const folder = kind === "folder";
  const first = folder ? picked : picked[0];
  const sep = first.includes("\\") ? "\\" : "/";
  // 표에 보일 경로: 폴더 선택이면 그 폴더 기준, 끌어다 놓은 폴더 안쪽이면 "그 폴더\…", 그 밖은 이름만
  const bases = folder ? [picked] : picked;
  const rel = (dir) => {
    for (const b of bases) {
      if (dir === b || dir.startsWith(b + sep)) {
        const from = folder ? b.length + 1 : b.lastIndexOf(sep) + 1;
        return dir.length >= from ? dir.slice(from) + sep : "";
      }
    }
    return "";
  };
  const fixPath = (p) => p.split(sep).map((s) => fixName(s).fixed).join(sep);
  const plan = (await (folder ? goApp().Preview(picked) : goApp().PreviewPaths(picked))) || [];
  const items = plan.map((r) => ({ key: r.Dir + sep + r.From, prefix: fixPath(rel(r.Dir)), from: rel(r.Dir) + r.From }));
  items.forEach((it, i) => (it.to = it.prefix + plan[i].To));
  job = {
    kind: "wails", items,
    run: async () => {
      let res;
      try {
        res = await (folder ? goApp().FixAll(picked, makeCSV) : goApp().FixPaths(picked, makeCSV));
      } catch (e) {
        $("summary").textContent = "수정하지 못했습니다: " + e;
        return;
      }
      const byKey = new Map((res.Items || []).map((r) => [r.Dir + sep + r.From, r]));
      let failed = 0;
      for (const it of items) {
        const r = byKey.get(it.key);
        const st = r ? r.Status : "error: 다시 확인 필요";
        if (st === "done") setStatus(it, ...DONE);
        else if (st === "done-suffix") { setTo(it, it.prefix + r.To); setStatus(it, ...DONE_SUFFIX); }
        else { failed++; setStatus(it, `실패: ${st.replace(/^error: /, "")}`, "st-err"); }
      }
      $("summary").textContent = failed ? `수정 완료 — 실패 ${failed}개는 상태 칸을 확인하세요` : "수정 완료";
      if (res.CSV) {
        $("csv-path").textContent = res.CSV;
        $("open-csv").onclick = () => goApp().RevealInFolder(res.CSV).catch((e) => alert("폴더를 열지 못했습니다: " + e));
        $("csv-line").hidden = false;
      }
    },
  };
  const where = folder ? `폴더: ${picked}` : `${kind === "drop" ? "끌어다 놓은" : "선택한"} 항목 ${picked.length}개 중`;
  show(items, items.length ? `수정할 파일 ${items.length}개 (${where})` : EMPTY);
}

// ── 연결 ───────────────────────────────────────────────────────────────
if (inWails) {
  document.body.classList.add("app");
  $("privacy").textContent = "🔒 파일 내용에 접근하지 않으며, 데이터를 외부로 전송하지 않습니다. — 로컬 환경에서 파일명만 수정합니다.";
  $("mode-folder").hidden = false;
  $("mode-zip").hidden = true;
  $("folder-title").textContent = "폴더·파일 선택하여 수정";
  $("folder-desc").textContent = "폴더나 파일을 선택하거나 이 창에 끌어다 놓으면 수정할 파일이 표시됩니다.";
  $("pick-app-files").hidden = false;
  $("csv-option").hidden = false;
  $("pick-folder").addEventListener("click", () => wailsPick("folder"));
  $("pick-app-files").addEventListener("click", () => wailsPick("files"));
  // 창에 파일·폴더를 끌어다 놓기(여러 개). 경로는 Wails가 넘겨준다(main.go DragAndDrop.EnableFileDrop)
  if (window.runtime && window.runtime.OnFileDrop) {
    window.runtime.OnFileDrop((x, y, paths) => wailsPick("drop", paths), false);
  }
  let dragDepth = 0;
  const isFiles = (e) => e.dataTransfer && [...e.dataTransfer.types].includes("Files");
  const dragEnd = () => { dragDepth = 0; document.body.classList.remove("dragging"); };
  window.addEventListener("dragenter", (e) => { if (isFiles(e)) { dragDepth++; document.body.classList.add("dragging"); } });
  window.addEventListener("dragleave", (e) => { if (isFiles(e) && --dragDepth <= 0) dragEnd(); });
  window.addEventListener("drop", dragEnd);
  goApp().GetSettings().then((s) => { makeCSV = !!(s && s.MakeCSV); $("make-csv").checked = makeCSV; }).catch(() => {});
  $("make-csv").addEventListener("change", (e) => {
    makeCSV = e.target.checked;
    goApp().SaveSettings({ MakeCSV: makeCSV }).catch(() => {});
  });
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
// ── 의견 보내기(구글 설문지 — 로그인 없음) ─────────────────────────────────
const feedbackForm = formFromPrefilledLink(FEEDBACK_LINK);
const feedbackInfo = inWails
  ? Promise.resolve().then(() => goApp().GetInfo()).catch(() => ({ Version: "?", OS: "?" }))
    .then((i) => ({ version: i.Version, env: i.OS }))
  : Promise.resolve({ version: "웹", env: navigator.userAgent });
function fbStatus(text, cls) {
  $("fb-status").textContent = text;
  $("fb-status").className = cls || "";
}
$("open-feedback").addEventListener("click", async () => {
  const info = await feedbackInfo;
  $("fb-info").textContent = `함께 보내는 정보: 버전 ${info.version} · ${info.env} — 파일 이름·내용은 보내지 않습니다`;
  fbStatus("");
  $("fb-send").disabled = false;
  $("feedback").showModal();
  $("fb-text").focus();
});
$("fb-close").addEventListener("click", () => $("feedback").close());
$("fb-text").addEventListener("input", () => fbStatus(""));
// 공개된 웹 버전에서는 설문지를 연결하기 전까지 링크를 숨긴다(프로그램 시험판에서는 창을 볼 수 있게 둔다)
if (!feedbackForm && !inWails) {
  $("open-feedback").hidden = true;
  $("open-feedback").previousSibling.textContent = ""; // 앞의 " · " 구분자도 지운다
}
$("fb-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  if (!feedbackForm) return fbStatus("설문지가 아직 연결되지 않았습니다(준비 중).", "err");
  if (!$("fb-text").value.trim()) return fbStatus("내용을 적어 주세요.", "err");
  $("fb-send").disabled = true;
  fbStatus("보내는 중…");
  try {
    await sendFeedback(feedbackForm, { text: $("fb-text").value, contact: $("fb-contact").value, ...(await feedbackInfo) });
    fbStatus("보냈습니다. 고맙습니다!", "ok");
    $("fb-text").value = "";
    setTimeout(() => $("feedback").open && $("feedback").close(), 1500);
  } catch {
    fbStatus("보내지 못했습니다 — 인터넷 연결을 확인하고 다시 시도해 주세요.", "err");
  } finally {
    $("fb-send").disabled = false;
  }
});

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
$("reset").addEventListener("click", () => {
  job = null;
  $("csv-line").hidden = true;
  $("result").style.display = "none";
  if (inWails) shrinkWindow();
});
