import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { asSeenOnWindows, fixName, planNames, uniqueName } from "./fix.js";

const { cases } = JSON.parse(readFileSync(new URL("../testdata/cases.json", import.meta.url)));
// 소스에 NFD 글자를 직접 적으면 편집기가 NFC로 바꿀 수 있어 코드로 만든다
const nfd = (s) => s.normalize("NFD");

test("기준 사례 (Go core와 같은 파일)", () => {
  assert.ok(cases.length > 0);
  for (const c of cases) {
    assert.deepEqual(fixName(c.input), { fixed: c.expected, changed: c.changed }, c.name);
  }
});

test("ZIP 경로는 조각마다 고친다", () => {
  const p = nfd("사진/보고서.txt");
  assert.deepEqual(planNames([p, "ok.txt"]), [{ from: p, to: "사진/보고서.txt" }]);
});

test("폴더 경로(끝 /)도 그대로 고친다", () => {
  const p = nfd("사진/");
  assert.deepEqual(planNames([p]), [{ from: p, to: "사진/" }]);
});

test("미리보기: 풀어진 이름은 윈도우에서 보이는 모양(ㅂㅗㄱㅗㅅㅓ)으로 보여 준다", () => {
  assert.equal(asSeenOnWindows(nfd("보고서.txt")), "ㅂㅗㄱㅗㅅㅓ.txt");
  assert.equal(asSeenOnWindows(nfd("각사")), "ㄱㅏㄱㅅㅏ");
  assert.equal(asSeenOnWindows("보고서.txt"), "보고서.txt");     // 정상 이름은 그대로
});

test("같은 이름이 있으면 (1), (2)… — (1)(1)처럼 잇지 않는다 (Go UniqueName과 같은 사례)", () => {
  const taken = new Set(["보고서.txt", "보고서 (1).txt", "회의록 (1).txt", "사진", "메모.tar.gz", ".env"]);
  const is = (n) => taken.has(n);
  assert.equal(uniqueName("보고서.txt", false, is), "보고서 (2).txt");
  assert.equal(uniqueName("회의록 (1).txt", false, is), "회의록 (2).txt");
  assert.equal(uniqueName("사진", true, is), "사진 (1)");
  assert.equal(uniqueName("메모.tar.gz", false, is), "메모.tar (1).gz");
  assert.equal(uniqueName(".env", false, is), ".env (1)");
  assert.equal(uniqueName("새 이름.txt", false, is), "새 이름.txt");
});
