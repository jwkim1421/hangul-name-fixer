// 의견 보내기: 구글 설문지(formResponse)로 바로 보낸다 — 쓰는 사람은 아무 데도 로그인하지 않는다.
// 결과는 설문지에 연결된 구글 시트에 한 줄씩 쌓인다.

// 설문지 "미리 채워진 링크"(칸마다 내용·연락처·버전·환경이라고 적어서 만든 것). 비어 있으면 아직 연결 전.
export const FEEDBACK_LINK =
  "https://docs.google.com/forms/d/e/1FAIpQLScZFf8ng2N-pJvPRHpz2qAyKTiX-HWFYfvR9Z-YNKHa5Q4WKA/viewform?usp=pp_url&entry.46769525=%EB%82%B4%EC%9A%A9&entry.2092205204=%EC%97%B0%EB%9D%BD%EC%B2%98&entry.138364016=%EB%B2%84%EC%A0%84&entry.2113207299=%ED%99%98%EA%B2%BD";

const LABELS = { 내용: "text", 연락처: "contact", 버전: "version", 환경: "env" };

/** 미리 채워진 링크에서 보낼 주소와 칸 번호(entry.N)를 꺼낸다. 맞지 않으면 null. */
export function formFromPrefilledLink(link) {
  let u;
  try { u = new URL(link); } catch { return null; }
  const m = u.pathname.match(/^\/forms\/d\/e\/([^/]+)\/viewform$/);
  if (u.hostname !== "docs.google.com" || !m) return null;
  const fields = {};
  for (const [k, v] of u.searchParams) if (k.startsWith("entry.") && LABELS[v]) fields[LABELS[v]] = k;
  if (Object.keys(fields).length !== 4) return null;
  return { action: `https://docs.google.com/forms/d/e/${m[1]}/formResponse`, fields };
}

/** 보낼 요청(url, body)을 만든다. 내용이 비었으면 오류. */
export function feedbackRequest(form, { text, contact, version, env }) {
  text = (text || "").trim();
  if (!text) throw new Error("내용을 적어 주세요");
  const body = new URLSearchParams();
  body.set(form.fields.text, text);
  body.set(form.fields.contact, (contact || "").trim());
  body.set(form.fields.version, version || "");
  body.set(form.fields.env, env || "");
  return { url: form.action, body: body.toString() };
}

/**
 * 보낸다. 구글은 다른 사이트에서 온 요청에 응답 내용을 보여 주지 않아(no-cors) 성공 여부는
 * "네트워크 오류가 없었다"까지만 안다. 프로그램 안에서도 윈도우 프록시 설정을 그대로 쓰도록 화면에서 보낸다.
 */
export async function sendFeedback(form, values) {
  const { url, body } = feedbackRequest(form, values);
  await fetch(url, {
    method: "POST", mode: "no-cors",
    headers: { "Content-Type": "application/x-www-form-urlencoded" }, body,
  });
}
