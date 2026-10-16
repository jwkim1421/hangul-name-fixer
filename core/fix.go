// 자/모음 분리 복구기의 규칙: 이름을 NFC(합친 글자)로 바꾼다.
package core

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Rename은 이름 하나를 고치는 계획과 결과다.
// Status: "pending" | "done" | "done-suffix"(같은 이름이 있어 To 끝에 (n)을 붙임) | "error: …"
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
	sortDeepestFirst(plan)
	return plan, err
}

// PlanPaths는 고르거나 끌어다 놓은 항목에서 고칠 이름을 모은다. 파일은 그 이름만,
// 폴더는 안쪽(Plan)과 폴더 자신의 이름까지. 같은 항목은 한 번만, 깊은 곳부터.
func PlanPaths(paths []string) []Rename {
	var plan []Rename
	seen := map[string]bool{}
	add := func(r Rename) {
		if k := filepath.Join(r.Dir, r.From); !seen[k] {
			seen[k] = true
			plan = append(plan, r)
		}
	}
	for _, p := range paths {
		p = filepath.Clean(p)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			inner, _ := Plan(p)
			for _, r := range inner {
				add(r)
			}
		}
		if fixed, changed := FixName(filepath.Base(p)); changed {
			add(Rename{Dir: filepath.Dir(p), From: filepath.Base(p), To: fixed, Status: "pending"})
		}
	}
	sortDeepestFirst(plan)
	return plan
}

func sortDeepestFirst(plan []Rename) {
	sort.SliceStable(plan, func(i, j int) bool {
		return strings.Count(plan[i].Dir, string(os.PathSeparator)) > strings.Count(plan[j].Dir, string(os.PathSeparator))
	})
}

var numbered = regexp.MustCompile(`^(.*) \((\d+)\)$`)

// UniqueName은 name이 이미 있으면 "이름 (1).확장자", 그것도 있으면 (2), (3)…을 돌려준다.
// 이미 "이름 (1)"이면 "(1) (1)"로 잇지 않고 번호를 올린다. 폴더와 점으로 시작하는 이름(.env)은 확장자가 없다.
func UniqueName(name string, isDir bool, exists func(string) bool) string {
	if !exists(name) {
		return name
	}
	stem, ext := name, ""
	if !isDir {
		if e := filepath.Ext(name); e != "" && e != name {
			stem, ext = name[:len(name)-len(e)], e
		}
	}
	n := 1
	if m := numbered.FindStringSubmatch(stem); m != nil {
		stem = m[1]
		if k, err := strconv.Atoi(m[2]); err == nil {
			n = k + 1
		}
	}
	for ; ; n++ {
		if c := fmt.Sprintf("%s (%d)%s", stem, n, ext); !exists(c) {
			return c
		}
	}
}

// Apply는 계획대로 이름을 바꾸고 결과 Status를 채워 돌려준다.
// 같은 이름이 이미 있으면 덮어쓰지 않고 UniqueName으로 (1), (2)…를 붙인다.
// listCSV가 ""가 아니면 수정 목록 "수정된 경로,원래 이름" CSV(엑셀용 BOM)를 남긴다.
func Apply(plan []Rename, listCSV string) []Rename {
	out := make([]Rename, len(plan))
	var undo [][]string
	for i, r := range plan {
		from, to := filepath.Join(r.Dir, r.From), filepath.Join(r.Dir, r.To)
		status := "done"
		if existsAsOtherFile(from, to) {
			info, _ := os.Stat(from)
			r.To = UniqueName(r.To, info != nil && info.IsDir(), func(n string) bool {
				return existsAsOtherFile(from, filepath.Join(r.Dir, n))
			})
			to, status = filepath.Join(r.Dir, r.To), "done-suffix"
		}
		if err := os.Rename(from, to); err != nil {
			r.Status = fmt.Sprintf("error: %v", err)
		} else {
			r.Status = status
			undo = append(undo, []string{to, r.From})
		}
		out[i] = r
	}
	// 자식 → 부모 순서로 바꾸므로, 먼저 적은 자식 경로는 나중에 바뀐 부모 폴더 이름을 반영해야
	// 실제 위치가 된다(윈도우에서는 옛 경로가 존재하지 않는다).
	for i := range undo {
		undo[i][0] = currentPath(undo[i][0], out)
	}
	if listCSV != "" && len(undo) > 0 {
		if f, err := os.Create(listCSV); err == nil {
			f.Write([]byte{0xEF, 0xBB, 0xBF}) // 엑셀에서 한글이 깨지지 않게 BOM
			w := csv.NewWriter(f)
			w.Write([]string{"수정된 경로", "원래 이름"})
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
			if !strings.HasPrefix(r.Status, "done") {
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
