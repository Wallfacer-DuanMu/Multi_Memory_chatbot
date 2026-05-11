package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	root, err := projectRoot()
	if err != nil {
		log.Fatalf("resolve project root failed: %v", err)
	}

	backendPort := 8080
	frontendPort := 5173
	backendURL := fmt.Sprintf("http://127.0.0.1:%d", backendPort)
	frontendURL := fmt.Sprintf("http://127.0.0.1:%d/frontend/", frontendPort)

	fmt.Println("[launcher] 清理残留进程与端口占用...")
	cleanupPort(strconv.Itoa(backendPort))
	cleanupPort(strconv.Itoa(frontendPort))
	cleanupLockFiles(root)

	if err := ensurePortFree(backendPort, 10*time.Second); err != nil {
		log.Fatalf("backend port not ready: %v", err)
	}
	if err := ensurePortFree(frontendPort, 10*time.Second); err != nil {
		log.Fatalf("frontend port not ready: %v", err)
	}

	backend, err := startBackend(root)
	if err != nil {
		log.Fatalf("start backend failed: %v", err)
	}
	defer terminateProc(backend)

	frontend, err := startFrontendServer(root, frontendPort)
	if err != nil {
		_ = terminateProc(backend)
		log.Fatalf("start frontend server failed: %v", err)
	}
	defer terminateProc(frontend)

	if err := waitHTTP(backendURL+"/api/health", 25*time.Second); err != nil {
		log.Fatalf("backend health check failed: %v", err)
	}
	if err := waitHTTP(frontendURL, 25*time.Second); err != nil {
		log.Fatalf("frontend health check failed: %v", err)
	}

	fmt.Println("[launcher] 前后端已启动")
	fmt.Println("[launcher] 前端地址: " + frontendURL)
	fmt.Println("[launcher] 后端地址: " + backendURL)
	fmt.Println("[launcher] 按 Enter 退出并关闭前后端")

	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
}

func startBackend(root string) (*exec.Cmd, error) {
	cmd := exec.Command("go", "run", "./cmd/whale", "serve")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func startFrontendServer(root string, port int) (*exec.Cmd, error) {
	if _, err := os.Stat(filepath.Join(root, "frontend")); err != nil {
		return nil, err
	}
	pythonScript := strings.Join([]string{
		"import functools",
		"import http.server",
		"import socketserver",
		"import sys",
		"directory = sys.argv[1]",
		"port = int(sys.argv[2])",
		"class QuietHandler(http.server.SimpleHTTPRequestHandler):",
		"    def log_message(self, format, *args):",
		"        pass",
		"handler = functools.partial(QuietHandler, directory=directory)",
		"socketserver.TCPServer.allow_reuse_address = True",
		"with socketserver.TCPServer(('127.0.0.1', port), handler) as httpd:",
		"    print(f'Serving HTTP on 127.0.0.1 port {port} (http://127.0.0.1:{port}/) ...')",
		"    httpd.serve_forever()",
	}, "\n")
	cmd := exec.Command("python", "-c", pythonScript, root, strconv.Itoa(port))
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func waitHTTP(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(700 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", url)
}

func ensurePortFree(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err != nil {
			return nil
		}
		_ = conn.Close()
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("port still busy: %d", port)
}

func cleanupLockFiles(root string) {
	files := []string{
		filepath.Join(root, "frontend", "node_modules", ".vite", "vite.lock"),
		filepath.Join(root, "frontend", "node_modules", ".vite", "deps_temp"),
	}
	for _, file := range files {
		_ = os.RemoveAll(file)
	}
}

func projectRoot() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root = filepath.Clean(root)
	if filepath.Base(root) == "launcher" {
		root = filepath.Dir(filepath.Dir(root))
	}
	if filepath.Base(root) != "01版本" {
		if _, err := os.Stat(filepath.Join(root, "cmd", "launcher", "main.go")); errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("cannot locate project root from %s", root)
		}
	}
	return root, nil
}

func cleanupPort(port string) {
	if runtime.GOOS != "windows" {
		return
	}
	cmd := exec.Command("cmd", "/C", fmt.Sprintf("for /f \"tokens=5\" %%a in ('netstat -ano ^| findstr :%s') do taskkill /F /T /PID %%a", port))
	_ = cmd.Run()
}

func terminateProc(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
		return nil
	}
	return cmd.Process.Signal(syscall.SIGTERM)
}

func setProcessGroup(cmd *exec.Cmd) {
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	}
}
