import { test } from "node:test";
import assert from "node:assert/strict";
import { formFromPrefilledLink, feedbackRequest } from "./feedback.js";

// 구글 설문지 "미리 채워진 링크"에 각 칸 이름(내용·연락처·버전·환경)을 적어 두면 칸 번호를 알 수 있다
const LINK = "https://docs.google.com/forms/d/e/1FAIpQLSd_TEST/viewform?usp=pp_url"
  + "&entry.111=%EB%82%B4%EC%9A%A9&entry.222=%EC%97%B0%EB%9D%BD%EC%B2%98&entry.333=%EB%B2%84%EC%A0%84&entry.444=%ED%99%98%EA%B2%BD";

test("미리 채워진 링크 → 보낼 주소와 칸 번호", () => {
  assert.deepEqual(formFromPrefilledLink(LINK), {
    action: "https://docs.google.com/forms/d/e/1FAIpQLSd_TEST/formResponse",
    fields: { text: "entry.111", contact: "entry.222", version: "entry.333", env: "entry.444" },
  });
});

test("설문지 링크가 없거나 구글 설문지가 아니면 null", () => {
  assert.equal(formFromPrefilledLink(""), null);
  assert.equal(formFromPrefilledLink("https://example.com/forms/d/e/x/viewform?entry.1=내용"), null);
  assert.equal(formFromPrefilledLink("https://docs.google.com/forms/d/e/x/viewform?entry.1=다른칸"), null);
});

test("보낼 내용: 칸 번호에 값을 담는다, 연락처는 비워도 된다", () => {
  const form = formFromPrefilledLink(LINK);
  const { url, body } = feedbackRequest(form, { text: " 폴더가 안 열려요 ", contact: "", version: "v0.1.0", env: "Windows 10.0.22631" });
  assert.equal(url, form.action);
  const p = new URLSearchParams(body);
  assert.equal(p.get("entry.111"), "폴더가 안 열려요");
  assert.equal(p.get("entry.222"), "");
  assert.equal(p.get("entry.333"), "v0.1.0");
  assert.equal(p.get("entry.444"), "Windows 10.0.22631");
});

test("내용이 비었으면 보내지 않는다", () => {
  assert.throws(() => feedbackRequest(formFromPrefilledLink(LINK), { text: "  ", contact: "", version: "", env: "" }), /내용/);
});
