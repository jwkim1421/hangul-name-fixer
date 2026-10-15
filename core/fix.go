// 자/모음 분리 복구기의 규칙: 이름을 NFC(합친 글자)로 바꾼다.
package core

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Rename은 이름 하나를 고치는 계획과 결과다.
// Status: "pending" | "done" | "skipped-exists" | "error: …"
type Rename struct {
	Dir, From, To string
	Status        string
}

// FixName은 이름을 NFC로 바꾼다. 바뀌었으면 changed가 true.
func FixName(name string) (string, bool) {
	fixed := norm.NFC.String(name)
	return fixed, fixed != name
}

// Plan은 root 아래(파일·폴더)에서 고칠 이름을 깊은 곳부터(자식 → 부모) 모은다.
func Plan(root string) ([]Rename, error) {
	var plan []Rename
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return err
		}
		if fixed, changed := FixName(d.Name()); changed {
			plan = append(plan, Rename{Dir: filepath.Dir(path), From: d.Name(), To: fixed, Status: "pending"})
		}
		return nil
	})
	// 깊은 곳부터 바꿔야 부모 폴더 이름이 바뀌어도 자식 경로가 유효하다
	sort.SliceStable(plan, func(i, j int) bool {
		return strings.Count(plan[i].Dir, string(os.PathSeparator)) > strings.Count(plan[j].Dir, string(os.PathSeparator))
	})
	return plan, err
}

// Apply는 계획대로 이름을 바꾸고 결과 Status를 채워 돌려준다.
// undoCSV가 ""가 아니면 "현재 경로,원래 이름" CSV(엑셀용 BOM)를 남긴다.
func Apply(plan []Rename, undoCSV string) []Rename {
	out := make([]Rename, len(plan))
	var undo [][]string
	for i, r := range plan {
		from, to := filepath.Join(r.Dir, r.From), filepath.Join(r.Dir, r.To)
		if existsAsOtherFile(from, to) {
			r.Status = "skipped-exists"
		} else if err := os.Rename(from, to); err != nil {
			r.Status = fmt.Sprintf("error: %v", err)
		} else {
			r.Status = "done"
			undo = append(undo, []string{to, r.From})
		}
		out[i] = r
	}
	// 자식 → 부모 순서로 바꾸므로, 먼저 적은 자식 경로는 나중에 바뀐 부모 폴더 이름을 반영해야
	// 실제 위치가 된다(윈도우에서는 옛 경로가 존재하지 않는다).
	for i := range undo {
		undo[i][0] = currentPath(undo[i][0], out)
	}
	if undoCSV != "" && len(undo) > 0 {
		if f, err := os.Create(undoCSV); err == nil {
			f.Write([]byte{0xEF, 0xBB, 0xBF}) // 엑셀에서 한글이 깨지지 않게 BOM
			w := csv.NewWriter(f)
			w.Write([]string{"현재 경로", "원래 이름"})
			w.WriteAll(undo)
			f.Close()
		}
	}
	return out
}

// currentPath는 path의 상위 폴더 중 이름이 바뀐(done) 것을 새 이름으로 바꿔 돌려준다.
func currentPath(path string, results []Rename) string {
	for changed := true; changed; {
		changed = false
		for _, r := range results {
			if r.Status != "done" {
				continue
			}
			oldDir := filepath.Join(r.Dir, r.From) + string(os.PathSeparator)
			if strings.HasPrefix(path, oldDir) {
				path = filepath.Join(r.Dir, r.To) + string(os.PathSeparator) + path[len(oldDir):]
				changed = true
			}
		}
	}
	return path
}

// to가 from과 다른 파일로 이미 있으면 true. APFS처럼 NFD/NFC를 같은 이름으로 보는 곳에서는
// to를 Stat하면 from 자신이 나오므로 SameFile로 구분한다.
func existsAsOtherFile(from, to string) bool {
	toInfo, err := os.Stat(to)
	if err != nil {
		return false
	}
	fromInfo, err := os.Stat(from)
	if err != nil || fromInfo == nil {
		return true
	}
	return !os.SameFile(fromInfo, toInfo)
}
