// 화면은 브라우저 버전(web/)을 그대로 쓴다. Go embed는 상위 폴더(../web)를 가리킬 수 없어 빌드 때 복사한다.
import { cpSync, mkdirSync, readdirSync, rmSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const web = join(here, "..", "..", "web");
const dist = join(here, "dist");
rmSync(dist, { recursive: true, force: true });
mkdirSync(dist, { recursive: true });
for (const f of readdirSync(web)) {
  if (/\.(html|js|css)$/.test(f) && !f.includes(".test.")) cpSync(join(web, f), join(dist, f));
}
console.log("web → frontend/dist:", readdirSync(dist).join(", "));
