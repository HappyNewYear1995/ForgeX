package service

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"jenkinsAgent/internal/model"
	"jenkinsAgent/internal/store"
)

// ExecResult holds the result of a script execution.
type ExecResult struct {
	Status string // success, failed
	Output string // combined stdout+stderr
}

// PlaywrightService handles Playwright script recording and execution.
type PlaywrightService struct {
	testEnvStore *store.TestEnvStore
}

func NewPlaywrightService() *PlaywrightService {
	return &PlaywrightService{
		testEnvStore: store.NewTestEnvStore(),
	}
}

// StartRecording launches Playwright codegen to record user actions.
func (s *PlaywrightService) StartRecording(env *model.TestEnv) (string, error) {
	url := env.URL

	tmpDir := os.TempDir()
	outFile := filepath.Join(tmpDir, fmt.Sprintf("pw_record_%d_%d.ts", env.ID, time.Now().Unix()))

	log.Printf("[playwright] recording: %s -> %s", url, outFile)

	cmd := exec.Command("npx", "playwright", "codegen",
		"--target", "javascript",
		"-o", outFile,
		url,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr != nil {
		log.Printf("[playwright] codegen exit error: %v, stderr: %s", runErr, stderr.String())
	}

	// Try reading the output file (with retry for timing)
	var data []byte
	var readErr error
	for i := 0; i < 5; i++ {
		data, readErr = os.ReadFile(outFile)
		if readErr == nil && len(data) > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	_ = os.Remove(outFile)

	// If file not found, try stdout as fallback
	if readErr != nil || len(data) == 0 {
		stdoutStr := stdout.String()
		if len(strings.TrimSpace(stdoutStr)) > 0 {
			log.Printf("[playwright] file not found, using stdout fallback (%d bytes)", len(stdoutStr))
			return stdoutStr, nil
		}

		log.Printf("[playwright] recording output file not found: %s, stderr: %s", outFile, stderr.String())
		if runErr != nil {
			return "", fmt.Errorf("录制失败：Playwright codegen 异常退出 (%v)。请确保已安装 Playwright (npx playwright install)", runErr)
		}
		return "", fmt.Errorf("录制结果为空：未生成脚本文件。\n\n正确操作步骤：\n1. 在浏览器中操作（点击、输入等）\n2. 在 Playwright Inspector 窗口点击「Record」按钮停止录制\n3. 等待代码出现在 Inspector 中\n4. 关闭浏览器\n\n如果问题仍然存在，请使用「编辑」按钮手动编写脚本")
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("录制结果为空，请在浏览器中操作后关闭浏览器")
	}

	log.Printf("[playwright] recorded %d bytes", len(content))
	return content, nil
}

// SaveScript saves script content for a test environment.
func (s *PlaywrightService) SaveScript(envID uint, content string) error {
	return s.testEnvStore.SaveScript(envID, content)
}

// GetTestEnv returns a test environment by ID.
func (s *PlaywrightService) GetTestEnv(id uint) (*model.TestEnv, error) {
	return s.testEnvStore.GetByID(id)
}

// ExecuteScript runs the script of a test environment.
func (s *PlaywrightService) ExecuteScript(env *model.TestEnv) (*ExecResult, error) {
	if strings.TrimSpace(env.ScriptContent) == "" {
		return nil, fmt.Errorf("脚本内容为空")
	}

	// Update status to running
	env.LastRunStatus = "running"
	now := time.Now()
	env.LastRunAt = &now
	_ = s.testEnvStore.Update(env)

	// Write script to project scripts dir
	scriptsDir := filepath.Join(".", "scripts_tmp")
	_ = os.MkdirAll(scriptsDir, 0755)
	scriptFile := filepath.Join(scriptsDir, fmt.Sprintf("pw_exec_%d_%d.ts", env.ID, time.Now().Unix()))
	if err := os.WriteFile(scriptFile, []byte(env.ScriptContent), 0644); err != nil {
		env.LastRunStatus = "failed"
		env.LastRunOutput = fmt.Sprintf("写入临时文件失败: %v", err)
		_ = s.testEnvStore.Update(env)
		return nil, fmt.Errorf("write script file: %w", err)
	}
	defer func(name string) {
		_ = os.Remove(name)
	}(scriptFile)

	testURL := env.URL

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "npx", "tsx", scriptFile)
	cmd.Dir = "."
	cmd.Env = append(os.Environ(),
		"TEST_URL="+testURL,
		"TEST_NAME="+env.Name,
	)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	log.Printf("[playwright] executing script for env #%d (%s) against %s", env.ID, env.Name, testURL)

	err := cmd.Run()
	result := &ExecResult{
		Output: output.String(),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = "failed"
			result.Output += "\n\n[超时] 脚本执行超过5分钟限制"
		} else {
			result.Status = "failed"
			result.Output += fmt.Sprintf("\n\n[错误] %v", err)
		}
	} else {
		result.Status = "success"
	}

	env.LastRunStatus = result.Status
	env.LastRunOutput = result.Output
	_ = s.testEnvStore.Update(env)

	log.Printf("[playwright] env #%d script finished: %s", env.ID, result.Status)
	return result, nil
}

// ExecuteScriptAsync runs a script in a background goroutine.
func (s *PlaywrightService) ExecuteScriptAsync(env *model.TestEnv) {
	go func() {
		_, err := s.ExecuteScript(env)
		if err != nil {
			log.Printf("[playwright] async exec error: %v", err)
		}
	}()
}

// RunScriptsForBuild executes scripts for a product's linked test environments after a successful build.
func (s *PlaywrightService) RunScriptsForBuild(build *model.Build, buildStore *store.BuildStore, testEnvStore *store.TestEnvStore) {
	if build.Status != model.BuildStatusSuccess {
		return
	}

	envs, err := testEnvStore.GetByProductID(build.ProductID)
	if err != nil || len(envs) == 0 {
		log.Printf("[playwright] no linked test environments for build#%d (product #%d)", build.ID, build.ProductID)
		return
	}

	build.ScriptRunStatus = "running"
	_ = buildStore.Update(build)

	var allOutput strings.Builder
	finalStatus := "success"

	for _, env := range envs {
		if strings.TrimSpace(env.ScriptContent) == "" {
			continue
		}
		allOutput.WriteString(fmt.Sprintf("\n=== [%s] ===\n", env.Name))
		result, err := s.ExecuteScript(&env)
		if err != nil {
			allOutput.WriteString(fmt.Sprintf("[错误] %v\n", err))
			finalStatus = "failed"
		} else {
			allOutput.WriteString(result.Output)
			if result.Status == "failed" {
				finalStatus = "failed"
			}
		}
		allOutput.WriteString("\n")
	}

	build.ScriptRunStatus = finalStatus
	build.ScriptRunOutput = allOutput.String()
	_ = buildStore.Update(build)
	log.Printf("[playwright] build#%d scripts finished: %s", build.ID, finalStatus)
}
