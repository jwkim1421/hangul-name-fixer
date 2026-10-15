import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { asSeenOnWindows, fixName, planNames } from "./fix.js";

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
