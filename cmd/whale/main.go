package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"whale/internal/api"
	"whale/internal/runtime"
)

func main() {
	root := filepath.Clean(".")
	agent := runtime.NewAgentRuntime(root)
	if err := agent.Initialize(); err != nil {
		log.Fatalf("initialize runtime failed: %v", err)
	}

	if len(os.Args) > 1 {
		switch strings.ToLower(strings.TrimSpace(os.Args[1])) {
		case "serve":
			server := api.NewServer(agent)
			addr := ":8080"
			log.Printf("HTTP server listening on %s", addr)
			if err := http.ListenAndServe(addr, withCORS(server.Handler())); err != nil {
				log.Fatalf("http server failed: %v", err)
			}
			return
		case "dream":
			result, err := agent.RunAutodream()
			if err != nil {
				log.Fatalf("autodream failed: %v", err)
			}
			fmt.Println("whale dream executed successfully")
			fmt.Printf("processed=%d\n", result.ProcessedCount)
			if len(result.Summaries) > 0 {
				fmt.Println("--- SUMMARY FILES ---")
				for _, path := range result.Summaries {
					fmt.Println(path)
				}
			}
			fmt.Println("--- TRACE ---")
			fmt.Println(runtime.FormatTrace(result.Trace))
			return
		case "help", "-h", "--help":
			fmt.Println("whale demo")
			fmt.Println("")
			fmt.Println("用法:")
			fmt.Println("  whale serve    启动 HTTP 接口服务")
			fmt.Println("  whale dream    手动触发 Autodream")
			fmt.Println("  whale help     查看帮助")
			return
		}
	}

	currentWindowID := ""
	currentWindowRole := ""
	fmt.Println("whale demo initialized successfully")
	fmt.Println("请输入一句话，回车后将生成回复。输入 exit 退出。")
	fmt.Println("可用命令: /new [标题]  新建窗口 | /window window-001  加载旧窗口 | /list  查看窗口 | /dream  执行子梦")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		if strings.TrimSpace(currentWindowID) != "" {
			fmt.Printf("[%s]> ", currentWindowID)
		} else {
			fmt.Print("> ")
		}
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if strings.EqualFold(input, "exit") {
			fmt.Println("bye")
			return
		}
		if strings.EqualFold(input, "/list") {
			store, err := agent.Session.LoadWindows()
			if err != nil {
				log.Printf("list windows failed: %v", err)
				continue
			}
			if len(store.Windows) == 0 {
				fmt.Println("暂无窗口。输入 /new 创建第一个窗口。")
				continue
			}
			fmt.Println("--- WINDOWS ---")
			for _, w := range store.Windows {
				fmt.Printf("%s | %s | turns=%d | last=%s\n", w.ID, w.Title, w.TurnCount, w.LastActiveAt.Format(time.RFC3339))
			}
			continue
		}
		if strings.HasPrefix(strings.ToLower(input), "/new") {
			title := strings.TrimSpace(strings.TrimPrefix(input, "/new"))
			win, err := agent.Session.CreateWindow(title)
			if err != nil {
				log.Printf("create window failed: %v", err)
				continue
			}
			currentWindowID = win.ID
			currentWindowRole = win.AgentRole
			fmt.Printf("已创建并进入窗口: %s (%s)\n", win.ID, win.Title)
			continue
		}
		if strings.HasPrefix(strings.ToLower(input), "/window ") {
			parts := strings.Fields(input)
			if len(parts) >= 2 {
				if _, err := agent.Session.GetWindow(parts[1]); err != nil {
					log.Printf("window not found: %v", err)
					continue
				}
				currentWindowID = parts[1]
				if win, err := agent.Session.GetWindow(parts[1]); err == nil {
					currentWindowRole = win.AgentRole
				}
				fmt.Printf("已切换到窗口: %s\n", currentWindowID)
				continue
			}
			fmt.Println("用法: /window window-001")
			continue
		}
		if strings.EqualFold(input, "/dream") {
			result, err := agent.RunAutodream()
			if err != nil {
				log.Printf("autodream failed: %v", err)
				continue
			}
			fmt.Printf("processed=%d\n", result.ProcessedCount)
			fmt.Println(runtime.FormatTrace(result.Trace))
			continue
		}
		if currentWindowID == "" {
			fmt.Println("请先使用 /new 创建窗口，或使用 /window window-001 加载旧窗口。")
			continue
		}

		result, err := agent.HandleInput(currentWindowID, input, currentWindowRole)
		if err != nil {
			log.Printf("handle input failed: %v", err)
			continue
		}
		currentWindowID = result.WindowID

		fmt.Println("--- TRACE ---")
		fmt.Println(runtime.FormatTrace(result.Trace))
		fmt.Println("--- RESPONSE ---")
		fmt.Println(result.Reply)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("scan input failed: %v", err)
	}
}
