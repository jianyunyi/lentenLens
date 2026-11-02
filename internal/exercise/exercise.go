// Package exercise implements bounded, offline single-package exercises.
package exercise

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const AcceptanceFile = "intentlens_acceptance_test.go"
const Module = "module intentlens.exercise\n\ngo 1.24.0\n"
const MissingMarker = "INTENTLENS_EXPECTED_MISSING"

type Requirement struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Test string `json:"test"`
}
type Spec struct {
	Title        string            `json:"title"`
	Requirements []Requirement     `json:"requirements"`
	Baseline     map[string]string `json:"baseline"`
	Acceptance   string            `json:"acceptance"`
	Reference    map[string]string `json:"reference,omitempty"`
}
type CheckResult struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Test   string `json:"test"`
	Status string `json:"status"`
}

var fileName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*\.go$`)
var testName = regexp.MustCompile(`^TestIntentLens[A-Za-z0-9_]+$`)
var imports = strings.Fields("bytes cmp container/heap container/list container/ring context encoding/base64 encoding/binary encoding/hex encoding/json errors fmt hash hash/crc32 hash/fnv io math math/big math/bits math/rand math/rand/v2 path regexp slices sort strconv strings sync sync/atomic testing time unicode unicode/utf8 unicode/utf16")

func Hash(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		// Hash has a fixed, non-error-returning API; keep failures distinct
		// instead of silently hashing an empty serialization.
		return hashBytes([]byte("marshal-error:" + fmt.Sprintf("%T", v)))
	}
	return hashBytes(b)
}
func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func Version(s Spec) string { s.Reference = nil; return Hash(s) }
func SafeName(name string) bool {
	return fileName.MatchString(name) && !strings.HasPrefix(strings.ToLower(name), "intentlens_")
}

// Qualify never invokes Go or project code. The whitelist is a scope limit, not an OS sandbox.
func Qualify(files map[string]string) error {
	if len(files) == 0 || len(files) > 20 {
		return errors.New("练习仅支持 1–20 个文件的独立 Go 包")
	}
	total := 0
	seen := map[string]bool{}
	pkg := ""
	for name, source := range files {
		lower := strings.ToLower(name)
		if (!SafeName(name) && name != AcceptanceFile) || seen[lower] {
			return errors.New("文件名不安全、保留名称或大小写冲突")
		}
		seen[lower] = true
		total += len(source)
		if total > 60000 {
			return errors.New("练习包超过 60 KiB 读取范围")
		}
		if strings.Contains(source, "//go:") || strings.Contains(source, "// +build") {
			return errors.New("暂不支持 Go 编译指令或嵌入资源")
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("%s 无法解析 Go 语法", name)
		}
		p := strings.TrimSuffix(f.Name.Name, "_test")
		if pkg == "" {
			pkg = p
		}
		if p != pkg {
			return errors.New("练习必须为单个包")
		}
		for _, im := range f.Imports {
			path, err := strconv.Unquote(im.Path.Value)
			if err != nil {
				return err
			}
			found := false
			for _, allowed := range imports {
				if path == allowed {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("依赖 %s 不在本地练习允许范围；不下载依赖或连接服务", path)
			}
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "TestMain" {
				return errors.New("暂不支持 TestMain 自定义测试进程")
			}
		}
	}
	return nil
}
func Check(s Spec) error {
	if s.Title == "" || len(s.Title) > 300 || len(s.Requirements) == 0 || len(s.Requirements) > 6 {
		return errors.New("题目需要标题与 1–6 项行为要求")
	}
	for name := range s.Baseline {
		if !SafeName(name) {
			return errors.New("基线文件名不可用")
		}
	}
	if err := Qualify(s.Baseline); err != nil {
		return err
	}
	for name := range s.Reference {
		if _, ok := s.Baseline[name]; !ok || strings.HasSuffix(name, "_test.go") {
			return errors.New("参考修改只能替换基线非测试 Go 文件")
		}
	}
	if err := Qualify(Files(s, false)); err != nil {
		return err
	}
	if err := Qualify(Files(s, true)); err != nil {
		return err
	}
	f, err := parser.ParseFile(token.NewFileSet(), AcceptanceFile, s.Acceptance, 0)
	if err != nil {
		return err
	}
	tests := map[string]bool{}
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Recv != nil || !testName.MatchString(fn.Name.Name) {
				return errors.New("验收文件仅允许 TestIntentLens 前缀的测试函数")
			}
			tests[fn.Name.Name] = true
		}
	}
	ids := map[string]bool{}
	used := map[string]bool{}
	for _, r := range s.Requirements {
		if r.ID == "" || ids[r.ID] || r.Text == "" || len(r.Text) > 2000 || !tests[r.Test] || used[r.Test] {
			return errors.New("行为要求必须唯一绑定实际存在的验收测试")
		}
		ids[r.ID] = true
		used[r.Test] = true
	}
	if len(tests) != len(used) || !strings.Contains(s.Acceptance, MissingMarker) {
		return errors.New("每项验收测试须对应要求，并包含预期行为缺失断言标识")
	}
	return nil
}
func Files(s Spec, reference bool) map[string]string {
	out := map[string]string{}
	for p, v := range s.Baseline {
		out[p] = v
	}
	out[AcceptanceFile] = s.Acceptance
	if reference {
		for p, v := range s.Reference {
			out[p] = v
		}
	}
	return out
}
func Materialize(dir string, files map[string]string) error {
	if err := Qualify(files); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	all := map[string]string{"go.mod": Module}
	for p, s := range files {
		all[p] = s
	}
	for p, s := range all {
		f, e := root.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if e != nil {
			return e
		}
		_, e = f.WriteString(s)
		ce := f.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
	}
	return nil
}
func ReadWorkspace(dir string, s Spec) (map[string]string, error) {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return nil, errors.New("练习目录不存在或是链接")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, ent := range entries {
		name := ent.Name()
		info, err := root.Lstat(name)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > 60000 {
			return nil, errors.New("练习范围包含目录、链接或超限文件；请移出后重试")
		}
		if name != "go.mod" && !SafeName(name) && name != AcceptanceFile {
			return nil, errors.New("练习只允许 Go 源文件与固定 go.mod")
		}
		file, err := root.Open(name)
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(file, 60001))
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		if name == "go.mod" {
			if string(b) != Module {
				return nil, errors.New("模块范围变化，需要重新准备题目")
			}
			continue
		}
		files[name] = string(b)
	}
	if files[AcceptanceFile] != s.Acceptance {
		return nil, errors.New("固定验收测试已变化；请创建新题目版本，不能算原题通过")
	}
	for name, src := range s.Baseline {
		v, ok := files[name]
		if !ok {
			return nil, errors.New("不支持删除基线文件")
		}
		if strings.HasSuffix(name, "_test.go") && v != src {
			return nil, errors.New("原有测试已变化；请创建新题目版本")
		}
	}
	if err := Qualify(files); err != nil {
		return nil, err
	}
	return files, nil
}
func Scope(files map[string]string) []string {
	out := []string{}
	for name, src := range files {
		out = append(out, "file:"+name)
		f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ImportsOnly)
		if err != nil {
			// Scope cannot return parse errors; retaining the file scope while
			// omitting uncertain imports is the conservative fallback.
			continue
		}
		if f != nil {
			for _, im := range f.Imports {
				out = append(out, "import:"+im.Path.Value)
			}
		}
	}
	sort.Strings(out)
	return out
}
func Behavior(s Spec, result Result) []CheckResult {
	out := make([]CheckResult, 0, len(s.Requirements))
	for _, r := range s.Requirements {
		status := result.Tests[r.Test]
		if status == "" {
			status = "未执行或未完成"
		}
		out = append(out, CheckResult{r.ID, r.Text, r.Test, status})
	}
	return out
}
func Validated(s Spec, baseline, red, green Result) bool {
	if !baseline.Passed || red.Passed || red.Failure != "tests_failed" || !green.Passed {
		return false
	}
	for _, r := range s.Requirements {
		if red.Tests[r.Test] != "fail" || !red.Expected[r.Test] || green.Tests[r.Test] != "pass" {
			return false
		}
	}
	for name, status := range red.Tests {
		if status == "fail" {
			found := false
			for _, r := range s.Requirements {
				if r.Test == name {
					found = true
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}
func JoinSource(dir, name string) string {
	if dir == "." || dir == "" {
		return name
	}
	return filepath.ToSlash(filepath.Join(dir, name))
}
