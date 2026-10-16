// 자/모음 분리 복구기 규칙 — Go core/fix.go와 같은 규칙, 같은 사례(testdata/cases.json)로 검증한다.

/** 이름을 NFC(합친 글자)로 바꾼다. */
export function fixName(name) {
  const fixed = name.normalize("NFC");
  return { fixed, changed: fixed !== name };
}

/** 경로("폴더/파일") 목록에서 고칠 것만 {from, to}로. 경로 조각마다 고친다. */
export function planNames(paths) {
  return paths
    .map((p) => ({ from: p, to: p.split("/").map((s) => fixName(s).fixed).join("/") }))
    .filter((r) => r.from !== r.to);
}

/**
 * 같은 이름이 있으면 "이름 (1).확장자", 그것도 있으면 (2), (3)… — Go core.UniqueName과 같은 규칙.
 * 이미 "이름 (1)"이면 번호를 올린다. 폴더와 점으로 시작하는 이름(.env)은 확장자가 없다.
 */
export function uniqueName(name, isDir, exists) {
  if (!exists(name)) return name;
  let stem = name, ext = "";
  const dot = name.lastIndexOf(".");
  if (!isDir && dot > 0) { stem = name.slice(0, dot); ext = name.slice(dot); }
  let n = 1;
  const m = stem.match(/^(.*) \((\d+)\)$/);
  if (m) { stem = m[1]; n = Number(m[2]) + 1; }
  for (;; n++) {
    const c = `${stem} (${n})${ext}`;
    if (!exists(c)) return c;
  }
}

// 미리보기용: 풀어진 자모(조합용 U+1100~)를 윈도우 탐색기에서 보이는 낱자(ㄱ, ㅏ …)로 바꾼다.
// 맥 화면에서는 NFD와 NFC가 똑같아 보여서, 무엇이 고쳐지는지 보이지 않는다.
const LEAD = "ㄱㄲㄴㄷㄸㄹㅁㅂㅃㅅㅆㅇㅈㅉㅊㅋㅌㅍㅎ";
const TAIL = "ㄱㄲㄳㄴㄵㄶㄷㄹㄺㄻㄼㄽㄾㄿㅀㅁㅂㅄㅅㅆㅇㅈㅊㅋㅌㅍㅎ";

export function asSeenOnWindows(name) {
  return [...name].map((ch) => {
    const c = ch.codePointAt(0);
    if (c >= 0x1100 && c <= 0x1112) return LEAD[c - 0x1100];
    if (c >= 0x1161 && c <= 0x1175) return String.fromCodePoint(0x314f + (c - 0x1161));
    if (c >= 0x11a8 && c <= 0x11c2) return TAIL[c - 0x11a8];
    return ch;
  }).join("");
}
