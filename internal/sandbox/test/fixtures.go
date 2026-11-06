package test

// Fixtures provides test code snippets for sandbox security and functionality testing.

// LegitimateGoCode contains safe Go code that should execute successfully.
var LegitimateGoCode = map[string]string{
	"hello_world": `
package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`,

	"read_file": `
package main

import (
	"fmt"
	"os"
)

func main() {
	// This should succeed (reading from workdir)
	data, err := os.ReadFile("test.txt")
	if err == nil {
		fmt.Println("Read:", string(data))
	}
}
`,

	"write_file": `
package main

import (
	"fmt"
	"os"
)

func main() {
	err := os.WriteFile("output.txt", []byte("test"), 0644)
	if err != nil {
		fmt.Println("Error:", err)
	}
}
`,

	"stdlib_operations": `
package main

import (
	"fmt"
	"strings"
	"time"
)

func main() {
	text := "Hello, World!"
	fmt.Println(strings.ToUpper(text))
	fmt.Println(time.Now())
}
`,

	"math_operations": `
package main

import (
	"fmt"
	"math"
)

func main() {
	fmt.Println(math.Sqrt(16))
	fmt.Println(math.Pi)
}
`,

	"loop_simple": `
package main

import "fmt"

func main() {
	for i := 0; i < 10; i++ {
		fmt.Println(i)
	}
}
`,

	"function_call": `
package main

import "fmt"

func add(a, b int) int {
	return a + b
}

func main() {
	fmt.Println(add(2, 3))
}
`,

	"json_operations": `
package main

import (
	"encoding/json"
	"fmt"
)

type Person struct {
	Name string
	Age  int
}

func main() {
	p := Person{"Alice", 30}
	data, _ := json.Marshal(p)
	fmt.Println(string(data))
}
`,
}

// MaliciousGoCode contains code that attempts to break out of the sandbox.
var MaliciousGoCode = map[string]string{
	"network_access": `
package main

import (
	"fmt"
	"net"
)

func main() {
	conn, err := net.Dial("tcp", "google.com:80")
	if err != nil {
		fmt.Println("Blocked:", err)
	} else {
		conn.Close()
	}
}
`,

	"directory_escape": `
package main

import (
	"fmt"
	"os"
)

func main() {
	// Attempt to read /etc/passwd
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		fmt.Println("Blocked:", err)
	} else {
		fmt.Println("READ:", string(data))
	}
}
`,

	"fork_process": `
package main

import (
	"fmt"
	"syscall"
)

func main() {
	// Attempt to fork
	pid, _, err := syscall.RawSyscall(syscall.SYS_FORK, 0, 0, 0)
	if pid > 0 {
		fmt.Println("FORKED:", pid)
	} else {
		fmt.Println("Blocked:", err)
	}
}
`,

	"env_var_leak": `
package main

import (
	"fmt"
	"os"
)

func main() {
	// Try to read API keys or secrets
	apiKey := os.Getenv("API_KEY")
	if apiKey != "" {
		fmt.Println("SECRET:", apiKey)
	}
}
`,

	"socket_creation": `
package main

import (
	"fmt"
	"syscall"
)

func main() {
	// Try to create a socket
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err == nil {
		fmt.Println("Socket created:", fd)
	} else {
		fmt.Println("Blocked:", err)
	}
}
`,

	"process_execution": `
package main

import (
	"fmt"
	"os/exec"
)

func main() {
	// Try to execute external command
	cmd := exec.Command("/bin/bash", "-c", "cat /etc/passwd")
	output, err := cmd.CombinedOutput()
	if err == nil {
		fmt.Println("EXECUTED:", string(output))
	} else {
		fmt.Println("Blocked:", err)
	}
}
`,

	"ptrace_attach": `
package main

import (
	"fmt"
	"syscall"
)

func main() {
	// Try to ptrace(PTRACE_SEIZE, 1, ...)
	err := syscall.PtraceAttach(1)
	if err == nil {
		fmt.Println("ATTACHED")
	} else {
		fmt.Println("Blocked:", err)
	}
}
`,

	"memory_bomb": `
package main

import "fmt"

func main() {
	// Try to allocate unlimited memory
	var data [][]byte
	for i := 0; i < 100000; i++ {
		data = append(data, make([]byte, 1024*1024)) // 1MB chunks
		if i%100 == 0 {
			fmt.Println("Allocated:", i, "MB")
		}
	}
}
`,
}

// EdgeCaseGoCode contains edge cases and boundary conditions.
var EdgeCaseGoCode = map[string]string{
	"empty_main": `
package main

func main() {
}
`,

	"infinite_loop": `
package main

func main() {
	for {
		// Infinite loop - should be terminated by timeout
	}
}
`,

	"large_output": `
package main

import "fmt"

func main() {
	// Generate large output
	for i := 0; i < 100000; i++ {
		fmt.Println("Line", i)
	}
}
`,

	"panic": `
package main

func main() {
	panic("intentional panic")
}
`,

	"nil_dereference": `
package main

func main() {
	var p *int
	_ = *p
}
`,

	"import_external": `
package main

import (
	"fmt"
	"github.com/some/external/package"
)

func main() {
	fmt.Println(package.Something())
}
`,

	"cgo_code": `
package main

/*
#include <stdio.h>
*/
import "C"

func main() {
	C.printf("Hello from C\n")
}
`,

	"runtime_panic": `
package main

import (
	"fmt"
	"runtime"
)

func main() {
	fmt.Println(runtime.NumGoroutine())
}
`,
}

// TestCode provides complete test functions.
type TestCode struct {
	Name        string
	Description string
	Code        string
	ShouldFail  bool
}

// CompleteFunctionTests returns complete test function code.
func CompleteFunctionTests() []TestCode {
	return []TestCode{
		{
			Name:        "basic_test",
			Description: "Basic test that should pass",
			Code: `
package main

import "testing"

func TestBasic(t *testing.T) {
	if 1+1 != 2 {
		t.Error("1+1 should be 2")
	}
}
`,
			ShouldFail: false,
		},
		{
			Name:        "failing_test",
			Description: "Test that should fail",
			Code: `
package main

import "testing"

func TestFailing(t *testing.T) {
	if 1+1 != 3 {
		t.Error("1+1 should be 3")
	}
}
`,
			ShouldFail: true,
		},
		{
			Name:        "io_test",
			Description: "Test file I/O in sandbox",
			Code: `
package main

import (
	"os"
	"testing"
)

func TestFileIO(t *testing.T) {
	// Write file
	err := os.WriteFile("test.txt", []byte("hello"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Read file
	data, err := os.ReadFile("test.txt")
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "hello" {
		t.Error("file content mismatch")
	}
}
`,
			ShouldFail: false,
		},
	}
}
