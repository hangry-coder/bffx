package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func GenerateSkill(root, name string) error {
	if name == "" {
		return errors.New("skill name is required")
	}

	skillsDir := filepath.Join(root, "bffx", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("create skills directory: %w", err)
	}

	filePath := filepath.Join(skillsDir, strings.ToLower(name)+".yaml")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("skill manifest already exists: %s\nHint: Use 'bffx upgrade' to modify existing skills.", filePath)
	}

	manifest := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Skill",
		"metadata:",
		"  name: " + name,
		"spec:",
		"  queue: default",
		"  input:",
		"    data: string",
		"  output:",
		"    result: string",
		"  onSuccess:",
		"    updateResource:",
		"      resource: SomeResource",
		"      match: { id: input.id }",
		"      set: { status: processed }",
		"",
	}, "\n")

	if err := os.WriteFile(filePath, []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("write skill manifest: %w", err)
	}

	// Create Python stub
	workerDir := filepath.Join(root, "worker", "skills")
	os.MkdirAll(workerDir, 0o755)
	pythonStub := fmt.Sprintf("def run(input):\n    print(f\"Executing skill %s with {input}\")\n    return {\"result\": \"ok\"}\n", name)
	os.WriteFile(filepath.Join(workerDir, strings.ToLower(name)+".py"), []byte(pythonStub), 0o644)

	if err := generateWorkerTest(root, name); err != nil {
		return fmt.Errorf("generate worker test: %w", err)
	}
	return nil
}

func GenerateFunction(root, name string) error {
	if name == "" {
		return errors.New("function name is required")
	}

	functionsDir := filepath.Join(root, "bffx", "functions")
	if err := os.MkdirAll(functionsDir, 0o755); err != nil {
		return fmt.Errorf("create functions directory: %w", err)
	}

	filePath := filepath.Join(functionsDir, strings.ToLower(name)+".yaml")
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("function manifest already exists: %s\nHint: Use 'bffx upgrade' to modify existing functions.", filePath)
	}

	manifest := strings.Join([]string{
		"apiVersion: bffx.io/v1alpha1",
		"kind: Function",
		"metadata:",
		"  name: " + name,
		"spec:",
		"  input:",
		"    param1: string",
		"  output:",
		"    result: string",
		"",
	}, "\n")

	if err := os.WriteFile(filePath, []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("write function manifest: %w", err)
	}

	// Create Python stub
	workerDir := filepath.Join(root, "worker", "functions")
	os.MkdirAll(workerDir, 0o755)
	pythonStub := fmt.Sprintf("def run(input):\n    return {\"result\": f\"Hello from %s\"}\n", name)
	os.WriteFile(filepath.Join(workerDir, strings.ToLower(name)+".py"), []byte(pythonStub), 0o644)

	if err := generateWorkerTest(root, name); err != nil {
		return fmt.Errorf("generate worker test: %w", err)
	}
	return nil
}

func generateWorkerTest(root, name string) error {
	testDir := filepath.Join(root, "tests", "worker")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		return err
	}

	lname := strings.ToLower(name)
	testPath := filepath.Join(testDir, "test_"+lname+".py")

	template := fmt.Sprintf(`import sys
import os

# Ensure worker package is in path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), ../../worker)))

from skills import %s

def test_%s_run():
    input_data = {"id": "mock-123"}
    res = %s.run(input_data)
    assert res is not None
`, lname, lname, lname)

	return os.WriteFile(testPath, []byte(template), 0o644)
}
